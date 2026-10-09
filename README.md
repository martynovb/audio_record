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
| Windows | CLI contract ready; native capture not implemented yet | WASAPI planned |

The installed macOS executable does not require Go, Swift, FFmpeg, Python, or
the project source tree. It embeds its native helper and extracts a
content-addressed copy into the user's cache on first launch.

## Install on macOS

Install the latest release with one command:

```bash
curl -fsSL https://raw.githubusercontent.com/martynovb/audio_record/main/install.sh | sh
```

The installer verifies the archive checksum and puts `audio-record` in
`~/.local/bin`. Override the location with `AUDIO_RECORD_INSTALL_DIR`.

To uninstall:

```bash
rm -f "$HOME/.local/bin/audio-record"
rm -rf "$HOME/Library/Caches/audio-record"
```

Created `.m4a` files are not removed.

### Build from source

Build-time requirements are Go 1.21+ and Xcode Command Line Tools. They are not
runtime dependencies:

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

## Options

```text
-o, --output FILE       output .m4a file
--screen-id ID          select a macOS display
--mic-id ID             select a microphone
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
```

The common CLI already builds on Windows, but recording intentionally returns
an explicit “not implemented” error until `backend_windows.go` is backed by a
native WASAPI implementation. The command-line interface will remain the same.

This is a desktop CLI. iOS would require a separate ReplayKit application and
cannot use the desktop command-line installation model.
