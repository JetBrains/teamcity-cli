//go:build staging

package analytics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// TestStagingRealSend posts every declared event to the FUS staging endpoint.
// Gated by build tag `staging`; run with:
//
//	go test -tags=staging -count=1 -run TestStagingRealSend ./internal/analytics/...
func TestStagingRealSend(t *testing.T) {
	stagingCfg, err := fus.FetchTestConfig(RecorderID, ProductCode)
	if err != nil {
		t.Logf("FetchTestConfig: %v — falling back to direct send URL", err)
		stagingCfg = &fus.FUSConfig{SendEndpoint: "https://stgn.fus-stgn.aws.intellij.net/tcx/v4/send/"}
	}
	t.Logf("staging endpoint: %s", stagingCfg.SendEndpoint)

	salt := os.Getenv(EnvSalt)
	if salt == "" {
		salt = "tcx-staging-probe-placeholder"
		t.Logf("using placeholder salt; set %s to override", EnvSalt)
	}

	validator, err := fus.NewValidator(Scheme)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	dir := t.TempDir()
	logger, err := fus.NewLogger(
		t.Context(),
		fus.RecorderConfig{
			RecorderID:        RecorderID,
			RecorderVersion:   RecorderVersion,
			ProductCode:       ProductCode,
			BuildVersion:      "0.0.1-staging-probe",
			DataDir:           dir,
			AnonymizationSalt: salt,
		},
		fus.WithFUSConfig(stagingCfg),
		fus.WithValidator(validator),
	)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	for _, ev := range SampleEvents() {
		logger.Track(ev.Group, ev.Event.ID, ev.Event.Data)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := logger.Flush(ctx); err != nil {
		t.Fatalf("flush to staging: %v", err)
	}

	if info, err := os.Stat(dir + "/fus_buffer.jsonl"); err == nil && info.Size() != 0 {
		t.Errorf("buffer non-empty after flush (%d bytes) — server may have rejected some events", info.Size())
	}

	fmt.Printf("\n[staging probe] sent %d events to %s\n", len(SampleEvents()), stagingCfg.SendEndpoint)
}
