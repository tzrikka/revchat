package data

import (
	"reflect"
	"testing"
)

func TestReadWriteJSON(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_DATA_HOME", d)
	pathCache = map[string]string{} // Reset global state.

	filename := "test.json"

	got, err := readJSON(filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("readJSON() = %v, want empty map", got)
	}

	want := map[string]string{"key": "value"}
	if err := writeJSON(filename, want); err != nil {
		t.Fatal(err)
	}

	got, err = readJSON(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readJSON() = %v, want %v", got, want)
	}
}
