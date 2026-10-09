using System.Diagnostics;
using NAudio.CoreAudioApi;
using NAudio.Wave;
using NAudio.Wave.SampleProviders;

namespace AudioCaptureWindows;

internal sealed record Options(
    string OutputPath,
    string? MicrophoneId,
    bool CaptureSystemAudio,
    bool CaptureMicrophone)
{
    public static Options Parse(string[] arguments)
    {
        string? outputPath = null;
        string? microphoneId = null;
        var captureSystemAudio = true;
        var captureMicrophone = true;

        for (var index = 0; index < arguments.Length; index++)
        {
            switch (arguments[index])
            {
                case "--output":
                    outputPath = ReadValue(arguments, ref index, "--output");
                    break;
                case "--mic-id":
                    microphoneId = ReadValue(arguments, ref index, "--mic-id");
                    break;
                case "--no-system-audio":
                    captureSystemAudio = false;
                    break;
                case "--no-microphone":
                    captureMicrophone = false;
                    break;
                default:
                    throw new ArgumentException($"Unknown argument: {arguments[index]}");
            }
        }

        if (string.IsNullOrWhiteSpace(outputPath))
        {
            throw new ArgumentException("--output is required");
        }
        if (!captureSystemAudio && !captureMicrophone)
        {
            throw new ArgumentException("At least one audio source must be enabled");
        }
        if (!string.Equals(Path.GetExtension(outputPath), ".m4a", StringComparison.OrdinalIgnoreCase))
        {
            throw new ArgumentException("Output must have the .m4a extension");
        }

        return new Options(
            Path.GetFullPath(outputPath),
            microphoneId,
            captureSystemAudio,
            captureMicrophone);
    }

    private static string ReadValue(string[] arguments, ref int index, string name)
    {
        index++;
        if (index >= arguments.Length)
        {
            throw new ArgumentException($"{name} requires a value");
        }
        return arguments[index];
    }
}

internal sealed class RecordedSource : IDisposable
{
    private readonly IWaveIn capture;
    private readonly WaveFileWriter writer;
    private readonly Stopwatch timeline;
    private readonly TaskCompletionSource stopped = new(
        TaskCreationOptions.RunContinuationsAsynchronously);
    private readonly object writerLock = new();
    private long bytesWritten;
    private Exception? captureError;
    private bool disposed;

    public RecordedSource(IWaveIn capture, string path, Stopwatch timeline)
    {
        this.capture = capture;
        this.timeline = timeline;
        writer = new WaveFileWriter(path, capture.WaveFormat);
        capture.DataAvailable += OnDataAvailable;
        capture.RecordingStopped += OnRecordingStopped;
    }

    public void Start() => capture.StartRecording();

    public void Stop()
    {
        try
        {
            capture.StopRecording();
        }
        catch (InvalidOperationException)
        {
            stopped.TrySetResult();
        }
    }

    public async Task FinishAsync(TimeSpan duration)
    {
        await stopped.Task.ConfigureAwait(false);
        lock (writerLock)
        {
            WriteSilenceUntil(BytesAt(duration));
            writer.Flush();
        }
        Dispose();
        if (captureError is not null)
        {
            throw new InvalidOperationException("WASAPI capture failed", captureError);
        }
    }

    private void OnDataAvailable(object? sender, WaveInEventArgs eventArgs)
    {
        lock (writerLock)
        {
            var expectedEnd = BytesAt(timeline.Elapsed);
            var desiredStart = Math.Max(0, expectedEnd - eventArgs.BytesRecorded);
            WriteSilenceUntil(desiredStart);
            writer.Write(eventArgs.Buffer, 0, eventArgs.BytesRecorded);
            bytesWritten += eventArgs.BytesRecorded;
        }
    }

    private void OnRecordingStopped(object? sender, StoppedEventArgs eventArgs)
    {
        captureError = eventArgs.Exception;
        stopped.TrySetResult();
    }

