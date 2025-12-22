package config

import (
	"log/slog"
	"strings"
)

// KVSliceToMap converts a slice of "key=value" strings into a map.
// Invalid entries are logged and skipped.
func KVSliceToMap(pairs []string) map[string]string {
	m := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			slog.Error("invalid key-value pair in linkification map configuration", slog.String("kv", kv))
			continue
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return m
}
