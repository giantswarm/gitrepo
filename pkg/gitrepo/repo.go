package gitrepo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/go-errors/errors"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

var tagRegex = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+`)

var tagPrefixEnvVarName = "GS_TAG_PREFIX"
var prefixedTagRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+/v[0-9]+\.[0-9]+\.[0-9]+`)

type Config struct {
	AuthBasicToken string
	Dir            string
	URL            string
}

type Repo struct {
	url string

	auth     transport.AuthMethod
	storage  *filesystem.Storage
	worktree billy.Filesystem
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

	worktree := osfs.New(config.Dir)
	fs := osfs.New(filepath.Join(config.Dir, ".git"))
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{})

	// When URL is not configured assume the repository is cloned on disk
	// and take the URL or origin remote.
	if config.URL == "" {
		repo, err := git.Open(storage, worktree)
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

		auth:     auth,
		storage:  storage,
		worktree: worktree,
	}

	return r, nil
}

// EnsureUpToDate fetches latest changes from remote.
func (r *Repo) EnsureUpToDate(ctx context.Context) error {
	cloneOpts := &git.CloneOptions{
		Auth:       r.auth,
		URL:        r.url,
		NoCheckout: true,
	}

	_, err := r.worktree.Stat("/")
	if os.IsNotExist(err) {
		// Repo is empty so perform an initial checkout
		cloneOpts.NoCheckout = false
	} else if err != nil {
		return microerror.Mask(err)
	}

	repo, err := git.Clone(r.storage, r.worktree, cloneOpts)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err = git.Open(r.storage, r.worktree)
		if err != nil {
			return microerror.Mask(err)
		}
	} else if errors.Is(err, transport.ErrRepositoryNotFound) {
		return microerror.Maskf(repositoryNotFoundError, "%#q", r.url)
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
	} else if errors.Is(err, transport.ErrRepositoryNotFound) {
		// This could happen if the repository does not exist, but you already have the folder on the filesystem.
		// In that case Fetch will be the first to realise that repo does not exist since Clone only performs an Open.
		// Also, Clone creates the folder on the filesystem even if it fails so you end simulate the same situation when
		// you call EnsureUpToDate more that once on the same non-existent repo.
		return microerror.Maskf(repositoryNotFoundError, "%#q", r.url)
	} else if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

// HeadBranch returns branch name for the HEAD ref.
func (r *Repo) HeadBranch(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", microerror.Mask(err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", microerror.Mask(err)
	}

	return head.Name().Short(), nil
}

// HeadSHA returns sha for the HEAD ref.
func (r *Repo) HeadSHA(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", microerror.Mask(err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", microerror.Mask(err)
	}

	return head.Hash().String(), nil
}

// HeadTag returns tag for the HEAD ref.
//
// It returns error handled by IsReferenceNotFound if the HEAD ref is not
// tagged.
func (r *Repo) HeadTag(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
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
		return "", microerror.Maskf(referenceNotFoundError, "HEAD ref is not tagged")
	}
	if len(tags) > 1 {
		return "", microerror.Maskf(executionFailedError, "HEAD ref has multiple tags %v", tags)
	}

	return tags[0], nil
}

