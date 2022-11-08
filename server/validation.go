package server

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func validateNewWorkShift(shift structs.WorkShift) error {
	errs := new(multierror.Error)

	addError := func(msg string, args ...any) {
		errs.Errors = append(errs.Errors, fmt.Errorf(msg, args...))
	}

	if shift.ShortName == "" {
		addError("shortName is not defined")
	}

	if shift.Name == "" {
		addError("name is not defined")
	}

	if len(shift.Days) == 0 {
		addError("no days defined")
	}

	if shift.RequiredStaffCount == 0 {
		addError("no required staff count")
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
