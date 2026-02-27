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

$repo = "JetBrains/teamcity-cli"
$binName = "teamcity.exe"
$installDir = "$HOME\.local\bin"

if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir | Out-Null
}

Write-Host "
 ████████╗ ██████╗
 ╚══██╔══╝██╔════╝   TeamCity CLI (installer)
    ██║   ██║        Documentation
    ██║   ██║        https://jb.gg/tc/docs
    ██║   ╚██████╗   Report issues
    ╚═╝    ╚═════╝   https://jb.gg/tc/issues
 ▄▄▄▄▄▄▄▄╗
 ╚═══════╝
"

Write-Host "This script will download TeamCity CLI to $installDir\$binName`n"

$releasesUrl = "https://api.github.com/repos/$repo/releases/latest"
$release = Invoke-RestMethod -Uri $releasesUrl

$tag = $release.tag_name
$version = $tag.TrimStart('v')

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }
$os = "windows"
$assetName = "teamcity_$($version)_$($os)_$($arch).zip"

$asset = $release.assets | Where-Object { $_.name -eq $assetName }
if (-not $asset) {
    Write-Error "Could not find asset $assetName in release $tag"
    exit 1
}

$downloadUrl = $asset.browser_download_url
$tempZip = Join-Path $env:TEMP "teamcity.zip"
$tempExtract = Join-Path $env:TEMP "teamcity_extract"

Write-Host "Downloading $assetName from $downloadUrl..."
Invoke-WebRequest -Uri $downloadUrl -OutFile $tempZip

if (Test-Path $tempExtract) {
    Remove-Item -Recurse -Force $tempExtract
}
New-Item -ItemType Directory -Path $tempExtract | Out-Null

Write-Host "Extracting..."
Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force

$exePath = Get-ChildItem -Path $tempExtract -Filter $binName -Recurse | Select-Object -First 1
if (-not $exePath) {
    Write-Error "Could not find $binName in the downloaded archive"
    exit 1
}

Move-Item -Path $exePath.FullName -Destination "$installDir\$binName" -Force

Write-Host "`n✓ Installed at $installDir\$binName"

# Check if installDir is in PATH
$path = [Environment]::GetEnvironmentVariable("Path", "User")
if ($path -notlike "*$installDir*") {
    Write-Host "`nAdding $installDir to your PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$path;$installDir", "User")
    $env:Path += ";$installDir"
    Write-Host "You might need to restart your terminal for changes to take effect."
}

& "$installDir\$binName" --version

# Cleanup
Remove-Item $tempZip -Force
Remove-Item -Recurse -Force $tempExtract

Write-Host "`nNext steps:"
Write-Host "  Authenticate with TeamCity"
Write-Host "  teamcity auth login`n"
Write-Host "  List recent builds"
Write-Host "  teamcity run list`n"
Write-Host "  Get help"
Write-Host "  teamcity --help`n"
