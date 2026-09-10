package gitsemver

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// tagRegex and stableTagRegex reuse numID (from validate.go) so that the
// no-leading-zeros rule (semver §2) is enforced consistently with
// semverParseRegex in next.go.
var tagRegex = regexp.MustCompile(`^v` + numID + `\.` + numID + `\.` + numID + `(-rc\.` + numID + `)?$`)
var stableTagRegex = regexp.MustCompile(`^v` + numID + `\.` + numID + `\.` + numID + `$`)

var tagPrefixEnvVarName = "GS_GIT_TAG_PREFIX"
var prefixedTagRegex = regexp.MustCompile(`^[a-zA-Z0-9-_]+/v[0-9]+\.[0-9]+\.[0-9]+`)

var branchEnvVarName = "GS_BRANCH_NAME"

const unknownBranch = "unknown"

const (
	// devShortSHALen is the number of hex characters of the commit hash kept in
	// a dev version, matching the conventional git short-hash length.
	devShortSHALen = 7
	// devTimeLayout formats the committer date of a dev build. It carries no
	// separators so the whole pre-release part stays free of "." and "-".
	devTimeLayout = "20060102150405"
)

// CurrentBranch returns the name of the branch whose fingerprint is embedded in
// dev build versions.
// Priority: GS_BRANCH_NAME env var > HEAD branch of CWD git repo > unknownBranch.
func CurrentBranch() string {
	if b := strings.TrimSpace(os.Getenv(branchEnvVarName)); b != "" {
		return b
	}
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return unknownBranch
	}
	head, err := repo.Head()
	if err != nil {
		return unknownBranch
	}
	return head.Name().Short()
}

// BranchHash returns the fixed-width branch fingerprint used in dev build
// versions: the CRC32 checksum of the raw, unsanitized branch name in lowercase
// hex, padded to 8 digits. A fingerprint pins a build to its branch without
// putting a variable-length, possibly illegal branch name into the tag.
//
// The algorithm is CRC-32/ISO-HDLC, the variant that Go's crc32.ChecksumIEEE
// and Python's zlib.crc32 implement. The POSIX cksum tool uses a different
// variant and returns a different value.
func BranchHash(branch string) string {
	return fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(branch)))
}

// buildDevVersion assembles a dev build version of the form
// "X.Y.Z-b<branch-hash>t<YYYYMMDDHHMMSS>c<short-sha>", for example
// "1.9.2-b7b5b4fa7t20260127094959c1a2b3c4".
//
// The pre-release part is always 33 characters and holds no "." and no "-", so
// a caller that concatenates the version into a Kubernetes label and trims the
// result cannot cut it on a character Kubernetes rejects. The literal "b", "t"
// and "c" prefixes keep every part alphanumeric: an all-digit pre-release
// identifier is compared numerically and forbids leading zeros, which would
// break time stamps.
//
// commitSHA is the full commit hash; its first devShortSHALen hex characters are
// embedded. For one branch the "b<hash>t" prefix is constant, so semVer compares
// the fixed-width time stamps and the per-branch order stays chronological.
func buildDevVersion(base, branch, commitSHA string, t time.Time) string {
	short := commitSHA
	if len(short) > devShortSHALen {
		short = short[:devShortSHALen]
	}
	return fmt.Sprintf("%s-b%st%sc%s", base, BranchHash(branch), t.Format(devTimeLayout), short)
}

func incrementPatch(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", &ExecutionFailedError{message: fmt.Sprintf("invalid version %q: expected X.Y.Z", version)}
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", &ExecutionFailedError{message: fmt.Sprintf("invalid patch in version %q: %v", version, err)}
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, "."), nil
}

type Config struct {
	AuthBasicToken string
	Dir            string
	URL            string
	// WarnWriter is where non-fatal warnings are written. If nil, os.Stderr is
	// used. Set this to redirect or suppress warnings in library use.
	WarnWriter io.Writer
}

