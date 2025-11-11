package slack

import (
	"testing"
	"time"
)

func TestNormalizeTime(t *testing.T) {
	tests := []struct {
		name    string
		timeStr string
		amPm    string
		want    string
		wantErr bool
	}{
		{
			name:    "0",
			timeStr: "0",
			want:    "12:00AM",
		},
		{
			name:    "1",
			timeStr: "1",
			want:    "1:00AM",
		},
		{
			name:    "02",
			timeStr: "02",
			want:    "2:00AM",
		},
		{
			name:    "13",
			timeStr: "13",
			want:    "1:00PM",
		},
		{
			name:    "24",
			timeStr: "24",
			wantErr: true,
		},

		{
			name:    "0_00",
			timeStr: "0:00",
			want:    "12:00AM",
		},
		{
			name:    "1_01",
			timeStr: "1:01",
			want:    "1:01AM",
		},
		{
			name:    "02_59",
			timeStr: "02:59",
			want:    "2:59AM",
		},
		{
			name:    "13_30",
			timeStr: "13:30",
			want:    "1:30PM",
		},
		{
			name:    "14_60",
			timeStr: "14:60",
			wantErr: true,
		},
		{
			name:    "25_00",
			timeStr: "25:00",
			wantErr: true,
		},

		{
			name:    "0_a",
			timeStr: "0",
			amPm:    "a",
			want:    "12:00AM",
		},
		{
			name:    "1_a",
			timeStr: "1",
			amPm:    "a",
			want:    "1:00AM",
		},
		{
			name:    "02_a",
			timeStr: "02",
			amPm:    "a",
			want:    "2:00AM",
		},

		{
			name:    "0_p",
			timeStr: "0",
			amPm:    "p",
			want:    "12:00PM",
		},
		{
			name:    "1_p",
			timeStr: "1",
			amPm:    "p",
			want:    "1:00PM",
		},
		{
			name:    "02_p",
			timeStr: "02",
			amPm:    "p",
			want:    "2:00PM",
		},
		{
			name:    "11_pm",
			timeStr: "11",
			amPm:    "pm",
			want:    "11:00PM",
		},
		{
			name:    "12_p",
			timeStr: "12",
			amPm:    "p",
			want:    "12:00PM",
		},
		{
			name:    "13_p",
			timeStr: "13",
			amPm:    "p",
			wantErr: true,
		},

		{
			name:    "0_00_am",
			timeStr: "0:00",
			amPm:    "am",
			want:    "12:00AM",
		},
		{
			name:    "1_01_am",
			timeStr: "1:01",
			amPm:    "am",
			want:    "1:01AM",
		},
		{
			name:    "02_59_am",
			timeStr: "02:59",
			amPm:    "am",
			want:    "2:59AM",
		},
		{
			name:    "13_30_am",
			timeStr: "13:30",
			amPm:    "am",
			wantErr: true,
		},
		{
			name:    "14_60_am",
			timeStr: "14:60",
			amPm:    "am",
			wantErr: true,
		},
		{
			name:    "25_00_am",
			timeStr: "25:00",
			amPm:    "am",
			wantErr: true,
		},

		{
			name:    "0_00_pm",
			timeStr: "0:00",
			amPm:    "pm",
			want:    "12:00PM",
		},
		{
			name:    "1_01_pm",
			timeStr: "1:01",
			amPm:    "pm",
			want:    "1:01PM",
		},
		{
			name:    "02_59_pm",
			timeStr: "02:59",
			amPm:    "pm",
			want:    "2:59PM",
		},
		{
			name:    "13_30_pm",
			timeStr: "13:30",
			amPm:    "pm",
			wantErr: true,
		},
		{
			name:    "14_60_pm",
			timeStr: "14:60",
			amPm:    "pm",
			wantErr: true,
		},
		{
			name:    "25_00_pm",
			timeStr: "25:00",
			amPm:    "pm",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTime(tt.timeStr, tt.amPm)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeTime() error: %v, want %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("normalizeTime() = %q, want %q", got, tt.want)
			}
		})
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
			want:      "2h 15m",
		},
		{
			name:      "almost_1_day",
			timestamp: "2025-11-09T20:32:00.000000+00:00",
			want:      "23h 59m",
		},
		{
			name:      "1_day",
			timestamp: "2025-11-09T20:31:00.000000+00:00",
			want:      "1d",
		},
		{
			name:      "over_days",
			timestamp: "2025-10-13T21:26:20.352995+00:00",
			want:      "27d 23h 4m",
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
