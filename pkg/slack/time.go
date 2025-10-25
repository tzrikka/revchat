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
		timeStr = fmt.Sprintf("%s:00", timeStr)
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
