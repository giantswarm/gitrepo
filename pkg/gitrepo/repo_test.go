package gitrepo

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden files")

// Test_New_optionalURL tests if proper URL from origin branch is taken from
// existing repository if none is specified.
func Test_New_optionalURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dir := "/tmp/gitrepo-test-new-optionalurl"
	defer os.RemoveAll(dir)

	url := "git@github.com:giantswarm/gitrepo-test.git"

	// Clone the repo first.
	{
		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/gitrepo-test.git",
		}

		repo, err := New(c)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.EnsureUpToDate(ctx)
		if err != nil {
			t.Fatalf("err = %v, want = %v", microerror.JSON(err), nil)
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
// a repositoryNotFoundError when the repo does not exist.
func Test_Repo_EnsureUpToDate_nosuchrepo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var err error

	dir := "/tmp/gitrepo-test-ensureuptodate-nosuchrepo"
	defer os.RemoveAll(dir)

	// Checkout the gitrepo-test repository.
	var repo *Repo
	{
		defer os.RemoveAll(dir)

		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/does-not-exist.git",
		}
		repo, err = New(c)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Ensure we get a repositoryNotFoundError when we don't have repo on the filesystem
	err = repo.EnsureUpToDate(ctx)
	if !IsRepositoryNotFound(err) {
		t.Fatalf("err = %v, want %v", microerror.JSON(err), repositoryNotFoundError)
	}

	// Even if clone fails the first time, it's leaking the directory on the filesystem.
	// Ensure we keep getting a repositoryNotFoundError once repo is on the filesystem.
	err = repo.EnsureUpToDate(ctx)
	if !IsRepositoryNotFound(err) {
		t.Fatalf("err = %v, want %v", microerror.JSON(err), repositoryNotFoundError)
	}
}

// Test_Repo_Head tests Repo.HeadBranch, Repo.HeadSHA and Repo.HeadTag methods.
func Test_Repo_Head(t *testing.T) {
	ctx := context.Background()
	var err error

	dir := "/tmp/gitrepo-test-repo-headbranch"
	defer os.RemoveAll(dir)

	// Checkout the gitrepo-test repository.
	var repo *Repo
	{
		defer os.RemoveAll(dir)

		c := Config{
			Dir: dir,
			URL: "git@github.com:giantswarm/gitrepo-test.git",
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
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
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
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}

			expectedHeadSHA = ref.Hash().String()
		}

		headSHA, err := repo.HeadSHA(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
		}
		if !reflect.DeepEqual(headSHA, expectedHeadSHA) {
			t.Fatalf("headSHA = %v, want %v", headSHA, expectedHeadSHA)
		}
	}

	// Test HeadTag (with multiple tags for modules as well).
	{
		_, err := repo.HeadTag(ctx)
		if !IsReferenceNotFound(err) {
			t.Fatalf("err = %v, want %v", err, referenceNotFoundError)
		}

		// Create "test-tag" tag on HEAD.
		{
			gitRepo, err := git.Open(repo.storage, nil)
			if err != nil {
				t.Errorf("unexpected error in git.Open: %v", microerror.JSON(err))
			}

			head, err := gitRepo.Head()
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}

			_, err = gitRepo.CreateTag("test-tag", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}

			_, err = gitRepo.CreateTag("module-a/v1.0.0", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}

			_, err = gitRepo.CreateTag("module-b/v2.1.0", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}

			_, err = gitRepo.CreateTag("module-c/v0.7.5", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
			}
		}

		// Look for normal tag without prefix
		tag, err := repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
		}
		if !reflect.DeepEqual(tag, "test-tag") {
			t.Fatalf("tag = %v, want %v", tag, "test-tag")
		}

		// Look for module-a tag
		t.Setenv(tagPrefixEnvVarName, "module-a")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
		}
		if !reflect.DeepEqual(tag, "module-a/v1.0.0") {
			t.Fatalf("tag = %v, want %v", tag, "module-a/v1.0.0")
		}

		// Look for module-b tag
		t.Setenv(tagPrefixEnvVarName, "module-b")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
		}
		if !reflect.DeepEqual(tag, "module-b/v2.1.0") {
			t.Fatalf("tag = %v, want %v", tag, "module-b/v2.1.0")
		}

		// Look for module-c tag
		t.Setenv(tagPrefixEnvVarName, "module-c")
		tag, err = repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
		}
		if !reflect.DeepEqual(tag, "module-c/v0.7.5") {
			t.Fatalf("tag = %v, want %v", tag, "module-c/v0.7.5")
		}
	}
}

