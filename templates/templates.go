package templates

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/Masterminds/sprig"
	calendarv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/calendar/v1"
)

//go:generate npm run build
//go:embed dist
var dist embed.FS

type RosterUser struct {
	Name          string
	Color         string
	ContrastColor string
}

type RosterShift struct {
	ShiftName string
	Users     []RosterUser
	Color     string
	Order     int
}

type RosterShiftList []RosterShift

func (rsl RosterShiftList) Len() int           { return len(rsl) }
func (rsl RosterShiftList) Less(i, j int) bool { return rsl[i].Order < rsl[j].Order }
func (rsl RosterShiftList) Swap(i, j int)      { rsl[i], rsl[j] = rsl[j], rsl[i] }

type RosterDay struct {
	DayTitle string
	Shifts   []RosterShift
	Holiday  *calendarv1.PublicHoliday
	Disabled bool
}

type RosterWeek struct {
	Days []RosterDay
}

type RosterContext struct {
	Days  []RosterDay
	Weeks []RosterWeek
}

var temp *template.Template

func init() {
	var err error
	temp, err = template.New("").Funcs(sprig.HtmlFuncMap()).ParseFS(dist, "dist/**.html")
	if err != nil {
		panic("Failed to parse HTML templates: " + err.Error())
	}
}

func RenderRosterTemplate(ctx context.Context, renderContext RosterContext) (io.Reader, error) {
	// render
	buf := new(bytes.Buffer)
	if err := temp.ExecuteTemplate(buf, "roster-table", renderContext); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return buf, nil
}
