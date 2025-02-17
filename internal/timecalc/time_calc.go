package timecalc

import (
	"context"
	"fmt"
	stdlog "log"
	"time"

	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/rosterd/internal/structs"
	"golang.org/x/exp/maps"
)

type MonthlyWorkDays struct {
	Year     int
	Month    time.Month
	WorkDays []int
}

func (mwd MonthlyWorkDays) String() string {
	return fmt.Sprintf("WorkDays{for=%02d/%04d days=%d}", mwd.Month, mwd.Year, len(mwd.WorkDays))
}

func GatherWorkDaysByMonth(holidays map[string]*calendarv1.PublicHoliday, from, to string) ([]MonthlyWorkDays, error) {
	var (
		result  []MonthlyWorkDays
		current *MonthlyWorkDays
	)

	fromTime, err := time.ParseInLocation("2006-01-2", from, time.Local)
	if err != nil {
		return nil, fmt.Errorf("from: invalid date: %w", err)
	}

	toTime, err := time.ParseInLocation("2006-01-2", to, time.Local)
	if err != nil {
		return nil, fmt.Errorf("to: invalid date: %w", err)
	}

	// range over all days
	for iter := fromTime; iter.Before(toTime) || iter.Equal(toTime); iter = iter.AddDate(0, 0, 1) {
		year, month, date := iter.Date()

		// create a new "current" container for the monthly work days.
		if current == nil {
			current = &MonthlyWorkDays{
				Year:  year,
				Month: month,
			}
		} else if current.Year != year || current.Month != month {
			// if we moved over to the next month, append the previous to the
			// result set and start a fresh container.
			result = append(result, *current)
			current = &MonthlyWorkDays{
				Year:  year,
				Month: month,
			}
		}

		// check if iter is a public holiday and continue to the next if it is
		if hd, ok := holidays[iter.Format("2006-01-02")]; ok && hd.Type == calendarv1.HolidayType_PUBLIC {
			continue
		}

		// check the week-day to see if iter is a regular working day
		switch iter.Weekday() {
		case time.Saturday, time.Sunday:
			// Weekend days do not count as regular working days.
			continue

		default:
			current.WorkDays = append(current.WorkDays, date)
		}
	}

	if current != nil {
		result = append(result, *current)
	}

	return result, nil
}

type ExpectedMonthlyWorkTime struct {
	Year              int
	Month             time.Month
	TrackedWorkTime   time.Duration
	UntrackedWorkTime time.Duration
}

func (mwt ExpectedMonthlyWorkTime) String() string {
	return fmt.Sprintf("WorkTime{for=%02d/%04d duration=%s}", mwt.Year, mwt.Month, mwt.TrackedWorkTime)
}

type ExpectedMonthlyWorkTimeList []ExpectedMonthlyWorkTime

func (emwtl ExpectedMonthlyWorkTimeList) TotalTrackedWorkTime() time.Duration {
	var workTime time.Duration

	for _, e := range emwtl {
		workTime += e.TrackedWorkTime
	}

	return workTime
}

func (emwtl ExpectedMonthlyWorkTimeList) TotalWorkTime() time.Duration {
	var workTime time.Duration

	for _, e := range emwtl {
		workTime += e.TrackedWorkTime + e.UntrackedWorkTime
	}

	return workTime
}

func CalculateExpectedWorkTime(
	ctx context.Context,
	monthlyWorkDays []MonthlyWorkDays,
	workTimes map[string]WorkTimeList,
	from string,
	to string,
) (map[string]ExpectedMonthlyWorkTimeList, error) {

	var (
		fromTime time.Time
		toTime   time.Time
	)

	if from != "" {
		var err error
		fromTime, err = time.ParseInLocation("2006-01-02", from, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid from time: %w", err)
		}
	}

	if to != "" {
		var err error
		toTime, err = time.ParseInLocation("2006-01-02", to, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid to time: %w", err)
		}
	}

	result := make(map[string]ExpectedMonthlyWorkTimeList)
	for userId := range workTimes {
		result[userId] = make([]ExpectedMonthlyWorkTime, len(monthlyWorkDays))
	}

	for idx, mwd := range monthlyWorkDays {

		for userId := range workTimes {
			result[userId][idx] = ExpectedMonthlyWorkTime{
				Year:  mwd.Year,
				Month: mwd.Month,
			}
		}

		for _, date := range mwd.WorkDays {
			dateTime := time.Date(mwd.Year, mwd.Month, date, 0, 0, 0, 0, time.Local)

			if !fromTime.IsZero() && dateTime.Before(fromTime) {
				continue
			}

			if !toTime.IsZero() && dateTime.After(toTime) {
				continue
			}

			for userId := range workTimes {
				wt, ok := workTimes[userId].FindForDate(dateTime)
				if !ok {
					log.L(ctx).Warnf("User %q does not have a working-time set for %s", userId, dateTime.Local().Format("2006-01-02"))
					// no worktime for this date.
					continue
				}

				// Update the WorkTime for this month
				timePerWorkDay := float64(wt.TimePerWeek) / 5.0

				log.L(ctx).Warnf("User %q with work-time %s works %s on %s with time-tracking=%v", userId, wt.TimePerWeek, time.Duration(timePerWorkDay), dateTime.Local().Format("2006-01-2"), !wt.ExcludeFromTimeTracking)

				if wt.ExcludeFromTimeTracking {
					result[userId][idx].UntrackedWorkTime += time.Duration(timePerWorkDay)
				} else {
					result[userId][idx].TrackedWorkTime += time.Duration(timePerWorkDay)
				}
			}
		}
	}

	return result, nil
}

