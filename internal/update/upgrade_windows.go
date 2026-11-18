package update

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startWinGetUpgrade(ctx context.Context, args []string) (UpgradeResult, error) {
	if err := ctx.Err(); err != nil {
		return UpgradeResult{}, err
	}
	manager, err := exec.LookPath("winget")
	if err != nil {
		return UpgradeResult{}, err
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return UpgradeResult{}, err
	}
	log, err := os.CreateTemp("", "teamcity-update-*.log")
	if err != nil {
		return UpgradeResult{}, err
	}
	logPath := log.Name()
	if err := log.Close(); err != nil {
		_ = os.Remove(logPath)
		return UpgradeResult{}, err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = os.Remove(logPath)
		}
	}()

	eventName := `Local\TeamCityUpdate-` + rand.Text()
	eventNameUTF16, err := windows.UTF16PtrFromString(eventName)
	if err != nil {
		return UpgradeResult{}, err
	}
	ready, err := windows.CreateEvent(nil, 1, 0, eventNameUTF16)
	if err != nil {
		return UpgradeResult{}, err
	}
	defer func() { _ = windows.CloseHandle(ready) }()

	script := winGetUpgradeScript(os.Getpid(), eventName, manager, args, logPath)
	encodedBytes := make([]byte, 0, 2*len(script))
	for _, value := range utf16.Encode([]rune(script)) {
		encodedBytes = binary.LittleEndian.AppendUint16(encodedBytes, value)
	}
	encoded := base64.StdEncoding.EncodeToString(encodedBytes)
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{
		filepath.Join(systemDir, "WindowsPowerShell", "v1.0", "powershell.exe"),
		"-NoLogo", "-NoProfile", "-NoExit", "-EncodedCommand", encoded,
	}))
	if err != nil {
		return UpgradeResult{}, err
	}
	directory, err := windows.UTF16PtrFromString(os.TempDir())
	if err != nil {
		return UpgradeResult{}, err
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var process windows.ProcessInformation
	// Let the new console own its standard handles, independently of CLI output pipes.
	if err := windows.CreateProcess(nil, commandLine, nil, nil, false, windows.CREATE_NEW_CONSOLE, nil, directory, &startup, &process); err != nil {
		return UpgradeResult{}, fmt.Errorf("start update window: %w", err)
	}
	defer func() {
		if !handedOff {
			_ = windows.TerminateProcess(process.Process, 1)
			_, _ = windows.WaitForSingleObject(process.Process, 5000)
		}
		_ = windows.CloseHandle(process.Thread)
		_ = windows.CloseHandle(process.Process)
	}()

	startupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		if err := startupCtx.Err(); err != nil {
			return UpgradeResult{}, fmt.Errorf("wait for update window: %w", err)
		}
		status, err := windows.WaitForMultipleObjects([]windows.Handle{ready, process.Process}, false, 100)
		if err != nil {
			return UpgradeResult{}, fmt.Errorf("wait for update window: %w", err)
		}
		switch status {
		case windows.WAIT_OBJECT_0:
			if err := startupCtx.Err(); err != nil {
				return UpgradeResult{}, fmt.Errorf("wait for update window: %w", err)
			}
			handedOff = true
			return UpgradeResult{Deferred: true, LogPath: logPath}, nil
		case windows.WAIT_OBJECT_0 + 1:
			return UpgradeResult{}, errors.New("update window exited before it was ready")
		}
	}
}

func winGetUpgradeScript(parentID int, eventName, manager string, args []string, logPath string) string {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	command := []string{quote(manager)}
	for _, arg := range args {
		command = append(command, quote(arg))
	}
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$Host.UI.RawUI.WindowTitle = 'TeamCity CLI update'
try {
    Start-Transcript -LiteralPath %s -Append | Out-Null
    $parent = [System.Diagnostics.Process]::GetProcessById(%d)
    $parent.Handle | Out-Null
    $ready = [System.Threading.EventWaitHandle]::OpenExisting(%s)
    $ready.Set() | Out-Null
    $ready.Dispose()
    Write-Host 'Waiting for TeamCity CLI to exit...'
    $exited = $parent.WaitForExit(15000)
    $parent.Dispose()
    if (-not $exited) {
        throw 'TeamCity CLI did not exit within 15 seconds. Update canceled.'
    }
    & %s
    if ($LASTEXITCODE -ne 0) {
        throw ('WinGet failed with exit code 0x{0:X8}.' -f ($LASTEXITCODE -band 0xffffffffL))
    }
    Write-Host "Update completed. Run 'teamcity --version' to check the installed version."
    Write-Host 'You can close this window.'
} catch {
    Write-Host ('Error: ' + $_.Exception.Message) -ForegroundColor Red
} finally {
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
}
`, quote(logPath), parentID, quote(eventName), strings.Join(command, " "))
}
