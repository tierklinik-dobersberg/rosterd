package identity

import (
	"context"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/tierklinik-dobersberg/rosterd/structs"
)

type FileProvider struct {
	users []structs.User
}

func NewFileProvider(identityFile string) (*FileProvider, error) {
	var file struct {
		User []structs.User `hcl:"user,block"`
	}

	if err := hclsimple.DecodeFile(identityFile, nil, &file); err != nil {
		return nil, err
	}

	return &FileProvider{
		users: file.User,
	}, nil
}

func (fp *FileProvider) ListUsers(ctx context.Context, auth string) ([]structs.User, error) {
	return fp.users, nil
}

var _ Provider = new(FileProvider)
