package client

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) CreateOffTimeRequest(ctx context.Context, req structs.CreateOffTimeRequest) error {
	res, err := cli.doReq(ctx, "v1/offtime/request/", "POST", req, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) AddOffTimeCredit(ctx context.Context, staff string, credit float64, from time.Time, comment string) error {
	res, err := cli.doReq(ctx, "v1/offtime/credit/"+staff, "POST", structs.CreateOffTimeCreditsRequest{
		StaffID:     staff,
		From:        from,
		Description: comment,
		Days:        credit,
	}, nil, nil)

	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) DeleteOffTimeRequest(ctx context.Context, id string) error {
	res, err := cli.doReq(ctx, "v1/offtime/request/"+id, "DELETE", nil, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) ApproveOffTimeRequest(ctx context.Context, id string, approved bool, comment string) error {
	url := "v1/offtime/request/" + id + "/"
	if approved {
		url += "approve"
	} else {
		url += "reject"
	}

	res, err := cli.doReq(ctx, url, "POST", map[string]any{"comment": comment}, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string) ([]structs.OffTimeEntry, error) {
	query := url.Values{}

	if !from.IsZero() {
		query.Add("from", from.Format("2006-01-02"))
	}

	if !to.IsZero() {
		query.Add("to", to.Format("2006-01-02"))
	}

	if approved != nil {
		query.Add("approved", strconv.FormatBool(*approved))
	}

	for _, s := range staff {
		query.Add("staff", s)
	}

	var result struct {
		Requests []structs.OffTimeEntry `json:"offTimeRequests"`
	}

	res, err := cli.doReq(ctx, "v1/offtime/", "GET", nil, query, &result)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	return result.Requests, nil
}