// ResolveVersion resolves version of a reference. It may be a version in
// format "X.Y.Z" if the reference is tagged with tag in format "vX.Y.Z" (note
// that the "v" prefix is trimmed). Otherwise it will be a pseudo-version in
// format "X.Y.Z-SHA" where "X.Y.Z" part is the value taken from the most
// recent parent commit tagged with "vX.Y.Z" or "0.0.0" if no such parent exist
// and "SHA" part is the git SHA of the given reference.
//
// It returns error handled by IsReferenceNotFound if the HEAD ref is not
// tagged.
func (r *Repo) ResolveVersion(ctx context.Context, ref string) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", microerror.Mask(err)
	}

	tagPrefix := os.Getenv(tagPrefixEnvVarName)

	versionsByHash := map[string]string{}
	{

		tagsByHash, err := r.tags(repo)
		if err != nil {
			return "", microerror.Mask(err)
		}
		for hash, tags := range tagsByHash {
			for _, t := range tags {
				var versionTags []string

				if tagPrefix != "" {
					if prefixedTagRegex.MatchString(t) && strings.HasPrefix(t, tagPrefix+"/") {
						versionTags = append(versionTags, t)
						versionsByHash[hash] = strings.TrimPrefix(t, "v")
					}
				} else {
					if tagRegex.MatchString(t) {
						versionTags = append(versionTags, t)
						versionsByHash[hash] = strings.TrimPrefix(t, "v")
					}
				}

				if len(versionTags) > 1 {
					return "", microerror.Maskf(executionFailedError, "multiple version tags %#v found for hash %#q", versionTags, hash)
				}
			}
		}

	}

	var commit *object.Commit
	{
		hash, err := repo.ResolveRevision(plumbing.Revision(ref))
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", microerror.Maskf(referenceNotFoundError, "%#q", ref)
		} else if err != nil {
			return "", microerror.Mask(err)
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
			err = c.Parents().ForEach(func(p *object.Commit) error {
				// If the commit is already in the queue skip
				// it. This is possible multiple commits have
				// the same parent. Adding all of them to the
				// queue may lead in exponential growth of the
				// queue resulting in extremely long execution.
				for _, c := range queue {
					if c.Hash == p.Hash {
						return nil
					}
				}

				queue = append(queue, p)
				return nil
			})
			if err != nil {
				return "", microerror.Mask(err)
			}

			// Sort commits in the queue by commit date in
			// descending order to find the most recent tag first.
			sort.Slice(queue, func(i, j int) bool { return queue[i].Committer.When.After(queue[j].Committer.When) })
		}

		pseudoVersion = lastVersion + "-" + commit.Hash.String()
	}

	return pseudoVersion, nil
}

// GetFileContent retrieves content of file stored at path on version specified in ref.
// When empty ref defaults to master branch.
func (r *Repo) GetFileContent(path, ref string) ([]byte, error) {
	worktree, err := r.checkoutRef(ref)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	file, err := worktree.Filesystem.Open(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(fileNotFoundError, "%#q", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return content, nil
}

// GetFolderContent retrieves content of a folder stored at path on version specified in ref.
// When empty ref defaults to master branch.
func (r *Repo) GetFolderContent(path, ref string) ([]os.FileInfo, error) {
	worktree, err := r.checkoutRef(ref)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	files, err := worktree.Filesystem.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, microerror.Maskf(folderNotFoundError, "%#q", path)
	} else if err != nil {
		return nil, microerror.Mask(err)
	}

	return files, nil
}

func (r *Repo) checkoutRef(ref string) (*git.Worktree, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// When empty CheckoutOptions defaults to master branch.
	opt := &git.CheckoutOptions{}
	if ref != "" {
		hash, err := repo.ResolveRevision(plumbing.Revision(ref))
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, microerror.Maskf(referenceNotFoundError, "%#q", ref)
		} else if err != nil {
			return nil, microerror.Mask(err)
		}

		head, err := repo.Head()
		if err != nil {
			return nil, microerror.Mask(err)
		}

		if head.Hash() == *hash {
			// We're already at the right ref, no need to checkout
			return worktree, nil
		}

		opt.Hash = *hash
	}

	err = worktree.Checkout(opt)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	err = worktree.Clean(&git.CleanOptions{Dir: true})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return worktree, nil
}

func (r *Repo) tags(repo *git.Repository) (map[string][]string, error) {
	tags := map[string][]string{}

	// Get lightweight tags.
	{
		tagsIter, err := repo.Tags()
		if err != nil {
			return nil, microerror.Mask(err)
		}
		defer tagsIter.Close()

		err = tagsIter.ForEach(func(tag *plumbing.Reference) error {
			v := tags[tag.Hash().String()]
			if v == nil {
				v = []string{}
			}
			v = append(v, tag.Name().Short())

			tags[tag.Hash().String()] = v

			return nil
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	// Get tag objects.
	{
		tagObjectsIter, err := repo.TagObjects()
		if err != nil {
			return nil, microerror.Mask(err)
		}
		defer tagObjectsIter.Close()

		err = tagObjectsIter.ForEach(func(tag *object.Tag) error {
			commit, err := tag.Commit()
			if err != nil {
				return microerror.Mask(err)
			}

			v := tags[commit.Hash.String()]
			if v == nil {
				v = []string{}
			}
			v = append(v, tag.Name)

			tags[commit.Hash.String()] = v

			return nil
		})
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return tags, nil
}
