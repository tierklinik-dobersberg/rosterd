package timecalc

import "time"

func StartOfWeek(t time.Time) time.Time {
	if t.Weekday() == time.Monday {
		return t
	}

	var iter = t
	for ; iter.Weekday() != time.Monday; iter = iter.AddDate(0, 0, -1) {
	}

	return iter
}

func EndOfWeek(t time.Time) time.Time {
	if t.Weekday() == time.Sunday {
		return t
	}

	var iter = t
	for ; iter.Weekday() != time.Sunday; iter = iter.AddDate(0, 0, 1) {
	}

	return iter
}
