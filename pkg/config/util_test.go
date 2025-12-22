package config_test

import (
	"reflect"
	"testing"

	"github.com/tzrikka/revchat/pkg/config"
)

func TestKVSliceToMap(t *testing.T) {
	tests := []struct {
		name  string
		pairs []string
		want  map[string]string
	}{
		{
			name:  "empty",
			pairs: []string{},
			want:  map[string]string{},
		},
		{
			name:  "single pair",
			pairs: []string{"key=value"},
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "multiple pairs",
			pairs: []string{"key1=value1", "key2=value2"},
			want:  map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:  "invalid pair",
			pairs: []string{"key1=value1", "invalid", "key2=value2"},
			want:  map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.KVSliceToMap(tt.pairs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("KVSliceToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
