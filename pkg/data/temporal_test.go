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

	got, err := readJSONActivity(t.Context(), filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("readJSON() = %v, want empty map", got)
	}

	want := map[string]string{"key": "value"}
	if err := writeJSONActivity(t.Context(), filename, want); err != nil {
		t.Fatal(err)
	}

	got, err = readJSONActivity(t.Context(), filename)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readJSONActivity() = %v, want %v", got, want)
	}
}