type WorkTimeList []structs.WorkTime

func (wtl WorkTimeList) FindForDate(t time.Time) (structs.WorkTime, bool) {
	key := t.Local().Format("2006-01-02")

	for idx, wt := range wtl {
		// if wt is not even applicable yet, skip it
		if wt.ApplicableFrom.After(t) {
			continue
		}

		// EndsWith is inclusive, so we check against the date after EndsWith.
		if !wt.EndsWith.IsZero() && wt.EndsWith.AddDate(0, 0, 1).Before(t) {
			continue
		}

		if idx < len(wtl)-1 {
			next := wtl[idx+1]

			if next.ApplicableFrom.Before(wt.ApplicableFrom) {
				panic("expected WorkTimeList to be sorted!")
			}

			// the next entry is effective already.
			if next.ApplicableFrom.Before(t) || next.ApplicableFrom.Equal(t) {
				continue
			}
		}

		stdlog.Printf("%s (%s): applicable: applicableFrom=%s with-timetracking=%v", key, wt.UserID, wt.ApplicableFrom, !wt.ExcludeFromTimeTracking)

		return wt, true
	}

	return structs.WorkTime{}, false
}

type UserTime struct {
	Tracked   time.Duration
	Untracked time.Duration
}

func (ut UserTime) Total() time.Duration { return ut.Tracked + ut.Untracked }
func (ut UserTime) HasUntracked() bool   { return ut.Untracked > 0 }
func (ut UserTime) HasTracked() bool     { return ut.Tracked > 0 }

type PlannedMonthlyWorkTime struct {
	Year  int
	Month time.Month

	PerUser map[string]*UserTime
}

type PlannedMonthlyWorkTimeList []*PlannedMonthlyWorkTime

func (lst PlannedMonthlyWorkTimeList) TotalForUser(userId string) UserTime {
	result := UserTime{}

	for _, e := range lst {
		v, ok := e.PerUser[userId]
		if !ok {
			continue
		}

		result.Tracked += v.Tracked
		result.Untracked += v.Untracked
	}

	return result
}

func CalculatePlannedMonthlyWorkTime(
	ctx context.Context,
	rosters []structs.DutyRoster,
	from string,
	to string, // inclusive
	workShifts []structs.WorkShift,
	workTimes map[string]WorkTimeList,
) (PlannedMonthlyWorkTimeList, error) {

	result := make(map[string] /*YYYY-MM*/ *PlannedMonthlyWorkTime)

	fromTime, err := time.ParseInLocation("2006-01-02", from, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid from value: %w", err)
	}

	toTime, err := time.ParseInLocation("2006-01-02", to, time.Local)
	if err != nil {
		return nil, fmt.Errorf("invalid to value: %w", err)
	}
	toTime = toTime.AddDate(0, 0, 1)

	// wsMap := data.IndexSlice(workShifts, func(ws structs.WorkShift) string {
	// 	return ws.ID.Hex()
	// })

	for _, roster := range rosters {
		// immediately skip rosters that don't match from or to
		if roster.FromTime().After(toTime) || roster.ToTime().Before(fromTime) {
			log.L(ctx).Infof("skipping roster  %s - %s", roster.From, roster.To)

			continue
		}

		for _, shift := range roster.Shifts {
			// skip this shift if it is out-of-range
			if shift.To.Before(fromTime) || shift.From.After(toTime) {
				log.L(ctx).Infof("skipping shift %s on %s", shift.WorkShiftID, shift.From.Format("2006-01-02"))
				continue
			}

			// determine how much time this shift is worth for time-tracking
			// timeWorth := shift.To.Sub(shift.From)
			// if def, ok := wsMap[shift.WorkShiftID.Hex()]; ok && def.MinutesWorth != nil && *def.MinutesWorth > 0 {
			// 	timeWorth = time.Duration(*def.MinutesWorth) * time.Minute
			// }

			timeWorth := shift.TimeWorth

			log.L(ctx).Infof("shift %s on %s is %s time worth", shift.WorkShiftID, shift.From.Format("2006-01-02"), timeWorth)

			// Perpare the date key and make sure we have PlannedMonthlyWorkTime container
			// for the result.
			key := shift.From.Format("2006-01")
			if result[key] == nil {
				result[key] = &PlannedMonthlyWorkTime{
					Year:    shift.From.Year(),
					Month:   shift.From.Month(),
					PerUser: make(map[string]*UserTime),
				}
			}

			// Update the tracked and untracked work-times for each user.
			for _, userId := range shift.AssignedUserIds {
				if result[key].PerUser[userId] == nil {
					result[key].PerUser[userId] = new(UserTime)
				}

				wt, ok := workTimes[userId].FindForDate(shift.From)

				// If we don't have a worktime-definition for this user or
				// time-tracking is disabled, add the time to the .Untracked field
				if !ok || wt.ExcludeFromTimeTracking || (!wt.EndsWith.IsZero() && wt.EndsWith.AddDate(0, 0, 1).Before(shift.From)) { // FIXME(ppacher): check this again!!!
					result[key].PerUser[userId].Untracked += timeWorth
				} else {
					// Otherwise, there's a work-time definition and the user
					// has time-tracking enabled for this work-time so we add
					// the time-value to the .Tracked field
					result[key].PerUser[userId].Tracked += timeWorth
				}
			}
		}
	}

	return maps.Values(result), nil
}
