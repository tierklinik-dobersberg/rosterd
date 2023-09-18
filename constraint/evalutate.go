package constraint

import (
	"context"
	"fmt"
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/sirupsen/logrus"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	rosterv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/roster/v1"
	"github.com/tierklinik-dobersberg/rosterd/database"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func Evaluate(ctx context.Context, expr string, evalCtx EvalContext) (any, error) {
	return gval.EvaluateWithContext(ctx, expr, evalCtx)
}

type Cache struct {
	constraints map[string][]structs.Constraint
}

func EvaluateForStaff(ctx context.Context, includeSoft bool, log *logrus.Entry, db database.ConstraintDatabase, staff string, roles []*idmv1.Role, rosterShift structs.RosterShift, roster *structs.Roster, rosterOnly bool) ([]structs.ConstraintViolation, error) {
	return new(Cache).EvaluateForStaff(ctx, includeSoft, log, db, staff, roles, rosterShift, roster, rosterOnly)
}

func (cache *Cache) EvaluateForStaff(ctx context.Context, includeSoft bool, log *logrus.Entry, db database.ConstraintDatabase, staff string, roles []*idmv1.Role, rosterShift structs.RosterShift, roster *structs.Roster, rosterOnly bool) ([]structs.ConstraintViolation, error) {
	roleIds := make([]string, len(roles))
	for idx, role := range roles {
		roleIds[idx] = role.Id
	}

	key := fmt.Sprintf("%s-%s", staff, strings.Join(roleIds, ","))

	constraints, ok := cache.constraints[key]
	if !ok {
		var err error

		log.WithFields(logrus.Fields{
			"user":  staff,
			"roles": roles,
		}).Infof("loading constraints for user")

		constraints, err = db.FindConstraints(ctx, []string{staff}, roleIds)
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

func EvaluateForStaff2(ctx context.Context, cache *Cache, includeSoft bool, log *logrus.Entry, db database.ConstraintDatabase, staff string, roles []*idmv1.Role, rosterShift structs.RosterShift, roster *structs.Roster, rosterOnly bool) ([]*rosterv1.ConstraintViolation, error) {
	result, err := cache.EvaluateForStaff(ctx, includeSoft, log, db, staff, roles, rosterShift, roster, rosterOnly)
	if err != nil {
		return nil, err
	}

	protoResult := make([]*rosterv1.ConstraintViolation, len(result))
	for idx, v := range result {
		protoResult[idx] = &rosterv1.ConstraintViolation{
			Hard: v.Hard,
			Kind: &rosterv1.ConstraintViolation_Evaluation{
				Evaluation: &rosterv1.ConstraintEvaluationViolation{
					Id:          v.ID.Hex(),
					Description: v.Name,
				},
			},
		}
	}

	return protoResult, nil
}
