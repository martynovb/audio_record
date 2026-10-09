# audio_record

Installable desktop CLI for recording system audio and a microphone into one
M4A file. After installation, recording always starts with the same command:

```bash
audio-record
```

Press `Ctrl+C` to stop. The default file name is
`recording-YYYYMMDD-HHMMSS.m4a` in the current directory.

## Platform support

| Platform | Status | Native API |
| --- | --- | --- |
| macOS 15+ | Supported, self-contained release binary | ScreenCaptureKit + AVFoundation |
| Windows 10/11 x64 | Supported, self-contained release binary | WASAPI + Media Foundation |

The installed executable does not require Go, Swift, .NET, FFmpeg, Python, or
the project source tree. It embeds its native helper and extracts a
content-addressed copy into the user's cache on first launch.

## Install on macOS

Install the latest release with one command:

```bash
curl -fsSL https://raw.githubusercontent.com/martynovb/audio_record/main/install.sh | sh
```

The installer verifies the archive checksum and puts `audio-record` in
`~/.local/bin`. If necessary, it idempotently adds that directory to
`~/.zprofile` or `~/.bash_profile`; open a new terminal after the first
installation. Set `AUDIO_RECORD_NO_PATH_UPDATE=1` to disable profile changes,
or override the installation location with `AUDIO_RECORD_INSTALL_DIR`.

To uninstall:

```bash
rm -f "$HOME/.local/bin/audio-record"
rm -rf "$HOME/Library/Caches/audio-record"
```

Created `.m4a` files are not removed.

## Install on Windows

Run in PowerShell:

```powershell
irm https://raw.githubusercontent.com/martynovb/audio_record/main/install.ps1 | iex
```

The installer verifies the release checksum, installs into
`%LOCALAPPDATA%\Programs\audio-record`, and adds that directory to the user
`PATH`. The command is immediately available in the same PowerShell session:

```powershell
audio-record
```

To uninstall:

```powershell
irm https://raw.githubusercontent.com/martynovb/audio_record/main/uninstall.ps1 | iex
```

## Build from source

On macOS, the build-time requirements are Go 1.21+ and Xcode Command Line
Tools. They are not runtime dependencies:

```bash
make install
```

This installs to `~/.local/bin/audio-record` by default. Make sure that
`~/.local/bin` is in `PATH`, then run:

```bash
audio-record
```

To choose another prefix:

```bash
make install PREFIX=/usr/local
```

To create a portable release archive containing one executable:

```bash
make dist
```

On Windows, install Go 1.21+ and the .NET 9 SDK, then run in PowerShell:

```powershell
./scripts/build-windows.ps1
```

The self-contained executable is written to `bin/audio-record.exe`.

## Options

```text
-o, --output FILE       output .m4a file
--screen-id ID          select a macOS display
--mic-id ID             select a microphone on macOS or Windows
--no-system-audio       record only the microphone
--no-microphone         record only system audio
```

macOS asks for Screen & System Audio Recording and Microphone permissions on
first launch.

## Architecture

```text
cmd/audio-record/       shared executable entry point
internal/app/           shared arguments and lifecycle
internal/recorder/      platform contract and OS-selected backends
platforms/macos/        ScreenCaptureKit capture and native M4A export
platforms/windows/      WASAPI capture and Media Foundation M4A export
```

Both platforms use the same command-line contract and produce mixed AAC audio
inside an M4A container.

This is a desktop CLI. iOS would require a separate ReplayKit application and
cannot use the desktop command-line installation model.
