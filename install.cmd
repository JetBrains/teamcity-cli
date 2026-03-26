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

set "RELEASE=%~1"
set "INSTALL_DIR=%~2"
if "!INSTALL_DIR!"=="" set "INSTALL_DIR=%USERPROFILE%\.local\bin"
set "BIN_NAME=teamcity.exe"
set "REPO=JetBrains/teamcity-cli"

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
echo To install a specific version: install.cmd v0.7.0
echo.

:: Check curl is available (ships with Windows 10+)
curl --version >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo Error: curl is required but not found. Use install.ps1 instead. >&2
    exit /b 1
)

:: Resolve latest release via 302 redirect if no version specified
if "!RELEASE!"=="" (
    for /f "delims=" %%a in ('curl -s -o nul -w "%%{redirect_url}" "https://github.com/!REPO!/releases/latest"') do set "LOCATION=%%a"
    if "!LOCATION!"=="" (
        echo Error: failed to resolve latest release >&2
        exit /b 1
    )
    for %%t in ("!LOCATION!") do set "RELEASE=%%~nxt"
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
set "URL=https://github.com/!REPO!/releases/download/!RELEASE!/!ASSET_NAME!"

echo Installing teamcity (!RELEASE!) from !URL!
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
    if exist "!TEMP_ZIP!" del "!TEMP_ZIP!"
    exit /b 1
)

:: Extract using tar (ships with Windows 10+, handles zip)
mkdir "!TEMP_DIR!"
tar -xf "!TEMP_ZIP!" -C "!TEMP_DIR!"
if !ERRORLEVEL! neq 0 (
    echo Error: extraction failed >&2
    del "!TEMP_ZIP!" 2>nul
    rmdir /s /q "!TEMP_DIR!" 2>nul
    exit /b 1
)

:: Find the binary
set "FOUND_BIN="
for /r "!TEMP_DIR!" %%f in (!BIN_NAME!) do (
    if exist "%%f" set "FOUND_BIN=%%f"
)

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
