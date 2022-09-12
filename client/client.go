package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

type Client struct {
	client  *retryablehttp.Client
	baseUrl string
	token   string
}

func New(baseUrl string, token string) *Client {
	return &Client{
		client:  retryablehttp.NewClient(),
		baseUrl: baseUrl,
		token:   token,
	}
}

func (cli *Client) ep(endpoint string) string {
	base := cli.baseUrl
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + endpoint
}

func (cli *Client) doReq(ctx context.Context, endpoint string, method string, body any, query url.Values, result any) (*http.Response, error) {
	var bodyBlob []byte
	if body != nil {
		var err error
		bodyBlob, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	u := cli.ep(endpoint)
	if query != nil {
		u += "?" + query.Encode()
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, u, bodyBlob)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cli.token)

	res, err := cli.client.Do(req)
	if err != nil {
		return res, err
	}

	if res.StatusCode >= 300 {
		var x interface{}
		if res.ContentLength > 0 {
			decoder := json.NewDecoder(res.Body)
			_ = decoder.Decode(&x)

			res.Body.Close()
		}

		return nil, fmt.Errorf("%d: %s (%v)", res.StatusCode, res.Status, x)
	}

	if result != nil {
		defer res.Body.Close()

		decoder := json.NewDecoder(res.Body)
		if err := decoder.Decode(result); err != nil {
			return res, err
		}
	}

	return res, nil
}
