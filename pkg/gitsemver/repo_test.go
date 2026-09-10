package gitsemver

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden files")

// Test_New_optionalURL tests if proper URL from origin branch is taken from
// existing repository if none is specified.
func Test_New_optionalURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dir := t.TempDir()

	url := "git@github.com:giantswarm/gitsemver-test.git"

	// Clone the repo first.
	{
		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/gitsemver-test.git",
		}

		repo, err := New(c)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.EnsureUpToDate(ctx)
		if err != nil {
			t.Fatalf("err = %v, want = %v", err, nil)
		}
	}

	// Open the repo without specifying URL and check if it is set
	// properly.
	{
		c := Config{
			Dir: dir,
		}

		repo, err := New(c)
		if err != nil {
			t.Fatal(err)
		}

		if repo.url != url {
			t.Fatalf("repo.url = %#q, want %#q", repo.url, url)
		}
	}
}

// Test_Repo_EnsureUpToDate_nosuchrepo tests that EnsureUpToDate returns
// a RepositoryNotFoundError when the repo does not exist.
func Test_Repo_EnsureUpToDate_nosuchrepo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var err error

	dir := t.TempDir()

	// Checkout the gitsemver-test repository.
	var repo *Repo
	{
		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/does-not-exist.git",
		}
		repo, err = New(c)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Ensure we get a RepositoryNotFoundError when we don't have repo on the filesystem
	err = repo.EnsureUpToDate(ctx)
	if !errors.Is(err, &RepositoryNotFoundError{}) {
		t.Fatalf("err = %v, want %v", err, RepositoryNotFoundError{})
	}

	// Even if clone fails the first time, it's leaking the directory on the filesystem.
	// Ensure we keep getting a RepositoryNotFoundError once repo is on the filesystem.
	err = repo.EnsureUpToDate(ctx)
	if !errors.Is(err, &RepositoryNotFoundError{}) {
		t.Fatalf("err = %v, want %v", err, RepositoryNotFoundError{})
	}
}

// Test_Repo_Head tests Repo.HeadBranch, Repo.HeadSHA and Repo.HeadTag methods.
func Test_Repo_Head(t *testing.T) {
	ctx := context.Background()
	var err error

	dir := t.TempDir()

	// Checkout the gitsemver-test repository.
	var repo *Repo
	{
		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/gitsemver-test.git",
		}
		repo, err = New(c)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.EnsureUpToDate(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test HeadBranch.
	{
		headBranch, err := repo.HeadBranch(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(headBranch, "master") {
			t.Fatalf("headBranch = %v, want %v", headBranch, "master")
		}
	}

	// Test HeadSHA.
	{
		var expectedHeadSHA string
		{
			ref, err := repo.storage.Reference(plumbing.Master)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}

			expectedHeadSHA = ref.Hash().String()
		}

		headSHA, err := repo.HeadSHA(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(headSHA, expectedHeadSHA) {
			t.Fatalf("headSHA = %v, want %v", headSHA, expectedHeadSHA)
		}
	}

	// Test HeadTag (with multiple tags for modules as well).
	{
		_, err := repo.HeadTag(ctx)
		if !errors.Is(err, &ReferenceNotFoundError{}) {
			t.Fatalf("err = %v, want %v", err, ReferenceNotFoundError{})
		}

		// Create "test-tag" tag on HEAD.
		{
			gitRepo, err := git.Open(repo.storage, nil)
			if err != nil {
				t.Errorf("unexpected error in git.Open: %v", err)
			}

			head, err := gitRepo.Head()
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}

			_, err = gitRepo.CreateTag("test-tag", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}

			_, err = gitRepo.CreateTag("module-a/v1.0.0", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}

			_, err = gitRepo.CreateTag("module-b/v2.1.0", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}

			_, err = gitRepo.CreateTag("module-c/v0.7.5", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", err, nil)
			}
		}

		// Look for normal tag without prefix
		tag, err := repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(tag, "test-tag") {
			t.Fatalf("tag = %v, want %v", tag, "test-tag")
		}

		// Look for module-a tag
		t.Setenv(tagPrefixEnvVarName, "module-a")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(tag, "module-a/v1.0.0") {
			t.Fatalf("tag = %v, want %v", tag, "module-a/v1.0.0")
		}

		// Look for module-b tag
		t.Setenv(tagPrefixEnvVarName, "module-b")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(tag, "module-b/v2.1.0") {
			t.Fatalf("tag = %v, want %v", tag, "module-b/v2.1.0")
		}

		// Look for module-c tag
		t.Setenv(tagPrefixEnvVarName, "module-c")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", err, nil)
		}
		if !reflect.DeepEqual(tag, "module-c/v0.7.5") {
			t.Fatalf("tag = %v, want %v", tag, "module-c/v0.7.5")
		}
	}
}

