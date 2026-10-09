$ErrorActionPreference = "Stop"

$installDirectory = if ($env:AUDIO_RECORD_INSTALL_DIR) {
    $env:AUDIO_RECORD_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "Programs\audio-record"
}
$cacheDirectory = Join-Path $env:LOCALAPPDATA "audio-record"

Remove-Item -Path $installDirectory -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path $cacheDirectory -Recurse -Force -ErrorAction SilentlyContinue

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $updatedPath = (($userPath -split ";") | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_) -and
        $_.TrimEnd("\") -ine $installDirectory.TrimEnd("\")
    }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $updatedPath, "User")
}

Write-Host "Removed audio-record from $installDirectory and the user PATH."
