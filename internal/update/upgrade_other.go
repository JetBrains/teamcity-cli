//go:build !windows

package update

import (
	"context"
	"errors"
)

func startWinGetUpgrade(context.Context, []string) (UpgradeResult, error) {
	return UpgradeResult{}, errors.New("WinGet update handoff requires Windows")
}
