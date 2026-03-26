#
# Copyright 2021-2026 JetBrains s.r.o.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repo = "JetBrains/teamcity-cli"
$binName = "teamcity.exe"
$Release = if ($env:TC_INSTALL_RELEASE) { $env:TC_INSTALL_RELEASE } else { "" }
$OutDir = if ($env:TC_INSTALL_DIR) { $env:TC_INSTALL_DIR } else { "$HOME\.local\bin" }
$tempZip = $null
$tempExtract = $null
$stagedBin = $null

function Cleanup {
    if ($tempZip -and (Test-Path $tempZip)) {
        Remove-Item $tempZip -Force -ErrorAction SilentlyContinue
    }
    if ($tempExtract -and (Test-Path $tempExtract)) {
        Remove-Item -Recurse -Force $tempExtract -ErrorAction SilentlyContinue
    }
    if ($stagedBin -and (Test-Path $stagedBin)) {
        Remove-Item $stagedBin -Force -ErrorAction SilentlyContinue
    }
}

function Fail {
    param([string]$Message)
    Cleanup
    Write-Error "Error: $Message"
    exit 1
}

function Resolve-LatestRelease {
    $location = & curl.exe -s -o NUL -w "%{redirect_url}" "https://github.com/$repo/releases/latest"
    if (-not $location) {
        Fail "failed to resolve latest TeamCity CLI release"
    }
    $tag = ($location -split '/')[-1]
    if (-not $tag) {
        Fail "failed to resolve latest TeamCity CLI release"
    }
    return $tag
}

Write-Host "
 ████████╗ ██████╗
 ╚══██╔══╝██╔════╝   TeamCity CLI (installer)
    ██║   ██║        Documentation
    ██║   ██║        https://jb.gg/tc/docs
    ██║   ╚██████╗   Report issues
    ╚═╝    ╚═════╝   https://jb.gg/tc/issues
"

Write-Host "This script will download TeamCity CLI to $OutDir\$binName`n"
Write-Host "To install a specific version:"
Write-Host "  `$env:TC_INSTALL_RELEASE='v0.7.0'; irm https://jb.gg/tc/install.ps1 | iex`n"

try {
    if (-not $Release) {
        $Release = Resolve-LatestRelease
    }

    $version = $Release.TrimStart('v')
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }
    $assetName = "teamcity_$($version)_windows_$($arch).zip"
    $downloadUrl = "https://github.com/$repo/releases/download/$Release/$assetName"

    Write-Host "Installing teamcity ($Release) from $downloadUrl`n"

    if (-not (Test-Path $OutDir)) {
        New-Item -ItemType Directory -Path $OutDir | Out-Null
    }
    if (-not (Test-Path $OutDir -PathType Container)) {
        Fail "output path is not a directory: $OutDir"
    }

    $tempZip = Join-Path $env:TEMP "teamcity_$([guid]::NewGuid().ToString('N')).zip"
    $tempExtract = Join-Path $env:TEMP "teamcity_extract_$([guid]::NewGuid().ToString('N'))"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempZip
    } catch {
        Fail "download failed for $assetName"
    }

    New-Item -ItemType Directory -Path $tempExtract | Out-Null
    Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force

    $exePath = Get-ChildItem -Path $tempExtract -Filter $binName -Recurse | Select-Object -First 1
    if (-not $exePath) {
        Fail "could not find $binName in the downloaded archive"
    }

    $stagedBin = Join-Path $OutDir ".teamcity_staged_$([guid]::NewGuid().ToString('N')).exe"
    Copy-Item -Path $exePath.FullName -Destination $stagedBin -Force
    Move-Item -Path $stagedBin -Destination "$OutDir\$binName" -Force
    $stagedBin = $null

    Write-Host "`n✓ Installed at $OutDir\$binName`n"
    & "$OutDir\$binName" --version

    $path = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($path -notlike "*$OutDir*") {
        Write-Host "`nAdding $OutDir to your PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$path;$OutDir", "User")
        $env:Path += ";$OutDir"
        Write-Host "You might need to restart your terminal for changes to take effect."
    }

    Cleanup

    Write-Host "`nNext steps:"
    Write-Host "  Authenticate with TeamCity"
    Write-Host "  teamcity auth login`n"
    Write-Host "  List recent builds"
    Write-Host "  teamcity run list`n"
    Write-Host "  Get help"
    Write-Host "  teamcity --help`n"
} catch {
    Fail $_.Exception.Message
}