// Test_Repo_ResolveVersion tests Repo.ResolveVersion method which resolve
// a git reference and find the project version for it. Tested repository can
// be found at https://github.com/giantswarm/gitsemver-test.
func Test_Repo_ResolveVersion(t *testing.T) {
	const masterTarget = "ref: refs/heads/master"
	const monorepoTarget = "ref: refs/heads/monorepo"

	testCases := []struct {
		name            string
		inputHeadTarget string
		environment     map[string]string
		inputRef        string
		expectedVersion string
		expectedError   error
	}{
		{
			name:            "case 0: version tag",
			inputHeadTarget: masterTarget,
			inputRef:        "v1.0.0",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 1: another version tag",
			inputHeadTarget: masterTarget,
			inputRef:        "v2.0.0",
			expectedVersion: "2.0.0",
		},
		{
			name:            "case 2: tagged commit",
			inputHeadTarget: masterTarget,
			inputRef:        "02995edb3e6f14b8f9a83b84e3b8c7c8d9f60f86",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 3: another tagged commit",
			inputHeadTarget: masterTarget,
			inputRef:        "22b04802cd5ee933de078344fa53a3e37b826913",
			expectedVersion: "2.0.0",
		},
		{
			name:            "case 4: untagged commit without tagged parent",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "2091354c7b8659f1846a876fbe2032fd1390d569",
			expectedVersion: "0.0.0-b57a8dd33t20191108114200",
		},
		{
			name:            "case 5: untagged commit without tagged parent with detached head",
			inputHeadTarget: "2091354c7b8659f1846a876fbe2032fd1390d569",
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "HEAD",
			expectedVersion: "0.0.0-b57a8dd33t20191108114200",
		},
		{
			name:            "case 6: untagged commit with single tagged parent",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "5ff7013b7a5f43d39b8da62361cfbfd4d3bf9a50",
			expectedVersion: "1.0.1-b57a8dd33t20191108114503",
		},
		{
			name:            "case 7: another untagged commit with single tagged parent",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "0c57573cece531f840a167aa0ccc29b178b6de42",
			expectedVersion: "2.0.1-b57a8dd33t20191108114505",
		},
		{
			name:            "case 8: untagged commit with multiple tagged parents",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "c3726de44a2bb1bd898fdbe5632a90841636fa82",
			expectedVersion: "2.0.1-b57a8dd33t20191108114506",
		},
		{
			name:            "case 9: untagged branch with single tagged parent",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "origin/branch-of-2.0.0",
			expectedVersion: "2.0.1-b57a8dd33t20191108114506",
		},
		{
			name:            "case 10: untagged branch with multiple tagged parents",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "origin/branch-of-1.0.0",
			expectedVersion: "2.0.1-b57a8dd33t20191108114506",
		},
		{
			name:            "case 11: unknown reference",
			inputHeadTarget: masterTarget,
			inputRef:        "branch-of-1.0.0",
			expectedError:   &ReferenceNotFoundError{},
		},
		{
			name:            "case 12: resolving complex tree with multiple common parents and long history",
			inputHeadTarget: masterTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "origin/complex-tree",
			expectedVersion: "0.0.0-b57a8dd33t20191118120809",
		},
		{
			name:            "case 13: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-c",
			},
			inputRef:        "ab61bc963b844551ffaf080f84d217e483323210",
			expectedVersion: "2.0.0",
		},
		{
			name:            "case 14: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-a",
			},
			inputRef:        "4707825fd7775c69fbd2f72a990e315b367b5409",
			expectedVersion: "0.1.1",
		},
		{
			name:            "case 15: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-c",
			},
			inputRef:        "4707825fd7775c69fbd2f72a990e315b367b5409",
			expectedVersion: "1.1.0",
		},
		{
			name:            "case 16: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-b",
				branchEnvVarName:    "test-branch",
			},
			inputRef:        "4707825fd7775c69fbd2f72a990e315b367b5409",
			expectedVersion: "0.2.1-b57a8dd33t20240725131306",
		},
		{
			name:            "case 17: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-not-exist",
				branchEnvVarName:    "test-branch",
			},
			inputRef:        "57aae3db71bcd176dd5a39eb8b487aae54930dcd",
			expectedVersion: "0.0.0-b57a8dd33t20240725131406",
		},
		{
			name:            "case 18: ...",
			inputHeadTarget: monorepoTarget,
			environment:     map[string]string{branchEnvVarName: "test-branch"},
			inputRef:        "ab61bc963b844551ffaf080f84d217e483323210",
			expectedVersion: "2.0.1-b57a8dd33t20240725131325",
		},
		{
			name:            "case 19: ...",
			inputHeadTarget: monorepoTarget,
			inputRef:        "35d336b84623963eb4a9ea554b4ebf3f93a5d63d",
			environment: map[string]string{
				tagPrefixEnvVarName: "module-a",
				branchEnvVarName:    "test-branch",
			},
			expectedVersion: "0.0.0-b57a8dd33t20200424084147",
		},
	}

	dir := t.TempDir()

	c := Config{
		Dir: dir,
		URL: "git@github.com:giantswarm/gitsemver-test.git",
	}
	repo, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	err = repo.EnsureUpToDate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			var err error
			var version string

			// Set HEAD.
			{
				ref := plumbing.NewReferenceFromStrings(plumbing.HEAD.String(), tc.inputHeadTarget)
				err := repo.storage.SetReference(ref)
				if err != nil {
					t.Fatalf("err = %v, want %v", err, nil)
				}
			}

			doneCh := make(chan struct{})
			go func() {
				if tc.environment != nil {
					for key, value := range tc.environment {
						t.Setenv(key, value)
					}
				}

				version, err = repo.ResolveVersion(ctx, tc.inputRef)
				close(doneCh)
			}()

			select {
			case <-time.After(15 * time.Second):
				t.Fatalf("timeout after %v", 15*time.Second)
			case <-doneCh:
				switch {
				case err == nil && tc.expectedError == nil:
					// correct; carry on
				case err != nil && tc.expectedError == nil:
					t.Fatalf("error == %#v, want nil", err)
				case err == nil && tc.expectedError != nil:
					t.Fatalf("error == nil, want non-nil")
				case reflect.TypeOf(tc.expectedError) != reflect.TypeOf(err):
					t.Fatalf("error == %#v, want matching", err)
				}

				// Dev versions carry a trailing "c<7-hex>" short hash of the
				// resolved commit. Derive it from the same ref so the table can
				// keep the stable, hash-free prefix as the expectation. Every
				// expectation that is neither stable nor an RC is a dev prefix.
				expected := tc.expectedVersion
				if expected != "" && !IsValidStable(expected) && !IsValidRC(expected) {
					gitRepo, err := git.Open(repo.storage, repo.worktree)
					if err != nil {
						t.Fatalf("opening repo to resolve commit hash: %v", err)
					}
					h, err := gitRepo.ResolveRevision(plumbing.Revision(tc.inputRef))
					if err != nil {
						t.Fatalf("resolving %q: %v", tc.inputRef, err)
					}
					expected += "c" + h.String()[:devShortSHALen]
				}

				if version != expected {
					t.Errorf("got %q, expected %q\n", version, expected)
				}
			}
		})
	}
}

