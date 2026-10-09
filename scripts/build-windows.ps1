param(
    [string]$RuntimeIdentifier = "win-x64"
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$project = Join-Path $repositoryRoot "platforms\windows\AudioCaptureWindows.csproj"
$publishDirectory = Join-Path $repositoryRoot "platforms\windows\.build\$RuntimeIdentifier"
$assetDirectory = Join-Path $repositoryRoot "internal\recorder\assets"
$asset = Join-Path $assetDirectory "audio-capture-windows.exe"
$binaryDirectory = Join-Path $repositoryRoot "bin"

New-Item -ItemType Directory -Path $assetDirectory -Force | Out-Null
New-Item -ItemType Directory -Path $binaryDirectory -Force | Out-Null

dotnet publish $project --configuration Release --runtime $RuntimeIdentifier --self-contained true --output $publishDirectory
if ($LASTEXITCODE -ne 0) {
    throw "dotnet publish failed with exit code $LASTEXITCODE"
}
Copy-Item -Path (Join-Path $publishDirectory "audio-capture-windows.exe") -Destination $asset -Force

try {
    go test -tags embedded ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go test failed with exit code $LASTEXITCODE"
    }
    go build -tags embedded -trimpath -ldflags "-s -w" -o (Join-Path $binaryDirectory "audio-record.exe") ./cmd/audio-record
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
} finally {
    Remove-Item -Path $asset -Force -ErrorAction SilentlyContinue
}
