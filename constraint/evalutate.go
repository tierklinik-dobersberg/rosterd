package constraint

import (
	"context"
	"fmt"
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/hashicorp/go-hclog"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func Evaluate(ctx context.Context, expr string, evalCtx EvalContext) (any, error) {
	return gval.EvaluateWithContext(ctx, expr, evalCtx)
}

type Cache struct {
	constraints map[string][]structs.Constraint
}

func EvaluateForStaff(ctx context.Context, includeSoft bool, log hclog.Logger, db database.ConstraintDatabase, staff string, roles []string, rosterShift structs.RosterShift, roster *structs.Roster, rosterOnly bool) ([]structs.ConstraintViolation, error) {
	return new(Cache).EvaluateForStaff(ctx, includeSoft, log, db, staff, roles, rosterShift, roster, rosterOnly)
}

func (cache *Cache) EvaluateForStaff(ctx context.Context, includeSoft bool, log hclog.Logger, db database.ConstraintDatabase, staff string, roles []string, rosterShift structs.RosterShift, roster *structs.Roster, rosterOnly bool) ([]structs.ConstraintViolation, error) {
	key := fmt.Sprintf("%s-%s", staff, strings.Join(roles, ","))

	constraints, ok := cache.constraints[key]
	if !ok {
		var err error

		log.Info("loading constraints for user", "user", staff, "roles", roles)
		constraints, err = db.FindConstraints(ctx, []string{staff}, roles)
		if err != nil {
			return nil, err
		}

		if cache.constraints == nil {
			cache.constraints = make(map[string][]structs.Constraint)
		}

		cache.constraints[key] = constraints
	}

	var violations []structs.ConstraintViolation

	for _, c := range constraints {
		if !includeSoft && !c.Hard {
			continue
		}

		// skip rosters that should only be evaluted once against the whole roster.
		if c.RosterOnly && !rosterOnly {
			continue
		}
		if !c.RosterOnly && rosterOnly {
			continue
		}

		evalCtx := EvalContext{
			Roster:      roster,
			RosterShift: rosterShift,
			Staff:       staff,
			Day:         rosterShift.From.Weekday().String(),
		}
		evalCtx.init()

		res, err := Evaluate(ctx, c.Expression, evalCtx)
		if err != nil {
			return nil, err
		}

		b, ok := res.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value returned from constraint expression: %T", res)
		}

		if c.Deny == b {
			violations = append(violations, structs.ConstraintViolation{
				ID:      c.ID,
				Name:    c.Description,
				Type:    "constraint",
				Panalty: c.Penalty,
				Hard:    c.Hard,
			})
		}
	}

	return violations, nil
}
