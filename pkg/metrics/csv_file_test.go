package metrics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestIncrementSignalCounter(t *testing.T) {
	t.Chdir(t.TempDir())
	now := time.Now().UTC()
	path := fmt.Sprintf(DefaultMetricsFileSignals, now.Format(time.DateOnly))

	// Test 1: "metrics" directory does not exist - no file, but also no runtime effect.
	incrementSignalCounterAsSideEffect(nil, "signal1")

	_, err := os.ReadFile(path) //gosec:disable G304 // Unit test with fake files.
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}

	// Test 2: "metrics" directory exists - file should be created and appended.
	if err := os.Mkdir("metrics", 0o700); err != nil {
		t.Fatal(err)
	}

	incrementSignalCounterAsSideEffect(nil, "signal1")
	incrementSignalCounterAsSideEffect(nil, "signal2")
	incrementSignalCounterAsSideEffect(nil, "signal3")

	f, err := os.ReadFile(path) //gosec:disable G304 // Unit test with fake files.
	if err != nil {
		t.Fatal(err)
	}

	got := string(f)
	ts := now.Format(time.RFC3339)
	// Ensure timestamps are deterministic for test comparison.
	got = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`).ReplaceAllString(got, ts)

	want := fmt.Sprintf("%s,signal1\n%s,signal2\n%s,signal3\n", ts, ts, ts)
	if got != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
}