// Test_Repo_ResolveVersion tests Repo.ResolveVersion method which resolve
// a git reference and find the project version for it. Tested repository can
// be found at https://github.com/giantswarm/gitrepo-test.
func Test_Repo_ResolveVersion(t *testing.T) {
	const masterTarget = "ref: refs/heads/master"
	const monorepoTarget = "ref: refs/heads/monorepo"

	testCases := []struct {
		name            string
		inputHeadTarget string
		environment     map[string]string
		inputRef        string
		expectedVersion string
		errorMatcher    func(err error) bool
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
			inputRef:        "2091354c7b8659f1846a876fbe2032fd1390d569",
			expectedVersion: "0.0.0-2091354c7b8659f1846a876fbe2032fd1390d569",
		},
		{
			name:            "case 5: untagged commit without tagged parent with detached head",
			inputHeadTarget: "2091354c7b8659f1846a876fbe2032fd1390d569",
			inputRef:        "HEAD",
			expectedVersion: "0.0.0-2091354c7b8659f1846a876fbe2032fd1390d569",
		},
		{
			name:            "case 6: untagged commit with single tagged parent",
			inputHeadTarget: masterTarget,
			inputRef:        "5ff7013b7a5f43d39b8da62361cfbfd4d3bf9a50",
			expectedVersion: "1.0.0-5ff7013b7a5f43d39b8da62361cfbfd4d3bf9a50",
		},
		{
			name:            "case 7: another untagged commit with single tagged parent",
			inputHeadTarget: masterTarget,
			inputRef:        "0c57573cece531f840a167aa0ccc29b178b6de42",
			expectedVersion: "2.0.0-0c57573cece531f840a167aa0ccc29b178b6de42",
		},
		{
			name:            "case 8: untagged commit with multiple tagged parents",
			inputHeadTarget: masterTarget,
			inputRef:        "c3726de44a2bb1bd898fdbe5632a90841636fa82",
			expectedVersion: "2.0.0-c3726de44a2bb1bd898fdbe5632a90841636fa82",
		},
		{
			name:            "case 9: untagged branch with single tagged parent",
			inputHeadTarget: masterTarget,
			inputRef:        "origin/branch-of-2.0.0",
			expectedVersion: "2.0.0-3901da4b6b4cf68e3d11a10f60916107828fa9a7",
		},
		{
			name:            "case 10: untagged branch with multiple tagged parents",
			inputHeadTarget: masterTarget,
			inputRef:        "origin/branch-of-1.0.0",
			expectedVersion: "2.0.0-c3726de44a2bb1bd898fdbe5632a90841636fa82",
		},
		{
			name:            "case 11: unknown reference",
			inputHeadTarget: masterTarget,
			inputRef:        "branch-of-1.0.0",
			errorMatcher:    IsReferenceNotFound,
		},
		{
			name:            "case 12: resolving complex tree with multiple common parents and long history",
			inputHeadTarget: masterTarget,
			inputRef:        "origin/complex-tree",
			expectedVersion: "0.0.0-a42e026e60b4c191ffb29430f439ad4b3aced71b",
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
			},
			inputRef:        "4707825fd7775c69fbd2f72a990e315b367b5409",
			expectedVersion: "0.2.0-4707825fd7775c69fbd2f72a990e315b367b5409",
		},
		{
			name:            "case 17: ...",
			inputHeadTarget: monorepoTarget,
			environment: map[string]string{
				tagPrefixEnvVarName: "module-not-exist",
			},
			inputRef:        "57aae3db71bcd176dd5a39eb8b487aae54930dcd",
			expectedVersion: "0.0.0-57aae3db71bcd176dd5a39eb8b487aae54930dcd",
		},
		{
			name:            "case 18: ...",
			inputHeadTarget: monorepoTarget,
			inputRef:        "ab61bc963b844551ffaf080f84d217e483323210",
			expectedVersion: "2.0.0-ab61bc963b844551ffaf080f84d217e483323210",
		},
		{
			name:            "case 19: ...",
			inputHeadTarget: monorepoTarget,
			inputRef:        "35d336b84623963eb4a9ea554b4ebf3f93a5d63d",
			environment: map[string]string{
				tagPrefixEnvVarName: "module-a",
			},
			expectedVersion: "0.0.0-35d336b84623963eb4a9ea554b4ebf3f93a5d63d",
		},
	}

	dir := "/tmp/gitrepo-test-repo-resolveversion"
	defer os.RemoveAll(dir)

	c := Config{
		Dir: dir,
		URL: "git@github.com:giantswarm/gitrepo-test.git",
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
					t.Fatalf("err = %v, want %v", microerror.JSON(err), nil)
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
				err = microerror.Mask(err)
				close(doneCh)
			}()

			select {
			case <-time.After(15 * time.Second):
				t.Fatalf("timeout after %v", 15*time.Second)
			case <-doneCh:
				switch {
				case err == nil && tc.errorMatcher == nil:
					// correct; carry on
				case err != nil && tc.errorMatcher == nil:
					t.Fatalf("error == %#v, want nil", err)
				case err == nil && tc.errorMatcher != nil:
					t.Fatalf("error == nil, want non-nil")
				case !tc.errorMatcher(err):
					t.Fatalf("error == %#v, want matching", err)
				}

				if version != tc.expectedVersion {
					t.Errorf("got %q, expected %q\n", version, tc.expectedVersion)
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
//	https://github.com/giantswarm/gitrepo-test.
//
// It uses golden file as reference and when changes are intentional,
// they can be updated by providing -update flag for go test:
//
// go test . -run Test_Repo_GetFileContent -update
func Test_Repo_GetFileContent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		path         string
		expected     string
		ref          string
		errorMatcher func(err error) bool
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
			name:         "case 4: handle file not found error",
			path:         "non/existent/file/path",
			errorMatcher: IsFileNotFound,
		},
		{
			name:         "case 5: handle reference not found error",
			path:         "DCO",
			ref:          "does-not-exist",
			errorMatcher: IsReferenceNotFound,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			dir := fmt.Sprintf("/tmp/gitrepo-test-repo-getfilecontent-%d", i)
			defer os.RemoveAll(dir)

			c := Config{
				Dir: dir,
				URL: "https://github.com/giantswarm/gitrepo-test",
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

			content, err := repo.GetFileContent(tc.path, tc.ref)

			switch {
			case err == nil && tc.errorMatcher == nil:
				// correct; carry on
			case err != nil && tc.errorMatcher == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.errorMatcher != nil:
				t.Fatalf("error == nil, want non-nil")
			case !tc.errorMatcher(err):
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
					expectedContent, err = os.ReadFile(golden)
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
//	https://github.com/giantswarm/gitrepo-test.
//
// It uses golden file as reference and when changes are intentional,
// they can be updated by providing -update flag for go test:
//
// go test . -run Test_Repo_GetFileContent -update
func Test_Repo_GetFolderContent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		path         string
		expected     string
		ref          string
		errorMatcher func(err error) bool
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
			name:         "case 3: folder not found error",
			path:         "non/existent",
			errorMatcher: IsFolderNotFound,
		},
		{
			name:         "case 4: handle reference not found error",
			path:         "DCO",
			ref:          "does-not-exist",
			errorMatcher: IsReferenceNotFound,
		},
	}

	dir := "/tmp/gitrepo-test-repo-getfoldercontent"
	defer os.RemoveAll(dir)

	c := Config{
		Dir: dir,
		URL: "git@github.com:giantswarm/gitrepo-test.git",
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

			files, err := repo.GetFolderContent(tc.path, tc.ref)

			switch {
			case err == nil && tc.errorMatcher == nil:
				// correct; carry on
			case err != nil && tc.errorMatcher == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.errorMatcher != nil:
				t.Fatalf("error == nil, want non-nil")
			case !tc.errorMatcher(err):
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
