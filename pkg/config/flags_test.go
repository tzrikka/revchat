package config

import (
	"path/filepath"
	"testing"
)

func TestConfigDirAndFile(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)

	got := configFile()
	want := filepath.Join(d, DirName, ConfigFileName)
	if got.SourceURI() != want {
		t.Errorf("configFile() = %q, want %q", got.SourceURI(), want)
	}
}
