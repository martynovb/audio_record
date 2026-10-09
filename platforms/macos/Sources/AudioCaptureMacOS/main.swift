import AVFoundation
import CoreGraphics
import Foundation
import ScreenCaptureKit

private struct Options {
  let outputURL: URL
  let screenID: CGDirectDisplayID?
  let microphoneID: String?
  let capturesSystemAudio: Bool
  let capturesMicrophone: Bool

  static func parse(_ arguments: [String]) throws -> Options {
    var outputPath: String?
    var screenID: CGDirectDisplayID?
    var microphoneID: String?
    var capturesSystemAudio = true
    var capturesMicrophone = true
    var index = 0

    while index < arguments.count {
      switch arguments[index] {
      case "--output":
        index += 1
        guard index < arguments.count else { throw CaptureError("--output requires a value") }
        outputPath = arguments[index]
      case "--screen-id":
        index += 1
        guard index < arguments.count,
          let value = CGDirectDisplayID(arguments[index])
        else { throw CaptureError("--screen-id requires a numeric value") }
        screenID = value
      case "--mic-id":
        index += 1
        guard index < arguments.count else { throw CaptureError("--mic-id requires a value") }
        microphoneID = arguments[index]
      case "--no-system-audio":
        capturesSystemAudio = false
      case "--no-microphone":
        capturesMicrophone = false
      default:
        throw CaptureError("unknown argument: \(arguments[index])")
      }
      index += 1
    }

    guard let outputPath else { throw CaptureError("--output is required") }
    return Options(
      outputURL: URL(fileURLWithPath: outputPath),
      screenID: screenID,
      microphoneID: microphoneID,
      capturesSystemAudio: capturesSystemAudio,
      capturesMicrophone: capturesMicrophone
    )
  }
}

private final class CaptureDelegate: NSObject, SCRecordingOutputDelegate {
  private var continuation: CheckedContinuation<Void, Error>?
  private var pendingResult: Result<Void, Error>?

  func waitForFinish(removing output: SCRecordingOutput, from stream: SCStream) async throws {
    try await withCheckedThrowingContinuation { continuation in
      if let pendingResult {
        self.pendingResult = nil
        continuation.resume(with: pendingResult)
        return
      }

      self.continuation = continuation
      do {
        try stream.removeRecordingOutput(output)
      } catch {
        self.continuation = nil
        continuation.resume(throwing: error)
      }
    }
  }

  func recordingOutputDidStartRecording(_ recordingOutput: SCRecordingOutput) {}

  func recordingOutputDidFinishRecording(_ recordingOutput: SCRecordingOutput) {
    complete(with: .success(()))
  }

  func recordingOutput(_ recordingOutput: SCRecordingOutput, didFailWithError error: any Error) {
    complete(with: .failure(error))
  }

  private func complete(with result: Result<Void, Error>) {
    if let continuation {
      self.continuation = nil
      continuation.resume(with: result)
    } else {
      pendingResult = result
    }
  }
}

private final class MacOSRecorder {
  private let stream: SCStream
  private let recordingOutput: SCRecordingOutput
  private let delegate: CaptureDelegate

  init(options: Options, captureURL: URL) async throws {
    guard CGPreflightScreenCaptureAccess() || CGRequestScreenCaptureAccess() else {
      throw CaptureError("Screen & System Audio Recording permission is required")
    }

    if options.capturesMicrophone {
      guard await AVCaptureDevice.requestAccess(for: .audio) else {
        throw CaptureError("Microphone permission is required")
      }
    }

    let content = try await SCShareableContent.excludingDesktopWindows(
      false,
      onScreenWindowsOnly: true
    )
    let display: SCDisplay?
    if let requestedID = options.screenID {
      display = content.displays.first { $0.displayID == requestedID }
    } else {
      display = content.displays.first
    }
    guard let display else { throw CaptureError("requested display was not found") }

    let configuration = SCStreamConfiguration()
    configuration.width = 640
    configuration.height = max(
      2,
      Int((Double(display.height) / Double(display.width) * 640.0).rounded()) & ~1
    )
    configuration.minimumFrameInterval = CMTime(value: 1, timescale: 15)
    configuration.queueDepth = 3
    configuration.showsCursor = false
    configuration.capturesAudio = options.capturesSystemAudio
    configuration.excludesCurrentProcessAudio = true
    configuration.sampleRate = 48_000
    configuration.channelCount = 2

    var selectedMicrophone: AVCaptureDevice?
    if options.capturesMicrophone {
      selectedMicrophone = try Self.selectMicrophone(id: options.microphoneID)
      configuration.captureMicrophone = true
      if let selectedMicrophone {
        configuration.microphoneCaptureDeviceID = selectedMicrophone.uniqueID
      }
    }

    let filter = SCContentFilter(display: display, excludingWindows: [])
    stream = SCStream(filter: filter, configuration: configuration, delegate: nil)

    delegate = CaptureDelegate()
    let outputConfiguration = SCRecordingOutputConfiguration()
    outputConfiguration.outputURL = captureURL
    if outputConfiguration.availableOutputFileTypes.contains(.mp4) {
      outputConfiguration.outputFileType = .mp4
    }
    recordingOutput = SCRecordingOutput(configuration: outputConfiguration, delegate: delegate)
    try stream.addRecordingOutput(recordingOutput)

    print("Display: \(display.displayID)")
    if let selectedMicrophone {
      print("Microphone: \(selectedMicrophone.localizedName)")
    }
  }

