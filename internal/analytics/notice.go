package analytics

import (
	"fmt"
	"io"
)

const NoticeText = `Anonymous usage statistics help improve TeamCity CLI.
To disable: teamcity config set analytics false
            or set DO_NOT_TRACK=1
Learn more: https://jb.gg/tc/analytics
Data collection terms: https://www.jetbrains.com/legal/docs/terms/product_data_collection/

`

// PrintFirstRunNotice writes the one-time notice; suppressed when shown, opted out, quiet, or no-input.
func PrintFirstRunNotice(errOut io.Writer, alreadyShown, optedOut, quiet, noInput bool) bool {
	if alreadyShown || optedOut || quiet || noInput {
		return false
	}
	_, _ = fmt.Fprint(errOut, NoticeText)
	return true
}
