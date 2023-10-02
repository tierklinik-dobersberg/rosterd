package cmds

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
)

func getUserMap(root *cli.Root) map[string]*idmv1.Profile {
	res, err := root.Users().ListUsers(context.Background(), connect.NewRequest(&idmv1.ListUsersRequest{}))

	if err != nil {
		logrus.Fatalf("failed to fetch users: %s", err)
	}

	m := make(map[string]*idmv1.Profile)

	for _, u := range res.Msg.Users {
		m[u.User.Id] = u
	}

	return m
}
