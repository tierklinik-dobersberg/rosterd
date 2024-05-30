package timecalc_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	"github.com/tierklinik-dobersberg/cis/pkg/daytime"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"github.com/tierklinik-dobersberg/rosterd/internal/timecalc"
)

func Test_GatherWorkDaysByMonth(t *testing.T) {
	holidays := map[string]*calendarv1.PublicHoliday{
		"2024-05-01": {
			Name: "Staatsfeiertag",
			Type: calendarv1.HolidayType_PUBLIC,
		},
		"2024-05-09": {
			Name: "Christi Himmelfahrt",
			Type: calendarv1.HolidayType_PUBLIC,
		},
		"2024-05-20": {
			Name: "Phfingsmontag",
			Type: calendarv1.HolidayType_PUBLIC,
		},
		"2024-05-30": {
			Name: "Fronleichnam",
			Type: calendarv1.HolidayType_PUBLIC,
		},
		"2024-05-17": { // This one should be ignored, only PUBLIC holidays are counted
			Name: "Does-Not-Exist",
			Type: calendarv1.HolidayType_BANK,
		},
	}

	cases := []struct {
		from                   string
		to                     string
		expectedWorkDays       int
		expectedNumberOfMonths int
		errorExpected          bool
	}{
		{
			from:                   "2024-05-01",
			to:                     "2024-05-31",
			expectedWorkDays:       19,
			expectedNumberOfMonths: 1,
		},
		{
			from:                   "2024-05-09",
			to:                     "2024-06-10",
			expectedWorkDays:       20,
			expectedNumberOfMonths: 2,
		},
		{
			from:                   "2024-05-01",
			to:                     "2024-05-01",
			expectedWorkDays:       0,
			expectedNumberOfMonths: 1,
		},
		{
			from:                   "2024-05-01",
			to:                     "2024-05-02",
			expectedWorkDays:       1,
			expectedNumberOfMonths: 1,
		},
		{
			from:                   "2024-06-01",
			to:                     "2024-06-30",
			expectedWorkDays:       20,
			expectedNumberOfMonths: 1,
		},
		{
			from:          "2024-06",
			to:            "2024-06-30",
			errorExpected: true,
		},
		{
			from:          "2024-06-01",
			to:            "2024-06",
			errorExpected: true,
		},
	}

	for idx, testCase := range cases {
		title := fmt.Sprintf("#%d %s to %s (expected %d)", idx, testCase.from, testCase.to, testCase.expectedWorkDays)
		t.Run(title, func(t *testing.T) {
			result, err := timecalc.GatherWorkDaysByMonth(holidays, testCase.from, testCase.to)

			if testCase.errorExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				require.Equal(t, testCase.expectedNumberOfMonths, len(result))

				total := 0
				for _, mwd := range result {
					log.Printf("%s", mwd)
					total += len(mwd.WorkDays)
				}

				require.Equal(t, testCase.expectedWorkDays, total)
			}
		})
	}
}

