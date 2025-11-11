package slack

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var timePattern = regexp.MustCompile(`^\d{1,2}(:\d{2})?$`)

// normalizeTime takes a time string and an optional am/pm indicator,
// and returns the time in a standardized format or an error if invalid.
func normalizeTime(timeStr, amPm string) (string, error) {
	if !timePattern.MatchString(timeStr) {
		return "", fmt.Errorf("invalid time format: %s", timeStr)
	}

	switch len(timeStr) {
	case 1:
		timeStr = fmt.Sprintf("0%s:00", timeStr)
	case 2:
		timeStr += ":00"
	case 4:
		timeStr = "0" + timeStr
	}

	amPm = strings.ToUpper(amPm)
	layout := time.Kitchen
	switch len(amPm) {
	case 0:
		layout = "15:04" // 24-hour format.
	case 1:
		amPm += "M"
	}

	timeStr += amPm
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return "", err
	}

	return t.Format(time.Kitchen), nil
}

func timeSince(now time.Time, timestamp any) string {
	s, ok := timestamp.(string)
	if !ok || s == "" {
		return ""
	}

	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}

	d := now.Sub(t).Round(time.Minute)
	if d.Hours() < 24 {
		return strings.TrimSpace(strings.TrimSuffix(strings.Replace(d.String(), "h", "h ", 1), "0s"))
	}

	days := int(d.Hours()) / 24
	d -= time.Hour * time.Duration(24*days)
	s = fmt.Sprintf("%dd %s", days, d)
	return strings.TrimSpace(strings.TrimSuffix(strings.ReplaceAll(s, "h", "h "), "0s"))
}
