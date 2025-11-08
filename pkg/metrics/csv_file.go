// Package metrics provides functions to record metrics data.
// It is a very thin layer over OpenTelemetry, but it can
// also write logs to local files for simple setups.
package metrics

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/tzrikka/revchat/internal/log"
	"github.com/tzrikka/xdg"
)

const (
	DefaultMetricsFileSignals = "metrics/revchat_signals_%s.csv"

	fileFlags = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	filePerms = xdg.NewFilePermissions
)

var muSignals sync.Mutex

// IncrementSignalCounter monitors incoming Temporal signals
// (triggered by webhook events which were received by Timpani).
func IncrementSignalCounter(ctx workflow.Context, name string) {
	muSignals.Lock()
	defer muSignals.Unlock()

	now := time.Now().UTC()
	path := fmt.Sprintf(DefaultMetricsFileSignals, now.Format(time.DateOnly))
	if err := AppendToCSVFile(path, []string{now.Format(time.RFC3339), name}); err != nil && ctx != nil {
		log.Error(ctx, "metrics error: failed to increment signal counter", "signal", name, "error", err)
	}
}

func AppendToCSVFile(path string, record []string) error {
	f, err := os.OpenFile(path, fileFlags, filePerms) //gosec:disable G304 -- hardcoded path
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
