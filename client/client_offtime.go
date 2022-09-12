package client

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) CreateOffTimeRequest(ctx context.Context, req structs.OffTimeRequest) error {
	res, err := cli.doReq(ctx, "v1/offtime/", "POST", req, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) DeleteOffTimeRequest(ctx context.Context, id string) error {
	res, err := cli.doReq(ctx, "v1/offtime/"+id, "DELETE", nil, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) ApproveOffTimeRequest(ctx context.Context, id string, approved bool) error {
	url := "v1/offtime/" + id + "/"
	if approved {
		url += "approve"
	} else {
		url += "reject"
	}

	res, err := cli.doReq(ctx, url, "POST", nil, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) FindOffTimeRequests(ctx context.Context, from, to time.Time, approved *bool, staff []string) ([]structs.OffTimeRequest, error) {
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
		Requests []structs.OffTimeRequest `json:"offTimeRequests"`
	}

	res, err := cli.doReq(ctx, "v1/offtime/", "GET", nil, query, &result)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	return result.Requests, nil
}