type Repo struct {
	url string

	auth     transport.AuthMethod
	storage  *filesystem.Storage
	worktree billy.Filesystem

	// warn is where non-fatal warnings (e.g. a commit carrying multiple
	// version tags) are written. It defaults to os.Stderr so warnings never
	// pollute the version printed to stdout; tests override it to capture output.
	warn io.Writer
}

func New(config Config) (*Repo, error) {
	if config.Dir == "" {
		return nil, &InvalidConfigError{message: fmt.Sprintf("%T.Dir must not be empty", config)}
	}

	var auth transport.AuthMethod
	if config.AuthBasicToken != "" {
		auth = &http.BasicAuth{
			Username: "can-be-anything-but-not-empty",
			Password: config.AuthBasicToken,
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
			return nil, &InvalidConfigError{message: fmt.Sprintf("%T.URL not set and failed to open repository with error %#q", config, err)}
		}

		remoteName := "origin"

		remote, err := repo.Remote(remoteName)
		if err != nil {
			return nil, &InvalidConfigError{message: fmt.Sprintf("%T.URL not set and failed to find remote with name %#q with error %#q", config, remoteName, err)}
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

	warnW := io.Writer(os.Stderr)
	if config.WarnWriter != nil {
		warnW = config.WarnWriter
	}
	r := &Repo{
		url: config.URL,

		auth:     auth,
		storage:  storage,
		worktree: worktree,
		warn:     warnW,
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
		return err
	}

	repo, err := git.Clone(r.storage, r.worktree, cloneOpts)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err = git.Open(r.storage, r.worktree)
		if err != nil {
			return err
		}
	} else if errors.Is(err, transport.ErrRepositoryNotFound) {
		return &RepositoryNotFoundError{message: fmt.Sprintf("%#q", r.url)}
	} else if err != nil {
		return err
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
		// Also, Clone creates the folder on the filesystem even if it fails, so you end simulate the same situation when
		// you call EnsureUpToDate more that once on the same non-existent repo.
		return &RepositoryNotFoundError{message: fmt.Sprintf("%#q", r.url)}
	} else if err != nil {
		return err
	}

	return nil
}

// HeadBranch returns branch name for the HEAD ref.
func (r *Repo) HeadBranch(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Name().Short(), nil
}

// HeadSHA returns sha for the HEAD ref.
func (r *Repo) HeadSHA(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}

// HeadTag returns tag for the HEAD ref.
//
// If GS_GIT_TAG_PREFIX environment variable is set, it looks for tags prefixed with that.
// For example, when the value is 'module-a', it filters found tags to 'module-a/v1.2.0',
// must match <module_name>/v<semantic_version>.
//
// Note: if GS_GIT_TAG_PREFIX is not set, all tags matching the prefixed tag regex are filtered out!
//
// It returns a *ReferenceNotFoundError when the HEAD ref is not tagged,
// detectable via errors.Is(err, &ReferenceNotFoundError{}).
func (r *Repo) HeadTag(ctx context.Context) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	tagsBySHA, err := r.tags(repo)
	if err != nil {
		return "", err
	}

	tags := tagsBySHA[head.Hash().String()]

	tagPrefix := os.Getenv(tagPrefixEnvVarName)

	var filteredTags []string
	if tagPrefix != "" {
		for _, tag := range tags {
			if strings.HasPrefix(tag, tagPrefix+"/") {
				filteredTags = append(filteredTags, tag)
			}
		}
	} else {
		for _, tag := range tags {
			if !prefixedTagRegex.MatchString(tag) {
				filteredTags = append(filteredTags, tag)
			}
		}
	}

	if len(filteredTags) == 0 {
		return "", &ReferenceNotFoundError{message: fmt.Sprintf("HEAD ref is not tagged (filtered for prefix: '%s')", tagPrefix)}
	}
	if len(filteredTags) > 1 {
		return "", &ExecutionFailedError{message: fmt.Sprintf("HEAD ref has multiple tags %v (filtered for prefix: '%s')", filteredTags, tagPrefix)}
	}

	return filteredTags[0], nil
}

