@echo off
::
:: Copyright 2021-2026 JetBrains s.r.o.
::
:: Licensed under the Apache License, Version 2.0 (the "License");
:: you may not use this file except in compliance with the License.
:: You may obtain a copy of the License at
::
:: https://www.apache.org/licenses/LICENSE-2.0
::
:: Unless required by applicable law or agreed to in writing, software
:: distributed under the License is distributed on an "AS IS" BASIS,
:: WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
:: See the License for the specific language governing permissions and
:: limitations under the License.
::

setlocal enabledelayedexpansion

set "RELEASE="
set "INSTALL_DIR="
set "JETBRAINS_CDN=0"
set "RELEASE_SEEN=0"
set "BIN_NAME=teamcity.exe"
set "REPO=JetBrains/teamcity-cli"

:parse_loop
if [%1]==[] goto :end_parse
if /i "%~1"=="--jetbrains-cdn" (
    set "JETBRAINS_CDN=1"
    shift
    goto :parse_loop
)
if "!RELEASE_SEEN!"=="0" (
    set "RELEASE=%~1"
    set "RELEASE_SEEN=1"
    shift
    goto :parse_loop
)
if "!INSTALL_DIR!"=="" set "INSTALL_DIR=%~1"
shift
goto :parse_loop
:end_parse
if "!INSTALL_DIR!"=="" set "INSTALL_DIR=%USERPROFILE%\.local\bin"

echo.
echo  ========= ======
echo  ==   ==        TeamCity CLI (installer)
echo     ==   ==        Documentation
echo     ==   ==        https://jb.gg/tc/docs
echo     ==    ======   Report issues
echo     ==     =====   https://jb.gg/tc/issues
echo.
echo This script will download TeamCity CLI to !INSTALL_DIR!\!BIN_NAME!
echo.
echo To install a specific version: install.cmd v0.8.3
echo.
echo If GitHub is down, use JetBrains CDN directly:
echo   install.cmd --jetbrains-cdn
echo.

:: Check curl is available (ships with Windows 10+)
curl --version >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo Error: curl is required but not found. Use install.ps1 instead. >&2
    exit /b 1
)

:: Resolve latest release
if "!RELEASE!"=="" (
    if "!JETBRAINS_CDN!"=="1" (
        for /f "delims=" %%a in ('curl -fsSL "https://download.jetbrains.com/resources/teamcity-cli/latest"') do set "RELEASE=%%a"
        if "!RELEASE!"=="" (
            echo Error: failed to resolve latest release from JetBrains CDN >&2
            exit /b 1
        )
    ) else (
        for /f "delims=" %%a in ('curl -s -o nul -w "%%{redirect_url}" "https://github.com/!REPO!/releases/latest"') do set "LOCATION=%%a"
        if "!LOCATION!"=="" (
            echo Error: failed to resolve latest release >&2
            exit /b 1
        )
        for %%t in ("!LOCATION!") do set "RELEASE=%%~nxt"
    )
)

if "!RELEASE!"=="" (
    echo Error: failed to resolve latest release >&2
    exit /b 1
)

:: Strip leading 'v' for version
set "VERSION=!RELEASE!"
if "!VERSION:~0,1!"=="v" set "VERSION=!VERSION:~1!"

:: Detect architecture
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
    set "ARCH=arm64"
) else (
    set "ARCH=x86_64"
)

set "ASSET_NAME=teamcity_!VERSION!_windows_!ARCH!.zip"
set "GH_URL=https://github.com/!REPO!/releases/download/!RELEASE!/!ASSET_NAME!"
set "JB_URL=https://download.jetbrains.com/resources/teamcity-cli/!VERSION!/!ASSET_NAME!"
if "!JETBRAINS_CDN!"=="1" (set "URL=!JB_URL!") else (set "URL=!GH_URL!")

echo Installing teamcity (!RELEASE!)
echo.

