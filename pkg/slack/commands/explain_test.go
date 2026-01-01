package commands

import (
	"testing"
)

func TestOwnerMention(t *testing.T) {
	tests := []struct {
		name      string
		owner     string
		approvers map[string]bool
		want      string
	}{
		{
			name:  "is_approver_once",
			owner: "<@alice>",
			approvers: map[string]bool{
				"<@alice>": false,
			},
			want: "<@alice> :+1:",
		},
		{
			name:  "is_approver_multiple_times",
			owner: "<@alice>",
			approvers: map[string]bool{
				"<@alice>": true,
			},
			want: "<@alice> :+1:",
		},
		{
			name:  "isnt_approver",
			owner: "<@bob>",
			approvers: map[string]bool{
				"<@alice>": false,
			},
			want: "<@bob>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ownerMention(nil, tt.owner, tt.approvers)
			if got != tt.want {
				t.Errorf("ownerMention(%q) = %q, want %q", tt.owner, got, tt.want)
			}
			if tt.approvers[tt.owner] != (tt.want == tt.owner+" :+1:") {
				t.Errorf("approvers[%q] = %v, want %v", tt.owner, tt.approvers[tt.owner], tt.want == tt.owner+" :+1:")
			}
		})
	}
}