// buildVersionMaps filters tagsByHash by tagPrefix and returns two maps keyed
// by commit SHA: versionsByHash (all semver tags) and stableVersionsByHash
// (stable-only tags). Version strings in both maps have no leading "v".
//
// When a single commit carries more than one version tag — which happens, for
// example, when several tags point at the same commit — the highest tag in
// semver terms is chosen and a warning listing every discovered tag and the
// chosen one is written to warn.
func buildVersionMaps(warn io.Writer, tagsByHash map[string][]string, tagPrefix string) (versionsByHash, stableVersionsByHash map[string]string, err error) {
	versionsByHash = map[string]string{}
	stableVersionsByHash = map[string]string{}

	// versionOf strips the prefix and leading "v" so the remainder can be
	// parsed and compared by compareSemver.
	versionOf := func(t string) string { return strings.TrimPrefix(t, "v") }
	if tagPrefix != "" {
		versionOf = func(t string) string {
			return strings.TrimPrefix(strings.TrimPrefix(t, tagPrefix+"/"), "v")
		}
	}

	for hash, tags := range tagsByHash {
		var versionTags, stableTags []string
		for _, t := range tags {
			if tagPrefix != "" {
				if prefixedTagRegex.MatchString(t) && strings.HasPrefix(t, tagPrefix+"/") {
					versionTags = append(versionTags, t)
					if stableTagRegex.MatchString(strings.TrimPrefix(t, tagPrefix+"/")) {
						stableTags = append(stableTags, t)
					}
				}
			} else {
				if tagRegex.MatchString(t) {
					versionTags = append(versionTags, t)
					if stableTagRegex.MatchString(t) {
						stableTags = append(stableTags, t)
					}
				}
			}
		}

		if len(versionTags) > 0 {
			chosen, err := highestVersionTag(versionTags, versionOf)
			if err != nil {
				return nil, nil, err
			}
			if len(versionTags) > 1 {
				_, _ = fmt.Fprintf(warn, "warning: commit %s carries multiple version tags %s; using the highest, %q\n", hash, strings.Join(versionTags, ", "), chosen)
			}
			versionsByHash[hash] = versionOf(chosen)
		}
		if len(stableTags) > 0 {
			chosen, err := highestVersionTag(stableTags, versionOf)
			if err != nil {
				return nil, nil, err
			}
			stableVersionsByHash[hash] = versionOf(chosen)
		}
	}
	return versionsByHash, stableVersionsByHash, nil
}

// highestVersionTag returns the tag with the highest semver precedence among
// tags, all of which are assumed to belong to the same commit. versionOf maps a
// tag name to the bare version string that compareSemver understands (prefix
// and leading "v" stripped). tags must be non-empty.
func highestVersionTag(tags []string, versionOf func(string) string) (string, error) {
	best := tags[0]
	bestParsed, err := parseVersionString(versionOf(best))
	if err != nil {
		return "", err
	}
	for _, t := range tags[1:] {
		parsed, err := parseVersionString(versionOf(t))
		if err != nil {
			return "", err
		}
		if compareSemver(parsed, bestParsed) > 0 {
			best, bestParsed = t, parsed
		}
	}
	return best, nil
}

