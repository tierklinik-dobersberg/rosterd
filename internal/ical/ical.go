package ical

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	ics "github.com/arran4/golang-ical"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
)

type Event struct {
	From  time.Time
	To    time.Time
	Name  string
	Users []*idmv1.Profile
}

func (e Event) id() string {
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("%s-%s-%s-%s", e.Name, e.From, e.To, time.Now())))

	return hex.EncodeToString(h.Sum(nil))
}

type Calendar struct {
	Events []Event
}

func (c Calendar) ToICS(rosterFrom time.Time) string {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodAdd)
	cal.SetProductId("-//dobersberg.vet//Tierklinik Dobersberg 2023c//EN")
	cal.SetName("Dienstplan " + rosterFrom.Format("01/2006"))
	cal.SetTzid("Europe/Vienna")

	seq := 1
	dtTime := time.Now()
	for _, e := range c.Events {
		evt := cal.AddEvent(e.id())
		evt.SetStartAt(e.From)
		evt.SetEndAt(e.To)
		evt.SetSummary(e.Name)
		evt.SetDtStampTime(dtTime)
		evt.SetOrganizer("office@tierklinikdobersberg.at", ics.WithCN("Tierklinik Dobersberg"))

		for _, user := range e.Users {
			userDisplayName := user.User.DisplayName
			if userDisplayName == "" {
				userDisplayName = user.User.Username
			}
			userPrimaryMail := ""
			if user.User.PrimaryMail != nil {
				userPrimaryMail = user.User.PrimaryMail.Address
			}

			evt.AddAttendee(userPrimaryMail, ics.WithCN(userDisplayName), ics.ParticipationRoleReqParticipant, ics.ParticipationStatusAccepted)
		}

		evt.SetSequence(seq)
	}

	blob := cal.Serialize()

	return blob
}