func Test_WorkTimeList(t *testing.T) {
	list := timecalc.WorkTimeList{
		{
			UserID:         "may",
			ApplicableFrom: time.Date(2024, time.May, 1, 0, 0, 0, 0, time.Local),
			EndsWith:       time.Date(2024, time.May, 31, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:         "june",
			ApplicableFrom: time.Date(2024, time.June, 10, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:         "july",
			ApplicableFrom: time.Date(2024, time.July, 1, 0, 0, 0, 0, time.Local),
		},
	}

	cases := []struct {
		t        string
		expected string
	}{
		{
			t:        "2024-05-01",
			expected: "may",
		},
		{
			t:        "2024-05-31",
			expected: "may",
		},
		{
			t:        "2024-06-05",
			expected: "",
		},
		{
			t:        "2024-06-20",
			expected: "june",
		},
		{
			t:        "2024-07-02",
			expected: "july",
		},
	}

	for idx, testCase := range cases {
		title := fmt.Sprintf("#%d %s->%s", idx, testCase.t, testCase.expected)

		t.Run(title, func(t *testing.T) {
			tt, err := time.ParseInLocation("2006-01-02", testCase.t, time.Local)
			require.NoError(t, err)

			res, ok := list.FindForDate(tt)
			if testCase.expected == "" {
				require.False(t, ok)
			} else {
				require.True(t, ok)
				require.Equal(t, testCase.expected, res.UserID)
			}
		})
	}
}

func Test_CalculateExpectedWorkTime(t *testing.T) {
	workTimes := timecalc.WorkTimeList{
		{
			UserID:         "may",
			ApplicableFrom: time.Date(2024, time.May, 1, 0, 0, 0, 0, time.Local),
			TimePerWeek:    40 * time.Hour,
			EndsWith:       time.Date(2024, time.May, 31, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:         "june",
			TimePerWeek:    20 * time.Hour,
			ApplicableFrom: time.Date(2024, time.June, 1, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:         "july",
			TimePerWeek:    10 * time.Hour,
			ApplicableFrom: time.Date(2024, time.July, 10, 0, 0, 0, 0, time.Local),
			EndsWith:       time.Date(2024, time.July, 30, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:                  "august",
			TimePerWeek:             10 * time.Hour,
			ApplicableFrom:          time.Date(2024, time.August, 1, 0, 0, 0, 0, time.Local),
			ExcludeFromTimeTracking: true,
		},
	}

	cases := []struct {
		days              []timecalc.MonthlyWorkDays
		expectedTracked   string
		expectedUntracked string
		errorExpected     bool
		from              string
		to                string
	}{
		{
			days: []timecalc.MonthlyWorkDays{
				{
					Year:     2024,
					Month:    time.May,
					WorkDays: []int{2, 3, 6, 7, 8, 10, 13, 14, 15, 16, 17, 21, 22, 23, 24, 27, 28, 29, 31},
				},
			},
			expectedTracked: "152h",
		},
		{
			days: []timecalc.MonthlyWorkDays{
				{
					Year:     2024,
					Month:    time.May,
					WorkDays: []int{2, 3, 6, 7, 8, 10, 13, 14, 15, 16, 17, 21, 22, 23, 24, 27, 28, 29, 31},
				},
				{
					Year:     2024,
					Month:    time.June,
					WorkDays: []int{3, 4, 5, 6, 7, 10, 11, 12, 13, 14, 17, 18, 19, 20, 21, 24, 25, 26, 27, 28},
				},
			},
			expectedTracked: "232h",
		},
		{
			days: []timecalc.MonthlyWorkDays{
				{
					Year:     2024,
					Month:    time.July,
					WorkDays: []int{1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 15, 16, 17, 18, 19, 22, 23, 24, 25, 26, 29, 30, 31},
				},
			},
			expectedTracked: "60h",
		},
		{
			days: []timecalc.MonthlyWorkDays{
				{
					Year:     2024,
					Month:    time.August,
					WorkDays: []int{1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 15, 16, 17, 18, 19, 22, 23, 24, 25, 26, 29, 30, 31},
				},
			},
			expectedTracked:   "0h",
			expectedUntracked: "46h",
		},
		{
			days: []timecalc.MonthlyWorkDays{
				{
					Year:     2024,
					Month:    time.July,
					WorkDays: []int{1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 15, 16, 17, 18, 19, 22, 23, 24, 25, 26, 29, 30, 31},
				},
				{
					Year:     2024,
					Month:    time.August,
					WorkDays: []int{1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 15, 16, 17, 18, 19, 22, 23, 24, 25, 26, 29, 30, 31},
				},
			},
			expectedTracked:   "60h",
			expectedUntracked: "46h",
		},
		{
			from:          "2006-01",
			errorExpected: true,
		},
		{
			to:            "2006-01",
			errorExpected: true,
		},
	}

	for idx, testCase := range cases {
		title := fmt.Sprintf("#%d", idx)
		t.Run(title, func(t *testing.T) {

			result, err := timecalc.CalculateExpectedWorkTime(testCase.days, map[string]timecalc.WorkTimeList{
				"bob": workTimes,
			}, testCase.from, testCase.to)

			if testCase.errorExpected {
				require.Error(t, err)
			} else {
				expectedTracked, testCaseDurationErr := time.ParseDuration(testCase.expectedTracked)
				require.NoError(t, testCaseDurationErr)

				require.NoError(t, err)

				require.Len(t, result, 1)
				_, ok := result["bob"]
				require.True(t, ok)

				total := result["bob"].TotalTrackedWorkTime()

				require.Equal(t, expectedTracked, total)

				if testCase.expectedUntracked != "" {
					expectedUntracked, testCaseDurationErr := time.ParseDuration(testCase.expectedUntracked)
					require.NoError(t, testCaseDurationErr)

					totalUntracked := result["bob"].TotalWorkTime() - result["bob"].TotalTrackedWorkTime()
					require.Equal(t, expectedUntracked, totalUntracked)
				}
			}
		})
	}
}

func Test_CalculatePlannedMonthlyWorkTime(t *testing.T) {
	workTimes := timecalc.WorkTimeList{
		{
			UserID:         "may",
			ApplicableFrom: time.Date(2024, time.May, 1, 0, 0, 0, 0, time.Local),
			TimePerWeek:    40 * time.Hour,
			EndsWith:       time.Date(2024, time.May, 31, 0, 0, 0, 0, time.Local),
		},
		{
			UserID:         "june",
			TimePerWeek:    20 * time.Hour,
			ApplicableFrom: time.Date(2024, time.June, 1, 0, 0, 0, 0, time.Local),
			EndsWith:       time.Date(2024, time.June, 15, 0, 0, 0, 0, time.Local),
		},
	}

	makeTime := func(month time.Month, date int, timeString string) time.Time {
		dt, err := daytime.ParseDayTime(timeString)
		require.NoError(t, err)

		return time.Date(2024, month, date, 0, 0, 0, 0, time.Local).Add(dt.AsDuration())
	}

	makeShift := func(month time.Month, date int, fromString string, toString string, users ...string) structs.PlannedShift {
		from := makeTime(month, date, fromString)
		to := makeTime(month, date, toString)

		return structs.PlannedShift{
			From:            from,
			To:              to,
			AssignedUserIds: users,
		}
	}

	rosterMay := structs.DutyRoster{
		From: "2024-05-01",
		To:   "2024-05-31",
		Shifts: []structs.PlannedShift{
			makeShift(time.May, 1, "08:00", "12:00", "bob", "alice"),
			makeShift(time.May, 1, "14:00", "17:00", "bob"),
			makeShift(time.May, 2, "08:00", "12:00", "alice"),
			makeShift(time.May, 2, "14:00", "17:00", "alice"),
		},
	}

	rosterJune := structs.DutyRoster{
		From: "2024-06-01",
		To:   "2024-06-30",
		Shifts: []structs.PlannedShift{
			makeShift(time.June, 1, "08:00", "12:00", "bob", "alice"),
			makeShift(time.June, 1, "14:00", "17:00", "bob"),
			makeShift(time.June, 2, "08:00", "12:00", "alice"),
			makeShift(time.June, 2, "14:00", "17:00", "alice"),
			makeShift(time.June, 15, "08:00", "12:00", "alice"),
			makeShift(time.June, 15, "14:00", "17:00", "alice"),

			// no work-time definitions for this days
			makeShift(time.June, 16, "08:00", "12:00", "alice"),
			makeShift(time.June, 16, "14:00", "17:00", "alice"),
		},
	}

	cases := []struct {
		from      string
		to        string
		tracked   map[string]string
		untracked map[string]string
	}{
		{
			// only the first two days for may
			from: "2024-05-01",
			to:   "2024-05-02",
			tracked: map[string]string{
				"bob":   "7h",
				"alice": "11h",
			},
			untracked: map[string]string{
				"bob":   "0h",
				"alice": "0h",
			},
		},
		{
			// only the second day of may (i.e. from/to are inclusive)
			from: "2024-05-02",
			to:   "2024-05-02",
			tracked: map[string]string{
				"bob":   "0h",
				"alice": "7h",
			},
			untracked: map[string]string{
				"bob":   "0h",
				"alice": "0h",
			},
		},
		{
			// Spans multiple rosters (may + june)
			// where neither bob nor alice have valid work-time contracts
			// after June 15th
			from: "2024-05-01",
			to:   "2024-06-30",
			tracked: map[string]string{
				"bob":   "14h",
				"alice": "29h",
			},
			untracked: map[string]string{
				"bob":   "0h",
				"alice": "7h",
			},
		},
	}

	for idx, testCase := range cases {
		title := fmt.Sprintf("#%d %s-%s", idx, testCase.from, testCase.to)

		t.Run(title, func(t *testing.T) {
			res, err := timecalc.CalculatePlannedMonthlyWorkTime(
				[]structs.DutyRoster{rosterMay, rosterJune},
				testCase.from,
				testCase.to,
				nil,
				map[string]timecalc.WorkTimeList{
					"bob":   workTimes,
					"alice": workTimes,
				},
			)

			require.NoError(t, err)

			for key, value := range testCase.tracked {
				d, err := time.ParseDuration(value)
				require.NoError(t, err)

				require.Equal(t, d, res.TotalForUser(key).Tracked)
			}

			for key, value := range testCase.untracked {
				d, err := time.ParseDuration(value)
				require.NoError(t, err)
				require.Equal(t, d, res.TotalForUser(key).Untracked)
			}
		})
	}

}