// Test_Repo_GetFileContent tests Repo.GetFileContent method which retrieves
// the content of a file.
//
// Tested repository can be found here:
//
//	https://github.com/giantswarm/gitsemver-test.
//
// It uses golden file as reference and when changes are intentional,
// they can be updated by providing -update flag for go test:
//
// go test . -run Test_Repo_GetFileContent -update
func Test_Repo_GetFileContent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		path          string
		expected      string
		ref           string
		expectedError error
	}{
		{
			name:     "case 0: get DCO file content",
			path:     "DCO",
			expected: "DCO",
		},
		{
			name:     "case 1: get DCO file content on default branch (master)",
			path:     "DCO",
			expected: "DCO",
			ref:      "master",
		},
		{
			name:     "case 2: get DCO file content on branch-of-2.0.0 branch",
			path:     "DCO",
			expected: "DCO",
			ref:      "origin/branch-of-2.0.0",
		},
		{
			name:     "case 3: get DCO file content on v2.0.0 tag",
			path:     "DCO",
			expected: "DCO",
			ref:      "v2.0.0",
		},
		{
			name:          "case 4: handle file not found error",
			path:          "non/existent/file/path",
			expectedError: &FileNotFoundError{},
		},
		{
			name:          "case 5: handle reference not found error",
			path:          "DCO",
			ref:           "does-not-exist",
			expectedError: &ReferenceNotFoundError{},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			// Must not pre-exist: EnsureUpToDate skips checkout when the dir is already there.
			dir := filepath.Join(t.TempDir(), "repo")

			c := Config{
				Dir: dir,
				URL: "https://github.com/giantswarm/gitsemver-test",
			}
			repo, err := New(c)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			err = repo.EnsureUpToDate(ctx)
			if err != nil {
				t.Fatal(err)
			}

			content, err := repo.ReadFileAtRef(ctx, tc.path, tc.ref)

			switch {
			case err == nil && tc.expectedError == nil:
				// correct; carry on
			case err != nil && tc.expectedError == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.expectedError != nil:
				t.Fatalf("error == nil, want non-nil")
			case reflect.TypeOf(tc.expectedError) != reflect.TypeOf(err):
				t.Fatalf("error == %#v, want matching", err)
			}

			if err == nil {
				var expectedContent []byte
				{
					golden := filepath.Join("testdata", tc.expected)
					if *update {
						err := os.WriteFile(golden, content, 0644) // #nosec G306
						if err != nil {
							t.Fatal(err)
						}
					}
					expectedContent, err = os.ReadFile(golden) // #nosec G304
					if err != nil {
						t.Fatal(err)
					}
				}

				if !bytes.Equal(content, expectedContent) {
					t.Errorf("\n%s\n", cmp.Diff(content, expectedContent))
				}
			}
		})
	}
}