    private long BytesAt(TimeSpan duration)
    {
        var bytes = (long)(duration.TotalSeconds * capture.WaveFormat.AverageBytesPerSecond);
        return bytes - bytes % capture.WaveFormat.BlockAlign;
    }

    private void WriteSilenceUntil(long targetPosition)
    {
        var remaining = targetPosition - bytesWritten;
        if (remaining <= 0)
        {
            return;
        }

        var silence = new byte[Math.Min(64 * 1024, (int)Math.Min(int.MaxValue, remaining))];
        while (remaining > 0)
        {
            var count = (int)Math.Min(silence.Length, remaining);
            count -= count % capture.WaveFormat.BlockAlign;
            if (count == 0)
            {
                break;
            }
            writer.Write(silence, 0, count);
            bytesWritten += count;
            remaining -= count;
        }
    }

    public void Dispose()
    {
        if (disposed)
        {
            return;
        }
        disposed = true;
        capture.DataAvailable -= OnDataAvailable;
        capture.RecordingStopped -= OnRecordingStopped;
        writer.Dispose();
        capture.Dispose();
    }
}

internal sealed class CaptureSession : IDisposable
{
    private readonly Stopwatch timeline = new();
    private readonly List<RecordedSource> sources = [];
    private bool stopped;

    public CaptureSession(Options options, string temporaryDirectory)
    {
        using var enumerator = new MMDeviceEnumerator();
        try
        {
            if (options.CaptureSystemAudio)
            {
                var renderDevice = enumerator.GetDefaultAudioEndpoint(DataFlow.Render, Role.Multimedia);
                Console.WriteLine($"System audio: {renderDevice.FriendlyName}");
                sources.Add(new RecordedSource(
                    new WasapiLoopbackCapture(renderDevice),
                    Path.Combine(temporaryDirectory, "system.wav"),
                    timeline));
            }

            if (options.CaptureMicrophone)
            {
                var microphone = SelectMicrophone(enumerator, options.MicrophoneId);
                Console.WriteLine($"Microphone: {microphone.FriendlyName}");
                sources.Add(new RecordedSource(
                    new WasapiCapture(microphone),
                    Path.Combine(temporaryDirectory, "microphone.wav"),
                    timeline));
            }
        }
        catch
        {
            Dispose();
            throw;
        }
    }

    public IReadOnlyList<string> AudioFiles { get; private set; } = [];

    public void Start(string temporaryDirectory)
    {
        AudioFiles = Directory.GetFiles(temporaryDirectory, "*.wav");
        timeline.Start();
        try
        {
            foreach (var source in sources)
            {
                source.Start();
            }
        }
        catch
        {
            foreach (var source in sources)
            {
                source.Stop();
            }
            throw;
        }
    }

    public async Task StopAsync(string temporaryDirectory)
    {
        if (stopped)
        {
            return;
        }
        stopped = true;
        foreach (var source in sources)
        {
            source.Stop();
        }
        timeline.Stop();
        await Task.WhenAll(sources.Select(source => source.FinishAsync(timeline.Elapsed)))
            .ConfigureAwait(false);
        AudioFiles = Directory.GetFiles(temporaryDirectory, "*.wav");
    }

    private static MMDevice SelectMicrophone(MMDeviceEnumerator enumerator, string? id)
    {
        if (string.IsNullOrEmpty(id))
        {
            return enumerator.GetDefaultAudioEndpoint(DataFlow.Capture, Role.Multimedia);
        }

        var devices = enumerator.EnumerateAudioEndPoints(DataFlow.Capture, DeviceState.Active);
        var selected = devices.FirstOrDefault(device =>
            string.Equals(device.ID, id, StringComparison.OrdinalIgnoreCase));
        return selected ?? throw new ArgumentException($"Microphone with ID {id} was not found");
    }

    public void Dispose()
    {
        foreach (var source in sources)
        {
            source.Dispose();
        }
    }
}

