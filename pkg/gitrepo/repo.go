package gitrepo

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/go-errors/errors"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

var tagRegex = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+`)

type Config struct {
	AuthBasicToken string
	Dir            string
	URL            string
}

type Repo struct {
	url string

	auth    transport.AuthMethod
	storage *filesystem.Storage
}

func New(config Config) (*Repo, error) {
	if config.Dir == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Dir must not be empty", config)
	}

	var auth transport.AuthMethod
	{
		if config.AuthBasicToken != "" {
			auth = &http.BasicAuth{
				Username: "can-be-anything-but-not-empty",
				Password: config.AuthBasicToken,
			}
		}
	}

	fs := osfs.New(config.Dir)
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{ExclusiveAccess: true})

	// When URL is not configured assume the repository is cloned on disk
	// and take the URL or origin remote.
	if config.URL == "" {
		repo, err := git.Open(storage, nil)
		if err != nil {
			return nil, microerror.Maskf(invalidConfigError, "%T.URL not set and failed to open repository with error %#q", config, err)
		}

		remoteName := "origin"

		remote, err := repo.Remote(remoteName)
		if err != nil {
			return nil, microerror.Maskf(invalidConfigError, "%T.URL not set and failed to find remote with name %#q with error %#q", config, remoteName, err)
		}

		// According to
		// https://godoc.org/gopkg.in/src-d/go-git.v4/config#RemoteConfig:
		//
		//	URLs the URLs of a remote repository. It must be
		//	non-empty. Fetch will always use the first URL, while
		//	push will use all of them.
		//
		config.URL = remote.Config().URLs[0]
	}

	r := &Repo{
		url: config.URL,

		auth:    auth,
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
		// Fall through.
	} else if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *Repo) HeadBranch(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, nil)
	if err != nil {
		return "", microerror.Mask(err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", microerror.Mask(err)
	}

	return head.Name().Short(), nil
}

func (r *Repo) HeadSHA(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, nil)
	if err != nil {
		return "", microerror.Mask(err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", microerror.Mask(err)
	}

	return head.Hash().String(), nil
}

// HeadTag returns the tag for the HEAD ref.
//
// It returns error handled by IsNotFound if the HEAD ref is not tagged.
func (r *Repo) HeadTag(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, nil)
	if err != nil {
		return "", microerror.Mask(err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", microerror.Mask(err)
	}

	tagsBySHA, err := r.tags(repo)
	if err != nil {
		return "", microerror.Mask(err)
	}

	tags := tagsBySHA[head.Hash().String()]

	if len(tags) == 0 {
		return "", microerror.Maskf(notFoundError, "HEAD ref is not tagged")
	}
	if len(tags) > 1 {
		return "", microerror.Maskf(executionFailedError, "HEAD ref has multiple tags %v", tags)
	}

	return tags[0], nil
}

func (r *Repo) ResolveVersion(ctx context.Context, ref string) (string, error) {
	repo, err := git.Open(r.storage, nil)
	if err != nil {
		return "", microerror.Mask(err)
	}

	versionsByHash := map[string]string{}
	{

		tagsByHash, err := r.tags(repo)
		if err != nil {
			return "", microerror.Mask(err)
		}
		for hash, tags := range tagsByHash {
			for _, t := range tags {
				var versionTags []string
				if tagRegex.MatchString(t) {
					versionTags = append(versionTags, t)
					versionsByHash[hash] = strings.TrimPrefix(t, "v")
				}

				if len(versionTags) > 1 {
					return "", microerror.Maskf(executionFailedError, "multiple version tags %#v found for hash %#q", versionTags, hash)
				}
			}
		}

	}

	var commit *object.Commit
	{
		hash, err := repo.ResolveRevision(plumbing.Revision("origin/" + ref))
		if err != nil {
			hash, err = repo.ResolveRevision(plumbing.Revision(ref))
			if err != nil {
				return "", microerror.Mask(err)
			}
		}

		commit, err = repo.CommitObject(*hash)
		if err != nil {
			return "", microerror.Mask(err)
		}
	}

	// When the commit is tagged return the tag.
	{
		version, ok := versionsByHash[commit.Hash.String()]
		if ok {
			return version, nil
		}
	}

	// Otherwise find the first tagged parent and return it's tag glued
	// with the SHA.
	var pseudoVersion string
	{
		var lastVersion string

		queue := []*object.Commit{
			commit,
		}

		for {
			if len(queue) == 0 {
				lastVersion = "0.0.0"
				break
			}

			// Pop the first element from the queue.
			c := queue[0]
			queue = queue[1:]

			// Check if this commit is tagged. If so the most
			// recent tag is found and loop should be finished.
			v, ok := versionsByHash[c.Hash.String()]
			if ok {
				lastVersion = v
				break
			}

			// Push all the parents to the queue.
			c.Parents().ForEach(func(p *object.Commit) error {
				queue = append(queue, p)
				return nil
			})

			// Sort commits in the queue by commit date in
			// descending order to find the most recent tag first.
			sort.Sort(commitSlice(queue))
		}

		pseudoVersion = lastVersion + "-" + commit.Hash.String()
	}

	return pseudoVersion, nil
}

func (r *Repo) tags(repo *git.Repository) (map[string][]string, error) {
	tagsIter, err := repo.Tags()
	if err != nil {
		return nil, microerror.Mask(err)
	}
	defer tagsIter.Close()

	tags := map[string][]string{}

	tagsIter.ForEach(func(tag *plumbing.Reference) error {
		v := tags[tag.Hash().String()]
		if v == nil {
			v = []string{}
		}
		v = append(v, tag.Name().Short())

		tags[tag.Hash().String()] = v

		return nil
	})

	return tags, nil
}

type commitSlice []*object.Commit

func (p commitSlice) Len() int           { return len(p) }
func (p commitSlice) Less(i, j int) bool { return p[i].Committer.When.After(p[j].Committer.When) }
func (p commitSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
