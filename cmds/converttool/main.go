package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/spf13/cobra"
	"github.com/tierklinik-dobersberg/cis/pkg/models/roster/v1alpha"
	"github.com/tierklinik-dobersberg/rosterd/client"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func getRootCommand() *cobra.Command {
	var (
		cisdURL string
		cisdJWT string

		rosterdURL string
		rosterdJWT string

		analyze    bool
		dumpRoster bool
	)

	cmd := &cobra.Command{
		Use:  "converttool",
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if rosterdJWT == "" && cisdJWT != "" {
				rosterdJWT = cisdJWT
			}
			if rosterdJWT != "" && cisdJWT == "" {
				cisdJWT = rosterdJWT
			}

			year, err := strconv.ParseInt(args[0], 0, 0)
			if err != nil {
				hclog.L().Error("invalid year", "year", args[0], "error", err)
				os.Exit(1)
			}
			month, err := strconv.ParseInt(args[1], 0, 0)
			if err != nil {
				hclog.L().Error("invalid month", "month", args[1], "error", err)
				os.Exit(1)
			}

			res, err := loadCisdRoster(int(year), int(month), cisdURL, cisdJWT)
			if err != nil {
				hclog.L().Error("failed to load duty roster", "error", err)
				os.Exit(1)
			}

			cli := client.New(rosterdURL, rosterdJWT)

			roster := structs.Roster{
				Month: time.Month(month),
				Year:  int(year),
			}

			monthStart := time.Date(roster.Year, roster.Month, 1, 0, 0, 0, 0, time.Local)
			monthEnd := time.Date(roster.Year, roster.Month+1, 0, 0, 0, 0, 0, time.Local)

			workShifts, err := cli.GetRequiredShifts(cmd.Context(), monthStart, monthEnd, false)
			if err != nil {
				hclog.L().Error("failed to get working shifts", "error", err)
				os.Exit(1)
			}

			for dayKey, day := range res.Days {
				shiftsForToday := workShifts[fmt.Sprintf("%04d-%02d-%02d", year, month, dayKey)]

				for _, shift := range shiftsForToday {
					lunch := time.Date(shift.From.Year(), shift.From.Month(), shift.From.Day(), 12, 0, 0, 0, shift.From.Location())

					var staff []string
					switch {
					case shift.To.Sub(shift.From) == time.Hour*24: // emergency
						staff = day.OnCall.Night
					case shift.To.Equal(lunch):
						staff = day.Forenoon
					case shift.From.After(lunch):
						staff = day.Afternoon
					case (shift.IsHoliday || shift.IsWeekend) && shift.From.Hour() < 19:
						if len(day.OnCall.Day) > 0 {
							staff = day.OnCall.Day
						} else {
							staff = day.OnCall.Night
						}
					case (shift.IsHoliday || shift.IsWeekend) && shift.From.Hour() >= 19:
						staff = day.OnCall.Night

					default:
						hclog.L().Error("failed to find staff for shift %s (%s - %s)", shift.Definition.Name, shift.From, shift.To)
					}

					roster.Shifts = append(roster.Shifts, structs.RosterShift{
						Definition:         shift.Definition,
						Staff:              staff,
						ShiftID:            shift.ShiftID,
						IsHoliday:          shift.IsHoliday,
						IsWeekend:          shift.IsWeekend,
						From:               shift.From,
						To:                 shift.To,
						MinutesWorth:       shift.MinutesWorth,
						RequiredStaffCount: shift.RequiredStaffCount,
					})
				}
			}

			sort.Sort(ByTime(roster.Shifts))

			if dumpRoster {
				json.NewEncoder(os.Stdout).Encode(roster)
			}

			if analyze {
				analysisResult, err := cli.AnalyzeRoster(cmd.Context(), roster)
				if err != nil {
					hclog.L().Error("failed to analyze roster", "error", err)
					os.Exit(1)
				}

				json.NewEncoder(os.Stdout).Encode(analysisResult)
			}
		},
	}

	flags := cmd.Flags()
	{
		flags.StringVar(&rosterdJWT, "auth-rosterd", os.Getenv("ROSTERD_JWT"), "The JWT access token for rosterd")
		flags.StringVar(&cisdJWT, "auth-cisd", os.Getenv("CISD_JWT"), "The JWT access token for cisd")
		flags.StringVar(&cisdURL, "cisd", "http://localhost:4200", "Address of the CIS server")
		flags.StringVar(&rosterdURL, "rosterd", "http://localhost:8080", "Address of the Rosterd server")

		flags.BoolVar(&analyze, "analyze", false, "Analyze the new roster")
		flags.BoolVar(&dumpRoster, "dump", false, "Dump the new roster")
	}

	return cmd
}

func loadCisdRoster(year int, month int, url string, token string) (*v1alpha.DutyRoster, error) {
	// https://intern.tierklinikdobersberg.at/api/dutyroster/v1/roster/2022/10
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	req, err := retryablehttp.NewRequest("GET", fmt.Sprintf("%s%s/%d/%d", url, "api/dutyroster/v1/roster", year, month), nil)
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{
		Name:  "cis-session",
		Value: token,
	})

	res, err := retryablehttp.NewClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(res.Status)
	}

	var result v1alpha.DutyRoster
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func main() {
	if err := getRootCommand().Execute(); err != nil {
		hclog.L().Error(err.Error())
		os.Exit(1)
	}
}

type ByTime []structs.RosterShift

func (b ByTime) Len() int           { return len(b) }
func (b ByTime) Less(i, j int) bool { return b[i].From.Before(b[j].From) }
func (b ByTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
