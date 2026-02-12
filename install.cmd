@echo off
setlocal

set "REPO=JetBrains/teamcity-cli"
set "BIN_NAME=tc.exe"
set "INSTALL_DIR=%USERPROFILE%\.local\bin"

if not exist "%INSTALL_DIR%" (
    mkdir "%INSTALL_DIR%"
)

echo.

echo This script will download TeamCity CLI to %INSTALL_DIR%\%BIN_NAME%
echo.

:: Use PowerShell to get the latest release info and download the asset
powershell -NoProfile -Command ^
    "$releasesUrl = 'https://api.github.com/repos/%REPO%/releases/latest';" ^
    "$release = Invoke-RestMethod -Uri $releasesUrl;" ^
    "$tag = $release.tag_name;" ^
    "$version = $tag.TrimStart('v');" ^
    "$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x86_64' };" ^
    "$assetName = \"tc_$($version)_windows_$($arch).zip\";" ^
    "$asset = $release.assets | Where-Object { $_.name -eq $assetName };" ^
    "if (-not $asset) { Write-Error \"Could not find asset $assetName in release $tag\"; exit 1 };" ^
    "$tempZip = Join-Path $env:TEMP 'tc.zip';" ^
    "Write-Host \"Downloading $assetName...\";" ^
    "Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $tempZip;" ^
    "$tempExtract = Join-Path $env:TEMP 'tc_extract';" ^
    "if (Test-Path $tempExtract) { Remove-Item -Recurse -Force $tempExtract };" ^
    "New-Item -ItemType Directory -Path $tempExtract | Out-Null;" ^
    "Write-Host \"Extracting...\";" ^
    "Expand-Archive -Path $tempZip -DestinationPath $tempExtract -Force;" ^
    "$exePath = Get-ChildItem -Path $tempExtract -Filter '%BIN_NAME%' -Recurse | Select-Object -First 1;" ^
    "Move-Item -Path $exePath.FullName -Destination '%INSTALL_DIR%\%BIN_NAME%' -Force;" ^
    "Remove-Item $tempZip -Force;" ^
    "Remove-Item -Recurse -Force $tempExtract;"

if %ERRORLEVEL% neq 0 (
    echo.
    echo Error: Installation failed.
    exit /b %ERRORLEVEL%
)

echo.
echo v Installed at %INSTALL_DIR%\%BIN_NAME%
echo.

"%INSTALL_DIR%\%BIN_NAME%" --version

:: Add to PATH if not present
powershell -NoProfile -Command ^
    "$path = [Environment]::GetEnvironmentVariable('Path', 'User');" ^
    "if ($path -notlike '*%INSTALL_DIR%*') {" ^
    "  Write-Host 'Adding %INSTALL_DIR% to your PATH...';" ^
    "  [Environment]::SetEnvironmentVariable('Path', \"$path;%INSTALL_DIR%\", 'User');" ^
    "  Write-Host 'You might need to restart your terminal for changes to take effect.';" ^
    "}"

echo.
echo Next steps:
echo   Authenticate with TeamCity
echo   tc auth login
echo.
echo   List recent builds
echo   tc run list
echo.
echo   Get help
echo   tc --help
echo.

endlocal
