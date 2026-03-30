package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tzrikka/revchat/pkg/config"
	"github.com/tzrikka/xdg"
)

func TestDataPath(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	for _, subdir := range []string{"new", "existing_empty", "existing_non_empty"} {
		if err := os.MkdirAll(filepath.Join(d, config.DirName, subdir), xdg.NewDirectoryPermissions); err != nil {
			t.Fatalf("os.MkdirAll() error = %v", err)
		}
	}

	for _, filename := range []string{"users.json", "other.json", "file.txt"} {
		if err := os.WriteFile(filepath.Join(d, config.DirName, "existing_empty", filename), nil, xdg.NewFilePermissions); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
	}

	name := filepath.Join(d, config.DirName, "existing_non_empty/users.json")
	if err := os.WriteFile(name, []byte("{\"k\":1}\n"), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	name = filepath.Join(d, config.DirName, "existing_non_empty/other.json")
	if err := os.WriteFile(name, []byte("[{\"a\":1},{\"b\":2}]\n"), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	name = filepath.Join(d, config.DirName, "existing_non_empty/file.txt")
	if err := os.WriteFile(name, []byte("hello\n"), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	tests := []struct {
		name         string
		relativePath string
		wantContent  string
	}{
		{
			name:         "new_users_json_file",
			relativePath: "new/users.json",
			wantContent:  "[]\n",
		},
		{
			name:         "new_other_json_file",
			relativePath: "new/other.json",
			wantContent:  "{}\n",
		},
		{
			name:         "new_non_json_text_file",
			relativePath: "new/file.txt",
			wantContent:  "",
		},

		{
			name:         "existing_empty_users_json_file",
			relativePath: "existing_empty/users.json",
			wantContent:  "[]\n",
		},
		{
			name:         "existing_empty_other_json_file",
			relativePath: "existing_empty/other.json",
			wantContent:  "{}\n",
		},
		{
			name:         "existing_empty_non_json_text_file",
			relativePath: "existing_empty/file.txt",
			wantContent:  "",
		},

		{
			name:         "existing_non_empty_users_json_file",
			relativePath: "existing_non_empty/users.json",
			wantContent:  "{\"k\":1}\n",
		},
		{
			name:         "existing_non_empty_other_json_file",
			relativePath: "existing_non_empty/other.json",
			wantContent:  "[{\"a\":1},{\"b\":2}]\n",
		},
		{
			name:         "existing_non_empty_non_json_text_file",
			relativePath: "existing_non_empty/file.txt",
			wantContent:  "hello\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantPath := filepath.Join(d, config.DirName, tt.relativePath)

			gotPath, err := dataPath(tt.relativePath)
			if err != nil {
				t.Fatalf("dataPath() error = %v", err)
			}
			if gotPath != wantPath {
				t.Fatalf("dataPath() path = %q, want %q", gotPath, wantPath)
			}

			gotContent, err := os.ReadFile(gotPath) //gosec:disable G304 // Unit test.
			if err != nil {
				t.Fatalf("os.ReadFile() error = %v", err)
			}
			if string(gotContent) != tt.wantContent {
				t.Errorf("file content = %q, want %q", string(gotContent), tt.wantContent)
			}
		})
	}
}

func TestDeleteGenericPRFile(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)

	name := filepath.Join(d, config.DirName, "file.json")
	if err := os.MkdirAll(filepath.Dir(name), xdg.NewDirectoryPermissions); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(name, []byte("{\"k\":1}\n"), xdg.NewFilePermissions); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if err := DeleteGenericPRFile(t.Context(), "file.json"); err != nil {
		t.Fatalf("DeleteGenericPRFile() error = %v", err)
	}

	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("file still exists after DeleteGenericPRFile(): %v", err)
	}

	// Deleting again should not return an error.
	if err := DeleteGenericPRFile(t.Context(), "file.json"); err != nil {
		t.Fatalf("DeleteGenericPRFile() error on non-existent file = %v", err)
	}
}
