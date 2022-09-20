package client

import (
	"context"
	"net/url"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) CreateConstraint(ctx context.Context, body structs.Constraint) error {
	res, err := cli.doReq(ctx, "v1/constraint/", "POST", body, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) DeleteConstraint(ctx context.Context, id string) error {
	res, err := cli.doReq(ctx, "v1/constraint/"+id, "DELETE", nil, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) FindConstraints(ctx context.Context, staff []string, role []string) ([]structs.Constraint, error) {
	query := url.Values{}

	if len(staff) > 0 {
		query["staff"] = staff
	}

	if len(role) > 0 {
		query["role"] = role
	}

	var result struct {
		Constraints []structs.Constraint `json:"constraints"`
	}
	_, err := cli.doReq(ctx, "v1/constraint/", "GET", nil, query, &result)
	if err != nil {
		return nil, err
	}

	return result.Constraints, nil
}
