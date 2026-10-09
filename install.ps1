$ErrorActionPreference = "Stop"

$repository = if ($env:AUDIO_RECORD_REPOSITORY) {
    $env:AUDIO_RECORD_REPOSITORY
} else {
    "martynovb/audio_record"
}
$installDirectory = if ($env:AUDIO_RECORD_INSTALL_DIR) {
    $env:AUDIO_RECORD_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "Programs\audio-record"
}
$version = if ($env:AUDIO_RECORD_VERSION) {
    $env:AUDIO_RECORD_VERSION
} else {
    "latest"
}

if (-not $IsWindows -and $PSVersionTable.PSEdition -eq "Core") {
    throw "The PowerShell installer supports Windows only."
}

$architecture = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    default { throw "Unsupported Windows architecture: $env:PROCESSOR_ARCHITECTURE" }
}

$asset = "audio-record-windows-$architecture.zip"
if ($env:AUDIO_RECORD_DOWNLOAD_BASE) {
    $downloadBase = $env:AUDIO_RECORD_DOWNLOAD_BASE.TrimEnd("/")
} elseif ($version -eq "latest") {
    $downloadBase = "https://github.com/$repository/releases/latest/download"
} else {
    $downloadBase = "https://github.com/$repository/releases/download/$version"
}

$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) "audio-record-install-$([Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null

try {
    $archive = Join-Path $temporaryDirectory $asset
    $checksumFile = "$archive.sha256"
    Write-Host "Downloading $asset..."
    Invoke-WebRequest -UseBasicParsing -Uri "$downloadBase/$asset" -OutFile $archive
    Invoke-WebRequest -UseBasicParsing -Uri "$downloadBase/$asset.sha256" -OutFile $checksumFile

    $expectedChecksum = ((Get-Content -Raw $checksumFile).Trim() -split "\s+")[0].ToLowerInvariant()
    $actualChecksum = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToLowerInvariant()
    if ($actualChecksum -ne $expectedChecksum) {
        throw "SHA-256 checksum verification failed for $asset"
    }

    Expand-Archive -Path $archive -DestinationPath $temporaryDirectory -Force
    New-Item -ItemType Directory -Path $installDirectory -Force | Out-Null
    Copy-Item -Path (Join-Path $temporaryDirectory "audio-record.exe") -Destination (Join-Path $installDirectory "audio-record.exe") -Force
    foreach ($notice in @("THIRD_PARTY_NOTICES.md", "license")) {
        $noticePath = Join-Path $temporaryDirectory $notice
        if (Test-Path $noticePath) {
            Copy-Item -Path $noticePath -Destination (Join-Path $installDirectory $notice) -Force
        }
    }

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $alreadyInPath = ($userPath -split ";") | Where-Object {
        $_.TrimEnd("\") -ieq $installDirectory.TrimEnd("\")
    }
    if (-not $alreadyInPath) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $installDirectory
        } else {
            "$userPath;$installDirectory"
        }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
        Write-Host "Added $installDirectory to the user PATH."
    }
    if (-not (($env:Path -split ";") | Where-Object {
        $_.TrimEnd("\") -ieq $installDirectory.TrimEnd("\")
    })) {
        $env:Path = "$installDirectory;$env:Path"
    }

    Write-Host "Installed audio-record to $installDirectory\audio-record.exe"
    Write-Host "Run: audio-record"
} finally {
    Remove-Item -Path $temporaryDirectory -Recurse -Force -ErrorAction SilentlyContinue
}