// ResolveVersion resolves the version for a git reference:
//
//   - Stable tag vX.Y.Z on the commit → returns "X.Y.Z"
//   - Pre-release tag vX.Y.Z-<pre> on the commit → returns "X.Y.Z-<pre>" (e.g. "1.2.3-rc.1")
//   - Untagged commit → returns a semVer dev build:
//     "X.Y.(Z+1)-b<branch-hash>t<YYYYMMDDHHMMSS>c<commit-sha>"
//     where X.Y.Z is the most recent stable (non-pre-release) ancestor tag reachable
//     from the reference, or "0.0.0" when no stable ancestor exists. The timestamp is
//     the committer date (in UTC) of the resolved commit, so the version is stable for
//     a given commit. "c<commit-sha>" is the 7-char git short hash of the resolved
//     commit, for tag-to-commit traceability. "b<branch-hash>" is the CRC32 fingerprint
//     of the branch name (see BranchHash), resolved from the GS_BRANCH_NAME env var,
//     then the HEAD branch of the CWD git repo, then "unknown". Non-reachable tags are
//     never used as the base.
//
// The pre-release part is always 33 characters and holds no "." and no "-", so the
// version stays usable as a part of a Kubernetes attribute even after a caller trims
// the concatenated result. See buildDevVersion.
//
// If GS_GIT_TAG_PREFIX is set, only tags prefixed with "<value>/" are considered,
// e.g. "module-a/v1.2.3". The prefix, separator, and "v" are stripped from the result.
//
// It returns a *ReferenceNotFoundError when the reference cannot be resolved,
// detectable via errors.Is(err, &ReferenceNotFoundError{}).
func (r *Repo) ResolveVersion(ctx context.Context, ref string) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", err
	}

	tagPrefix := os.Getenv(tagPrefixEnvVarName)

	tagsByHash, err := r.tags(repo)
	if err != nil {
		return "", err
	}
	versionsByHash, stableVersionsByHash, err := buildVersionMaps(r.warn, tagsByHash, tagPrefix)
	if err != nil {
		return "", err
	}

	var commit *object.Commit
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", &ReferenceNotFoundError{message: fmt.Sprintf("%#q", ref)}
	} else if err != nil {
		return "", err
	}
	commit, err = repo.CommitObject(*hash)
	if err != nil {
		return "", err
	}

	// When the commit is tagged return the tag.
	if version, ok := versionsByHash[commit.Hash.String()]; ok {
		return version, nil
	}

	// Otherwise walk ancestors to find the most recent stable tagged parent,
	// then return a semVer-compatible dev build version.
	// The queue only ever holds commits reachable from `commit` (parents of parents,
	// etc.), so stableVersionsByHash is only consulted for actual ancestors — tags
	// on unrelated branches are never returned.
	var lastStableVersion string

	queue := []*object.Commit{
		commit,
	}

	for len(queue) > 0 {
		// Pop the first element from the queue.
		c := queue[0]
		queue = queue[1:]

		// Check if this commit has a stable tag. RC and other pre-release
		// tags are intentionally skipped so the dev build base is always
		// derived from the last stable release.
		v, ok := stableVersionsByHash[c.Hash.String()]
		if ok {
			lastStableVersion = v
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
			return "", err
		}

		// Sort commits in the queue by commit date in
		// descending order to find the most recent tag first.
		sort.Slice(queue, func(i, j int) bool { return queue[i].Committer.When.After(queue[j].Committer.When) })
	}

	var base string
	if lastStableVersion == "" {
		base = "0.0.0"
	} else {
		base, err = incrementPatch(lastStableVersion)
		if err != nil {
			return "", err
		}
	}
	t := commit.Committer.When.UTC()

	return buildDevVersion(base, CurrentBranch(), commit.Hash.String(), t), nil
}