:: Create install dir
if not exist "!INSTALL_DIR!" mkdir "!INSTALL_DIR!"

:: Create unique temp paths
set "TEMP_ZIP=%TEMP%\teamcity_%RANDOM%%RANDOM%.zip"
set "TEMP_DIR=%TEMP%\teamcity_extract_%RANDOM%%RANDOM%"

:: Download
curl -fsSL "!URL!" -o "!TEMP_ZIP!"
if !ERRORLEVEL! neq 0 (
    echo Error: download failed for !ASSET_NAME! >&2
    if "!JETBRAINS_CDN!"=="0" (
        echo. >&2
        echo If GitHub is unreachable, retry using the JetBrains CDN: >&2
        echo   install.cmd --jetbrains-cdn >&2
    )
    if exist "!TEMP_ZIP!" del "!TEMP_ZIP!"
    exit /b 1
)

:: Extract
mkdir "!TEMP_DIR!"
set "TC_ZIP=!TEMP_ZIP!"
set "TC_DIR=!TEMP_DIR!"
powershell -NoProfile -Command "Expand-Archive -LiteralPath $env:TC_ZIP -DestinationPath $env:TC_DIR -Force"
if !ERRORLEVEL! neq 0 (
    echo Error: extraction failed >&2
    del "!TEMP_ZIP!" 2>nul
    rmdir /s /q "!TEMP_DIR!" 2>nul
    exit /b 1
)

:: Find the binary
set "FOUND_BIN="
for /f "usebackq delims=" %%f in (`powershell -NoProfile -Command "(Get-ChildItem -LiteralPath $env:TC_DIR -Filter $env:BIN_NAME -Recurse | Select-Object -First 1).FullName"`) do set "FOUND_BIN=%%f"

if "!FOUND_BIN!"=="" (
    echo Error: could not find !BIN_NAME! in archive >&2
    del "!TEMP_ZIP!" 2>nul
    rmdir /s /q "!TEMP_DIR!" 2>nul
    exit /b 1
)

:: Atomic install: copy to staged file, then rename
set "STAGED=!INSTALL_DIR!\.teamcity_staged_%RANDOM%.exe"
copy "!FOUND_BIN!" "!STAGED!" >nul
if !ERRORLEVEL! neq 0 (
    echo Error: failed to stage binary >&2
    del "!TEMP_ZIP!" 2>nul
    rmdir /s /q "!TEMP_DIR!" 2>nul
    exit /b 1
)
move /y "!STAGED!" "!INSTALL_DIR!\!BIN_NAME!" >nul
if !ERRORLEVEL! neq 0 (
    echo Error: failed to install binary >&2
    del "!STAGED!" 2>nul
    del "!TEMP_ZIP!" 2>nul
    rmdir /s /q "!TEMP_DIR!" 2>nul
    exit /b 1
)

:: Cleanup temp files
del "!TEMP_ZIP!" 2>nul
rmdir /s /q "!TEMP_DIR!" 2>nul

echo.
echo v Installed at !INSTALL_DIR!\!BIN_NAME!
echo.

"!INSTALL_DIR!\!BIN_NAME!" --version

:: Add to PATH if not present
powershell -NoProfile -Command ^
    "$path = [Environment]::GetEnvironmentVariable('Path', 'User');" ^
    "if ($path -notlike '*!INSTALL_DIR!*') {" ^
    "  Write-Host 'Adding !INSTALL_DIR! to your PATH...';" ^
    "  [Environment]::SetEnvironmentVariable('Path', \"$path;!INSTALL_DIR!\", 'User');" ^
    "  Write-Host 'You might need to restart your terminal for changes to take effect.';" ^
    "}"

echo.
echo Next steps:
echo   Authenticate with TeamCity
echo   teamcity auth login
echo.
echo   List recent builds
echo   teamcity run list
echo.
echo   Get help
echo   teamcity --help
echo.

endlocal
