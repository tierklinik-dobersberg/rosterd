package client

import (
	"context"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) SetWorkTime(ctx context.Context, wt structs.WorkTime) error {
	res, err := cli.doReq(ctx, "v1/worktime/", "POST", wt, nil, nil)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

func (cli *Client) GetWorkTimeHistory(ctx context.Context, staff string) ([]structs.WorkTime, error) {
	var result struct {
		History []structs.WorkTime `json:"history"`
	}

	if _, err := cli.doReq(ctx, "v1/worktime/"+staff+"/history", "GET", nil, nil, &result); err != nil {
		return nil, err
	}

	return result.History, nil
}

func (cli *Client) GetCurrentWorkTimes(ctx context.Context) (map[string]structs.WorkTime, error) {
	var result struct {
		WorkTimes map[string]structs.WorkTime `json:"workTimes"`
	}

	if _, err := cli.doReq(ctx, "v1/worktime/", "GET", nil, nil, &result); err != nil {
		return nil, err
	}

	return result.WorkTimes, nil
}
