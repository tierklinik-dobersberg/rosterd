package server

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func validateNewWorkShift(shift structs.WorkShift) error {
	errs := new(multierror.Error)

	addError := func(msg string, args ...any) {
		errs.Errors = append(errs.Errors, fmt.Errorf(msg, args...))
	}

	if len(shift.Days) == 0 {
		addError("no days defined")
	}

	if shift.RequiredStaffCount == 0 {
		addError("no required staff count")
	}

	return errs.ErrorOrNil()
}

func validateNewOffTimeRequest(req structs.OffTimeRequest) error {
	errs := new(multierror.Error)

	addError := func(msg string, args ...any) {
		errs.Errors = append(errs.Errors, fmt.Errorf(msg, args...))
	}

	if req.Approved != nil {
		// FIXME(ppacher): for auditing purposes an admin should be allowed to do
		// that
		addError("requests must be approved in separate process")
	}

	if req.From.IsZero() {
		addError("missing from time")
	}

	if req.To.IsZero() {
		addError("missing to time")
	}

	if req.To.Before(req.From) {
		addError("invalid to/from values")
	}

	now := time.Now()
	if now.After(req.To) || now.After(req.From) {
		// FIXME(ppacher): for auditing purposes an admin should be allowed to do
		// that
		addError("not allowed to create off-time requests in the past")
	}

	if req.StaffID == "" {
		addError("missing staff identifier")
	}

	return errs.ErrorOrNil()
}

func unwrapErrors(err error) any {
	if merr, ok := err.(*multierror.Error); ok {
		var str []string
		for _, e := range merr.Errors {
			str = append(str, e.Error())
		}

		return str
	}

	return err.Error()
}
