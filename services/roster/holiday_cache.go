package roster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bufbuild/connect-go"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1/calendarv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"golang.org/x/sync/singleflight"
)

type HolidayCache struct {
	getter calendarv1connect.HolidayServiceClient

	singleflight.Group

	rw    sync.RWMutex
	cache map[string]map[string]*calendarv1.PublicHoliday
}

func NewHolidayCache(getter calendarv1connect.HolidayServiceClient) *HolidayCache {
	cache := &HolidayCache{
		getter: getter,
		cache:  make(map[string]map[string]*calendarv1.PublicHoliday),
	}

	return cache
}

func (cache *HolidayCache) For(ctx context.Context, from time.Time, to time.Time) (map[string]*calendarv1.PublicHoliday, error) {
	// Get a list of months to fetch holidays for
	holidaysToFetch := make([]time.Time, 0, 1)
	for iter := from; iter.Year() != to.Year() && iter.Month() != to.Month(); iter = iter.AddDate(0, 1, 0) {
		holidaysToFetch = append(holidaysToFetch, iter)
	}

	var result = make(map[string]*calendarv1.PublicHoliday)

	for _, t := range holidaysToFetch {
		key := t.Format("2006-01")
		date := t

		perMonthResult, err, _ := cache.Group.Do(key, func() (interface{}, error) {
			cache.rw.RLock()
			if result, ok := cache.cache[key]; ok {
				defer cache.rw.RUnlock()

				log.L(ctx).Infof("holiday cache hit for %s", key)

				return result, nil
			}

			cache.rw.RUnlock()

			log.L(ctx).Infof("holiday cache miss for %s, fetching ...", key)

			cache.rw.Lock()
			defer cache.rw.Unlock()

			res, err := cache.getter.GetHoliday(ctx, connect.NewRequest(&calendarv1.GetHolidayRequest{
				Year:  uint64(date.Year()),
				Month: uint64(date.Month()),
			}))
			if err != nil {
				return nil, fmt.Errorf("failed to fetch holidays for %s", t.Format("2006-01-02"))
			}

			holidayMap := data.IndexSlice(res.Msg.Holidays, func(ph *calendarv1.PublicHoliday) string {
				return ph.Date
			})

			cache.cache[key] = holidayMap

			return holidayMap, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch holidays for %s: %w", key, err)
		}

		for key, value := range perMonthResult.(map[string]*calendarv1.PublicHoliday) {
			result[key] = value
		}
	}

	return result, nil
}