// NextVersion returns the next version tag after the highest-semver tag
// reachable from HEAD for the given bumpType.
//
// Valid bump types from a stable tag: patch, minor, major, patch-rc, minor-rc, major-rc.
// Valid bump types from an RC tag: rc (bump counter), rc-release (finalize to stable).
//
// When no version tags are reachable from HEAD, v0.0.0 is used as the implicit base,
// which means only the stable bump types (patch/minor/major/patch-rc/minor-rc/major-rc)
// are valid.
//
// The returned string never carries a leading "v". If GS_GIT_TAG_PREFIX is set,
// only tags carrying that prefix are considered (monorepo support).
func (r *Repo) NextVersion(ctx context.Context, bumpType string) (string, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return "", err
	}

	tagPrefix := os.Getenv(tagPrefixEnvVarName)

	tagsByHash, err := r.tags(repo)
	if err != nil {
		return "", err
	}
	versionsByHash, _, err := buildVersionMaps(r.warn, tagsByHash, tagPrefix)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", &ReferenceNotFoundError{message: "HEAD"}
		}
		return "", err
	}

	// Walk every commit reachable from HEAD and find the highest-semver tagged one.
	var best *parsedVersion
	var bestStr string

	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return "", err
	}
	defer iter.Close()
	err = iter.ForEach(func(c *object.Commit) error {
		v, ok := versionsByHash[c.Hash.String()]
		if !ok {
			return nil
		}
		pv, parseErr := parseVersionString(v)
		if parseErr != nil {
			return nil // skip any tag that can't be parsed as semver
		}
		if best == nil || compareSemver(pv, *best) > 0 {
			best = &pv
			bestStr = v
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if bestStr == "" && (bumpType == "rc" || bumpType == "rc-release") {
		return "", &ExecutionFailedError{message: "no version tag reachable from HEAD; use patch, minor, major, patch-rc, minor-rc, or major-rc to start from 0.0.0"}
	}

	lastTag := "v0.0.0"
	if bestStr != "" {
		lastTag = bestStr
	}

	return ComputeNextVersion(lastTag, bumpType)
}

// ReadFileAtRef retrieves content of file stored at path on version specified in ref.
// When empty ref defaults to master branch.
// Note: internally this checks out the given ref, mutating the working tree.
func (r *Repo) ReadFileAtRef(ctx context.Context, path, ref string) ([]byte, error) {
	worktree, err := r.checkoutRef(ref)
	if err != nil {
		return nil, err
	}

	file, err := worktree.Filesystem.Open(path)
	if os.IsNotExist(err) {
		return nil, &FileNotFoundError{message: fmt.Sprintf("%#q", path)}
	} else if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// ReadFolderAtRef retrieves content of a folder stored at path on version specified in ref.
// When empty ref defaults to master branch.
// Note: internally this checks out the given ref, mutating the working tree.
func (r *Repo) ReadFolderAtRef(ctx context.Context, path, ref string) ([]os.FileInfo, error) {
	worktree, err := r.checkoutRef(ref)
	if err != nil {
		return nil, err
	}

	files, err := worktree.Filesystem.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, &FolderNotFoundError{message: fmt.Sprintf("%#q", path)}
	} else if err != nil {
		return nil, err
	}

	return files, nil
}

func (r *Repo) checkoutRef(ref string) (*git.Worktree, error) {
	repo, err := git.Open(r.storage, r.worktree)
	if err != nil {
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	// When empty CheckoutOptions defaults to master branch.
	opt := &git.CheckoutOptions{}
	if ref != "" {
		hash, err := repo.ResolveRevision(plumbing.Revision(ref))
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, &ReferenceNotFoundError{message: fmt.Sprintf("%#q", ref)}
		} else if err != nil {
			return nil, err
		}

		head, err := repo.Head()
		if err != nil {
			return nil, err
		}

		if head.Hash() == *hash {
			// We're already at the right ref, no need to checkout
			return worktree, nil
		}

		opt.Hash = *hash
	}

	err = worktree.Checkout(opt)
	if err != nil {
		return nil, err
	}

	err = worktree.Clean(&git.CleanOptions{Dir: true})
	if err != nil {
		return nil, err
	}

	return worktree, nil
}

func (r *Repo) tags(repo *git.Repository) (map[string][]string, error) {
	tags := map[string][]string{}

	// Get lightweight tags.
	{
		tagsIter, err := repo.Tags()
		if err != nil {
			return nil, err
		}
		defer tagsIter.Close()

		err = tagsIter.ForEach(func(tag *plumbing.Reference) error {
			tags[tag.Hash().String()] = append(tags[tag.Hash().String()], tag.Name().Short())
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Get tag objects.
	{
		tagObjectsIter, err := repo.TagObjects()
		if err != nil {
			return nil, err
		}
		defer tagObjectsIter.Close()

		err = tagObjectsIter.ForEach(func(tag *object.Tag) error {
			commit, err := tag.Commit()
			if err != nil {
				return err
			}
			tags[commit.Hash.String()] = append(tags[commit.Hash.String()], tag.Name)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return tags, nil
}