  func start() async throws {
    try await stream.startCapture()
  }

  func stop() async throws {
    try await delegate.waitForFinish(removing: recordingOutput, from: stream)
    do {
      try await stream.stopCapture()
    } catch let error as NSError where error.domain == SCStreamErrorDomain && error.code == -3808 {
      // Removing the only recording output may already stop the stream.
    }
  }

  private static func selectMicrophone(id: String?) throws -> AVCaptureDevice {
    let discovery = AVCaptureDevice.DiscoverySession(
      deviceTypes: [.microphone, .external],
      mediaType: .audio,
      position: .unspecified
    )
    let devices = discovery.devices.filter { $0.hasMediaType(.audio) }
    if let id {
      guard let requested = devices.first(where: { $0.uniqueID == id }) else {
        throw CaptureError("microphone with ID \(id) was not found")
      }
      return requested
    }
    if let builtIn = devices.first(where: {
      let name = $0.localizedName.lowercased()
      return name.contains("built-in") || name.contains("built in") || name.contains("macbook")
    }) {
      return builtIn
    }
    guard let first = devices.first else { throw CaptureError("no microphone was found") }
    return first
  }
}

private func exportAudio(from sourceURL: URL, to destinationURL: URL) async throws {
  let asset = AVURLAsset(url: sourceURL)
  let sourceTracks = try await asset.loadTracks(withMediaType: .audio)
  guard !sourceTracks.isEmpty else {
    throw CaptureError("capture contains no audio; check system permissions")
  }

  let composition = AVMutableComposition()
  let duration = try await asset.load(.duration)
  var mixParameters: [AVMutableAudioMixInputParameters] = []
  let volume = 1.0 / Float(sourceTracks.count)

  for sourceTrack in sourceTracks {
    guard
      let compositionTrack = composition.addMutableTrack(
        withMediaType: .audio,
        preferredTrackID: kCMPersistentTrackID_Invalid
      )
    else {
      throw CaptureError("cannot create output audio track")
    }
    try compositionTrack.insertTimeRange(
      CMTimeRange(start: .zero, duration: duration),
      of: sourceTrack,
      at: .zero
    )
    let parameters = AVMutableAudioMixInputParameters(track: compositionTrack)
    parameters.setVolume(volume, at: .zero)
    mixParameters.append(parameters)
  }

  guard
    let exporter = AVAssetExportSession(
      asset: composition,
      presetName: AVAssetExportPresetAppleM4A
    )
  else {
    throw CaptureError("cannot create the M4A exporter")
  }
  let audioMix = AVMutableAudioMix()
  audioMix.inputParameters = mixParameters
  exporter.audioMix = audioMix
  do {
    try await exporter.export(to: destinationURL, as: .m4a)
  } catch {
    try? FileManager.default.removeItem(at: destinationURL)
    throw error
  }
}

private struct CaptureError: Error, CustomStringConvertible {
  let description: String

  init(_ description: String) {
    self.description = description
  }
}

@main
private struct AudioCaptureMacOS {
  static func main() async {
    let fileManager = FileManager.default
    let temporaryDirectory = fileManager.temporaryDirectory
      .appendingPathComponent("audio-record-" + UUID().uuidString, isDirectory: true)
    do {
      let options = try Options.parse(Array(CommandLine.arguments.dropFirst()))
      guard !fileManager.fileExists(atPath: options.outputURL.path) else {
        throw CaptureError("output already exists: \(options.outputURL.path)")
      }
      try fileManager.createDirectory(
        at: temporaryDirectory,
        withIntermediateDirectories: true
      )
      defer { try? fileManager.removeItem(at: temporaryDirectory) }

      let captureURL = temporaryDirectory.appendingPathComponent("capture.mp4")
      let recorder = try await MacOSRecorder(options: options, captureURL: captureURL)
      try await recorder.start()
      print("Native capture started")
      waitForTerminationSignal()
      print("Stopping native capture...")
      try await recorder.stop()
      print("Creating M4A...")
      try await exportAudio(from: captureURL, to: options.outputURL)
    } catch {
      FileHandle.standardError.write(Data("audio-capture-macos: \(error)\n".utf8))
      Foundation.exit(1)
    }
  }

  private static func waitForTerminationSignal() {
    signal(SIGINT, SIG_IGN)
    signal(SIGTERM, SIG_IGN)

    let semaphore = DispatchSemaphore(value: 0)
    let interrupt = DispatchSource.makeSignalSource(signal: SIGINT, queue: .global())
    let terminate = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .global())
    interrupt.setEventHandler { semaphore.signal() }
    terminate.setEventHandler { semaphore.signal() }
    interrupt.resume()
    terminate.resume()
    semaphore.wait()
    interrupt.cancel()
    terminate.cancel()
  }
}
