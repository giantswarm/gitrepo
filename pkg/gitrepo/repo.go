package gitrepo

import (
	"context"

	"gopkg.in/src-d/go-billy.v4"
)

type Config struct {
	AuthBasicToken string
	Dir            string
	URL            string
}

type Repo struct {
	authBasicToken string
	url            string

	fs billy.Filesystem
}

func New(cnfig Config) (*Repo, error) {
	r := &Repo{}

	return r, nil
}

func (r *Repo) EnsureUpToDate(ctx context.Context) error {
}

func (r *Repo) ResolveVersion(ctx context.Context, ref string) (string, error) {
}