// Test_Repo_GetFolderContent tests Repo.GetFolderContent method which retrieves
// the content of a folder.
//
// Tested repository can be found here:
//
//	https://github.com/giantswarm/gitsemver-test.
//
// It uses golden file as reference and when changes are intentional,
// they can be updated by providing -update flag for go test:
//
// go test . -run Test_Repo_GetFileContent -update
func Test_Repo_GetFolderContent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		path          string
		expected      string
		ref           string
		expectedError error
	}{
		{
			name:     "case 0: get folder contents",
			path:     ".",
			expected: "DCO",
		},
		{
			name:     "case 1: get folder contents on a branch",
			path:     ".",
			expected: "DCO",
			ref:      "origin/branch-of-2.0.0",
		},
		{
			name:     "case 2: get folder contents on a tag",
			path:     ".",
			expected: "DCO",
			ref:      "v2.0.0",
		},
		{
			name:          "case 3: folder not found error",
			path:          "non/existent",
			expectedError: &FolderNotFoundError{},
		},
		{
			name:          "case 4: handle reference not found error",
			path:          "DCO",
			ref:           "does-not-exist",
			expectedError: &ReferenceNotFoundError{},
		},
	}

	dir := t.TempDir()

	c := Config{
		Dir: dir,
		URL: "git@github.com:giantswarm/gitsemver-test.git",
	}
	repo, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	err = repo.EnsureUpToDate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			files, err := repo.ReadFolderAtRef(ctx, tc.path, tc.ref)

			switch {
			case err == nil && tc.expectedError == nil:
				// correct; carry on
			case err != nil && tc.expectedError == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.expectedError != nil:
				t.Fatalf("error == nil, want non-nil")
			case reflect.TypeOf(tc.expectedError) != reflect.TypeOf(err):
				t.Fatalf("error == %#v, want matching", err)
			}

			if err == nil {
				if !containsFile(files, tc.expected) {
					t.Fatalf("folder %s does not contain %s", tc.path, tc.expected)
				}
			}
		})
	}
}

func containsFile(files []os.FileInfo, fileName string) bool {
	for _, f := range files {
		if f.Name() == fileName {
			return true
		}
	}

	return false
}

