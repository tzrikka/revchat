// Package metrics provides functions to record metrics data.
// It is a very thin layer over OpenTelemetry, but it can
// also write logs to local files for simple setups.
package metrics

import (
	"encoding/csv"
	"os"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/xdg"
)

const (
	DefaultMetricsFileSignals = "revchat_metrics_signals.csv"

	fileFlags = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

var muSignals sync.Mutex

// IncrementSignalCounter monitors incoming Temporal signals
// (triggered by webhook events which were received by Timpani).
func IncrementSignalCounter(ctx workflow.Context, name string) {
	muSignals.Lock()
	defer muSignals.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if err := AppendToCSVFile(DefaultMetricsFileSignals, []string{now, name}); err != nil {
		log.Error(ctx, "metrics error: failed to increment signal counter", "signal", name, "error", err)
	}
}

func AppendToCSVFile(path string, record []string) error {
	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- false positive
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(record); err != nil {
		return err
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	return nil
}
