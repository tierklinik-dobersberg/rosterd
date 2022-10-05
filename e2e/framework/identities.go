package framework

import (
	"context"
	"sync"
	"time"

	"github.com/tierklinik-dobersberg/cis/pkg/jwt"
	"github.com/tierklinik-dobersberg/rosterd/client"
	"github.com/tierklinik-dobersberg/rosterd/identity"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type MockIdentityProvider struct {
	secret []byte

	rw    sync.Mutex
	users []structs.User
}

func (mip *MockIdentityProvider) ListUsers(_ context.Context, _ string) ([]structs.User, error) {
	mip.rw.Lock()
	defer mip.rw.Unlock()

	res := make([]structs.User, len(mip.users))
	copy(res, mip.users)

	return res, nil
}

func (mip *MockIdentityProvider) GetClient(username string, roles ...string) *client.Client {
	token, _ := jwt.SignToken("HS256", mip.secret, jwt.Claims{
		Subject:   username,
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
		AppMetadata: &jwt.AppMetadata{
			Authorization: &jwt.Authorization{
				Roles: roles,
			},
		},
	})

	return client.New("http://localhost:12345", token)
}

func NewMockIdentityProvider(secret string) *MockIdentityProvider {
	return &MockIdentityProvider{
		secret: []byte(secret),
	}
}

var _ identity.Provider = new(MockIdentityProvider)