func Test_incrementPatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"1.2.3", "1.2.4", false},
		{"0.0.0", "0.0.1", false},
		{"2.0.0", "2.0.1", false},
		{"1.9.9", "1.9.10", false},
		{"bad", "", true},
		{"1.2", "", true},
		{"1.2.x", "", true},
	}

	for _, tc := range cases {
		got, err := incrementPatch(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("incrementPatch(%q): expected error, got %q", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("incrementPatch(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("incrementPatch(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func Test_CurrentBranch_envOverride(t *testing.T) {
	t.Setenv(branchEnvVarName, "my-override-branch")
	got := CurrentBranch()
	if got != "my-override-branch" {
		t.Errorf("CurrentBranch() = %q, want %q", got, "my-override-branch")
	}
}

func Test_CurrentBranch_unknownFallback(t *testing.T) {
	t.Setenv(branchEnvVarName, "")
	t.Chdir(t.TempDir())
	got := CurrentBranch()
	if got != "unknown" {
		t.Errorf("CurrentBranch() = %q, want %q (non-git dir should fall back)", got, "unknown")
	}
}

func Test_CurrentBranch_detachedHead(t *testing.T) {
	dir := t.TempDir()
	gitRepo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	hash := testCreateCommit(t, gitRepo, "init.txt", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	w, _ := gitRepo.Worktree()
	if err := w.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(branchEnvVarName, "")
	t.Chdir(dir)
	got := CurrentBranch()
	if got != "HEAD" {
		t.Errorf("CurrentBranch() = %q, want %q (detached HEAD should return \"HEAD\")", got, "HEAD")
	}
}

// testCreateCommit adds a single file and creates a commit in gitRepo.
func testCreateCommit(t *testing.T, gitRepo *git.Repository, filename string, when time.Time) plumbing.Hash {
	t.Helper()
	w, err := gitRepo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	f, err := w.Filesystem.Create(filename)
	if err != nil {
		t.Fatalf("Create %s: %v", filename, err)
	}
	_, _ = fmt.Fprint(f, filename)
	_ = f.Close()
	if _, err = w.Add(filename); err != nil {
		t.Fatalf("Add %s: %v", filename, err)
	}
	sig := &object.Signature{Name: "test", Email: "t@t.com", When: when}
	hash, err := w.Commit(filename, &git.CommitOptions{Author: sig, Committer: sig})
	if err != nil {
		t.Fatalf("Commit %s: %v", filename, err)
	}
	return hash
}

// newTestRepo creates a fresh on-disk git repo in t.TempDir and wraps it in a Repo.
func newTestRepo(t *testing.T) (*Repo, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	gitRepo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	repo, err := New(Config{Dir: dir, URL: "https://example.com/test.git"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return repo, gitRepo
}

// Test_Repo_ResolveVersion_rcAncestor verifies that RC tags in the ancestry are
// skipped when computing the dev build base version; only stable vX.Y.Z tags count.
func Test_Repo_ResolveVersion_rcAncestor(t *testing.T) {
	ctx := context.Background()
	t.Setenv(branchEnvVarName, "test-branch")

	repo, gitRepo := newTestRepo(t)

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	stableHash := testCreateCommit(t, gitRepo, "stable.txt", base)
	if _, err := gitRepo.CreateTag("v1.0.0", stableHash, nil); err != nil {
		t.Fatal(err)
	}
	rcHash := testCreateCommit(t, gitRepo, "rc.txt", base.Add(time.Hour))
	if _, err := gitRepo.CreateTag("v1.1.0-rc.1", rcHash, nil); err != nil {
		t.Fatal(err)
	}
	headHash := testCreateCommit(t, gitRepo, "dev.txt", base.Add(2*time.Hour))

	version, err := repo.ResolveVersion(ctx, headHash.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Timestamp is the committer date of the HEAD commit (base + 2h); the
	// trailing segment is the short hash of the resolved HEAD commit.
	want := "1.0.1-b57a8dd33t20200101020000c" + headHash.String()[:devShortSHALen]
	if version != want {
		t.Errorf("ResolveVersion = %q, want %q — RC ancestor must be skipped, base must come from v1.0.0", version, want)
	}
}

// Test_Repo_ResolveVersion_nonReachableTagIgnored verifies that a higher-version
// stable tag present in the repo but on a non-ancestor branch is never used as
// the dev build base. Only tags reachable (i.e. ancestor commits) from the given
// ref may be considered.
func Test_Repo_ResolveVersion_nonReachableTagIgnored(t *testing.T) {
	ctx := context.Background()
	t.Setenv(branchEnvVarName, "test-branch")

	repo, gitRepo := newTestRepo(t)

	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	// Initial commit on the default (master) branch, tagged v1.0.0.
	stableHash := testCreateCommit(t, gitRepo, "stable.txt", base)
	if _, err := gitRepo.CreateTag("v1.0.0", stableHash, nil); err != nil {
		t.Fatal(err)
	}

	// Create a side branch from the initial commit and tag it v2.0.0.
	// This commit will NOT be reachable from our eventual HEAD on master.
	w, err := gitRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Checkout(&git.CheckoutOptions{
		Hash:   stableHash,
		Branch: plumbing.NewBranchReferenceName("side"),
		Create: true,
	}); err != nil {
		t.Fatal(err)
	}
	sideHash := testCreateCommit(t, gitRepo, "side.txt", base.Add(time.Hour))
	if _, err := gitRepo.CreateTag("v2.0.0", sideHash, nil); err != nil {
		t.Fatal(err)
	}

	// Return to master and create the HEAD commit (parent: stableHash).
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	}); err != nil {
		t.Fatal(err)
	}
	headHash := testCreateCommit(t, gitRepo, "head.txt", base.Add(2*time.Hour))

	// headHash ancestry: stableHash (v1.0.0). sideHash (v2.0.0) is not reachable.
	version, err := repo.ResolveVersion(ctx, headHash.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Timestamp is the committer date of the HEAD commit (base + 2h); the
	// trailing segment is the short hash of the resolved HEAD commit.
	want := "1.0.1-b57a8dd33t20200101020000c" + headHash.String()[:devShortSHALen]
	if version != want {
		t.Errorf("ResolveVersion = %q, want %q — non-reachable v2.0.0 on side branch must be ignored", version, want)
	}
}

// A commit carrying multiple version tags must not error: ResolveVersion picks
// the highest-semver tag and emits a warning listing every discovered tag and
// the one chosen.
func Test_Repo_ResolveVersion_multipleTagsOnSameCommit_picksHighest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo, gitRepo := newTestRepo(t)
	var warn bytes.Buffer
	repo.warn = &warn

	h := testCreateCommit(t, gitRepo, "a.txt", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	// Created lowest-last to prove selection is by semver, not creation order.
	for _, tag := range []string{"v1.0.1", "v1.2.0", "v1.0.0"} {
		if _, err := gitRepo.CreateTag(tag, h, nil); err != nil {
			t.Fatal(err)
		}
	}

	version, err := repo.ResolveVersion(ctx, h.String())
	if err != nil {
		t.Fatalf("ResolveVersion: unexpected error %v", err)
	}
	if version != "1.2.0" {
		t.Errorf("ResolveVersion = %q, want %q (highest semver among the tags)", version, "1.2.0")
	}

	out := warn.String()
	for _, want := range []string{"v1.0.0", "v1.0.1", "v1.2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning %q does not list discovered tag %q", out, want)
		}
	}
}

// When an untagged commit's nearest ancestor carries multiple stable tags, the
// dev-build base is derived from the highest of them.
func Test_Repo_ResolveVersion_devBuild_multipleAncestorTags_picksHighest(t *testing.T) {
	ctx := context.Background()
	t.Setenv(branchEnvVarName, "test-branch")

	repo, gitRepo := newTestRepo(t)
	var warn bytes.Buffer
	repo.warn = &warn

	base := testCreateCommit(t, gitRepo, "a.txt", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	for _, tag := range []string{"v1.0.0", "v1.0.1"} {
		if _, err := gitRepo.CreateTag(tag, base, nil); err != nil {
			t.Fatal(err)
		}
	}
	head := testCreateCommit(t, gitRepo, "b.txt", time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC))

	version, err := repo.ResolveVersion(ctx, head.String())
	if err != nil {
		t.Fatalf("ResolveVersion: unexpected error %v", err)
	}
	want := "1.0.2-b57a8dd33t20200101010000c" + head.String()[:7]
	if version != want {
		t.Errorf("ResolveVersion = %q, want %q (dev base from highest ancestor tag 1.0.1)", version, want)
	}
}

// When a commit carries both a stable tag and a higher-patched RC tag, the RC
// wins for versionsByHash (semver §11: 1.0.1-rc.1 > 1.0.0) while the stable
// tag is still tracked in stableVersionsByHash.
func Test_Repo_ResolveVersion_mixedStableRC_picksHighestSemver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo, gitRepo := newTestRepo(t)
	var warn bytes.Buffer
	repo.warn = &warn

	h := testCreateCommit(t, gitRepo, "a.txt", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	// RC created first to prove selection is by semver, not insertion order.
	for _, tag := range []string{"v1.0.1-rc.1", "v1.0.0"} {
		if _, err := gitRepo.CreateTag(tag, h, nil); err != nil {
			t.Fatal(err)
		}
	}

	version, err := repo.ResolveVersion(ctx, h.String())
	if err != nil {
		t.Fatalf("ResolveVersion: unexpected error %v", err)
	}
	// compareSemver §11: v1.0.1-rc.1 > v1.0.0 (patch 1 > 0), so the RC is chosen.
	if version != "1.0.1-rc.1" {
		t.Errorf("ResolveVersion = %q, want %q", version, "1.0.1-rc.1")
	}
}
