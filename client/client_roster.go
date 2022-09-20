package client

import (
	"context"
	"fmt"
	"time"

	"github.com/tierklinik-dobersberg/rosterd/structs"
)

func (cli *Client) AnalyzeRoster(ctx context.Context, roster structs.Roster) (*structs.RosterAnalysis, error) {
	var result structs.RosterAnalysis

	if _, err := cli.doReq(ctx, "v1/roster/analyze", "POST", roster, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type RosterResult struct {
	Roster   structs.Roster          `json:"roster"`
	Analysis *structs.RosterAnalysis `json:"analysis"`
}

func (cli *Client) GenerateRoster(ctx context.Context, year int, month time.Month) (*RosterResult, error) {
	var result RosterResult

	if _, err := cli.doReq(ctx, fmt.Sprintf("v1/roster/generate/%04d/%d", year, month), "POST", nil, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
