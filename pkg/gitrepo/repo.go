package gitrepo

import (
	"context"
	"io"
	"log"

	"github.com/giantswarm/microerror"
	"github.com/go-errors/errors"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

type Config struct {
	AuthBasicToken string
	Dir            string
	URL            string
}

type Repo struct {
	url string

	auth    *http.BasicAuth
	storage *filesystem.Storage
}

func New(config Config) (*Repo, error) {
	if config.AuthBasicToken == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.AuthBasicToken must not be empty", config)
	}
	if config.Dir == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Dir must not be empty", config)
	}
	if config.URL == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.URL must not be empty", config)
	}

	fs := osfs.New(config.Dir)
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{ExclusiveAccess: true})

	r := &Repo{
		url: config.URL,

		auth: &http.BasicAuth{
			Username: "can-be-anything-but-not-empty",
			Password: config.AuthBasicToken,
		},
		storage: storage,
	}

	return r, nil
}

func (r *Repo) EnsureUpToDate(ctx context.Context) error {
	cloneOpts := &git.CloneOptions{
		Auth:       r.auth,
		URL:        r.url,
		NoCheckout: true,
	}

	repo, err := git.Clone(r.storage, nil, cloneOpts)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err = git.Open(r.storage, nil)
		if err != nil {
			return microerror.Mask(err)
		}
	} else if err != nil {
		return microerror.Mask(err)
	}

	fetchOpts := &git.FetchOptions{
		Auth:  r.auth,
		Force: true,
	}

	err = repo.Fetch(fetchOpts)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		// Fall trough.
	} else if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *Repo) ResolveVersion(ctx context.Context, ref string) (string, error) {
	repo, err := git.Open(r.storage, nil)
	if err != nil {
		return "", microerror.Mask(err)
	}

	tags := map[string]string{}
	{
		tagsIter, err := repo.Tags()
		if err != nil {
			log.Fatal(err)
		}
		defer tagsIter.Close()
		for {
			tag, err := tagsIter.Next()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				log.Fatal(err)
			}
			tags[tag.Hash().String()] = tag.Name().Short()
		}
	}

	var commit *object.Commit
	{
		hash, err := repo.ResolveRevision(plumbing.Revision(ref))
		if err != nil {
			log.Fatal(err)
		}

		commit, err = repo.CommitObject(*hash)
		if err != nil {
			log.Fatal(err)
		}
	}

	// When the commit is tagged return the tag.
	tag, ok := tags[commit.Hash.String()]
	if ok {
		return tag, nil
	}

	// Otherwise find the first tagged parent and return it's tag glued
	// with the SHA.
	var version string
	{
		var lastTag string

		c := commit
		for {
			tag, ok := tags[c.Hash.String()]
			if ok {
				lastTag = tag
				break
			}
			c, err = c.Parent(0)
			if errors.Is(err, object.ErrParentNotFound) {
				lastTag = "v0.0.0"
				break
			} else if err != nil {
				log.Fatal(err)
			}
		}

		version = lastTag + "-" + commit.Hash.String()
	}

	return version, nil
}
