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
	listUsersEndpoint = "identity/v1/users"
	profileEndpoint   = "identity/v1/profile"
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

	req.AddCookie(&http.Cookie{
		Name:  "cis-session",
		Value: auth,
	})

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

	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var users []structs.User
	if err := json.Unmarshal(blob, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// Interface check
var _ Provider = new(HTTPProvider)
