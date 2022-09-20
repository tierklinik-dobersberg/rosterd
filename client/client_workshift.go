package client

import (
	"context"
	"net/url"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) ListWorkShifts(ctx context.Context) ([]structs.WorkShift, error) {
	var result struct {
		Workshifts []structs.WorkShift `json:"workShifts"`
	}

	_, err := cli.doReq(ctx, "v1/workshift", "GET", nil, nil, &result)
	if err != nil {
		return nil, err
	}

	return result.Workshifts, nil
}

func (cli *Client) CreateWorkShift(ctx context.Context, shift structs.WorkShift) error {
	res, err := cli.doReq(ctx, "v1/workshift", "POST", shift, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) UpdateWorkShift(ctx context.Context, shift structs.WorkShift) error {
	res, err := cli.doReq(ctx, "v1/workshift/"+shift.ID.Hex(), "PUT", shift, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) DeleteWorkShift(ctx context.Context, id string) error {
	res, err := cli.doReq(ctx, "v1/workshift/"+id, "DELETE", nil, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) GetRequiredShifts(ctx context.Context, from time.Time, to time.Time, evalConstraints bool) (map[string][]structs.RosterShiftWithStaffList, error) {
	query := url.Values{
		"from": []string{from.Format("2006-01-02")},
		"to":   []string{to.Format("2006-01-02")},
	}

	if evalConstraints {
		query.Add("stafflist", "y")
	}

	var result map[string][]structs.RosterShiftWithStaffList
	res, err := cli.doReq(ctx, "v1/roster/shifts", "GET", nil, query, &result)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	return result, nil
}
