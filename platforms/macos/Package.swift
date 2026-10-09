// swift-tools-version: 5.9

import PackageDescription

let package = Package(
  name: "audio-capture-macos",
  platforms: [.macOS("15.0")],
  targets: [
    .executableTarget(
      name: "audio-capture-macos",
      path: "Sources/AudioCaptureMacOS"
    )
  ]
)
