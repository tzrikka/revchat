package slack

import (
	"testing"
	"time"
)

func TestReminderTimes(t *testing.T) {
	startTime := time.Date(2025, 12, 20, 13, 0, 0, 0, time.UTC)

	_, gotTime, gotErr := reminderTimes(nil, startTime, "userID", "8:00AM America/New_York")
	if gotErr != nil {
		t.Fatalf("reminderTimes() error: %v", gotErr)
	}

	hours, mins, secs := gotTime.Clock()
	if hours != 8 || mins != 0 || secs != 0 {
		t.Errorf("reminderTimes() = %v, want 08:00:00", gotTime)
	}
}

func TestTimeSince(t *testing.T) {
	now := time.Date(2025, 11, 10, 20, 30, 40, 50, time.UTC)

	tests := []struct {
		name      string
		timestamp any
		want      string
	}{
		{
			name:      "nil",
			timestamp: nil,
			want:      "",
		},
		{
			name:      "empty",
			timestamp: "",
			want:      "",
		},
		{
			name:      "within_hours",
			timestamp: "2025-11-10T18:15:40.000000+00:00",
			want:      "2h15m",
		},
		{
			name:      "almost_1_day",
			timestamp: "2025-11-09T20:32:00.000000+00:00",
			want:      "23h59m",
		},
		{
			name:      "1_day",
			timestamp: "2025-11-09T20:31:00.000000+00:00",
			want:      "1d",
		},
		{
			name:      "over_days",
			timestamp: "2025-10-13T21:26:20.352995+00:00",
			want:      "27d23h4m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeSince(now, tt.timestamp)
			if got != tt.want {
				t.Errorf("timeSince(%v) = %q, want %q", tt.timestamp, got, tt.want)
			}
		})
	}
}
