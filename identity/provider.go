package identity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type (
	Provider interface {
		// ListUsers returns a list of all users.
		ListUsers(ctx context.Context, auth string) ([]structs.User, error)
	}

	HTTPProvider struct {
		BaseURL string
		Client  *retryablehttp.Client
	}
)

const (
	listUsersEndpoint = "v1/users"
	profileEndpoint   = "v1/profile"
)

func (cli *HTTPProvider) endpoint(e string) string {
	base := cli.BaseURL
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	return base + e
}

func (cli *HTTPProvider) doRequest(ctx context.Context, method string, endpoint string, auth string, body io.Reader) (*http.Response, error) {
	url := cli.endpoint(endpoint)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+auth)

	httpCli := cli.Client
	if httpCli == nil {
		httpCli = retryablehttp.NewClient()
	}

	return httpCli.Do(req)
}

func (cli *HTTPProvider) ListUsers(ctx context.Context, auth string) ([]structs.User, error) {
	res, err := cli.doRequest(ctx, http.MethodGet, listUsersEndpoint, auth, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var users []structs.User
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&users); err != nil {
		return nil, err
	}

	return nil, nil
}

// Interface check
var _ Provider = new(HTTPProvider)