internal static class AudioExporter
{
    private const int TargetSampleRate = 48_000;

    public static void Export(IReadOnlyList<string> files, string outputPath)
    {
        if (File.Exists(outputPath))
        {
            throw new IOException($"Output already exists: {outputPath}");
        }

        var readers = new List<AudioFileReader>();
        var temporaryOutput = Path.ChangeExtension(outputPath, $"{Guid.NewGuid():N}.mp4");
        try
        {
            var providers = new List<ISampleProvider>();
            foreach (var file in files)
            {
                var reader = new AudioFileReader(file);
                readers.Add(reader);
                providers.Add(Normalize(reader));
            }
            if (providers.Count == 0)
            {
                throw new InvalidOperationException("Capture produced no audio sources");
            }

            ISampleProvider output = providers.Count == 1
                ? providers[0]
                : new MixingSampleProvider(providers.Select(provider =>
                    new VolumeSampleProvider(provider) { Volume = 1f / providers.Count }));

            MediaFoundationApi.Startup();
            try
            {
                MediaFoundationEncoder.EncodeToAac(
                    output.ToWaveProvider16(),
                    temporaryOutput,
                    192_000);
            }
            finally
            {
                MediaFoundationApi.Shutdown();
            }

            File.Move(temporaryOutput, outputPath);
        }
        catch
        {
            File.Delete(temporaryOutput);
            File.Delete(outputPath);
            throw;
        }
        finally
        {
            foreach (var reader in readers)
            {
                reader.Dispose();
            }
        }
    }

    private static ISampleProvider Normalize(AudioFileReader reader)
    {
        ISampleProvider provider = reader;
        provider = provider.WaveFormat.Channels switch
        {
            1 => new MonoToStereoSampleProvider(provider),
            2 => provider,
            _ => throw new NotSupportedException(
                $"Audio devices with {provider.WaveFormat.Channels} channels are not supported")
        };
        if (provider.WaveFormat.SampleRate != TargetSampleRate)
        {
            provider = new WdlResamplingSampleProvider(provider, TargetSampleRate);
        }
        return provider;
    }
}

internal static class Program
{
    public static async Task<int> Main(string[] arguments)
    {
        string? temporaryDirectory = null;
        try
        {
            var options = Options.Parse(arguments);
            temporaryDirectory = Path.Combine(Path.GetTempPath(), $"audio-record-{Guid.NewGuid():N}");
            Directory.CreateDirectory(temporaryDirectory);

            using var session = new CaptureSession(options, temporaryDirectory);
            session.Start(temporaryDirectory);
            Console.WriteLine("Native Windows capture started");
            await WaitForStopAsync().ConfigureAwait(false);
            Console.WriteLine("Stopping native Windows capture...");
            await session.StopAsync(temporaryDirectory).ConfigureAwait(false);
            Console.WriteLine("Creating M4A...");
            AudioExporter.Export(session.AudioFiles, options.OutputPath);
            return 0;
        }
        catch (Exception exception)
        {
            Console.Error.WriteLine($"audio-capture-windows: {exception.Message}");
            return 1;
        }
        finally
        {
            if (temporaryDirectory is not null)
            {
                try
                {
                    Directory.Delete(temporaryDirectory, recursive: true);
                }
                catch
                {
                    // Temporary capture files are best-effort cleanup.
                }
            }
        }
    }

    private static async Task WaitForStopAsync()
    {
        var stopped = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        ConsoleCancelEventHandler cancelHandler = (_, eventArgs) =>
        {
            eventArgs.Cancel = true;
            stopped.TrySetResult();
        };
        Console.CancelKeyPress += cancelHandler;
        _ = Task.Run(async () =>
        {
            await Console.In.ReadLineAsync().ConfigureAwait(false);
            stopped.TrySetResult();
        });

        try
        {
            await stopped.Task.ConfigureAwait(false);
        }
        finally
        {
            Console.CancelKeyPress -= cancelHandler;
        }
    }
}
