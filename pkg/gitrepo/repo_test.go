package gitrepo

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/giantswarm/microerror"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

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
			t.Fatalf("err = %v, want = %v", microerror.Stack(err), nil)
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

// Test_Repo_Head tests Repo.HeadBranch, Repo.HeadSHA and Repo.HeadTag methods.
func Test_Repo_Head(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var err error

	dir := "/tmp/gitrepo-test-repo-headbranch"

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
			t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
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
				t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
			}

			expectedHeadSHA = ref.Hash().String()
		}

		headSHA, err := repo.HeadSHA(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
		}
		if !reflect.DeepEqual(headSHA, expectedHeadSHA) {
			t.Fatalf("headSHA = %v, want %v", headSHA, expectedHeadSHA)
		}
	}

	// Test HeadTag.
	{
		_, err := repo.HeadTag(ctx)
		if !IsNotFound(err) {
			t.Fatalf("err = %v, want %v", err, notFoundError)
		}

		// Create "test-tag" tag on HEAD.
		{
			gitRepo, err := git.Open(repo.storage, nil)

			head, err := gitRepo.Head()
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
			}

			_, err = gitRepo.CreateTag("test-tag", head.Hash(), nil)
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
			}
		}

		tag, err := repo.HeadTag(ctx)
		if err != nil {
			t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
		}
		if !reflect.DeepEqual(tag, "test-tag") {
			t.Fatalf("tag = %v, want %v", tag, "test-tag")
		}
	}
}

// Test_Repo_ResolveVersion tests Repo.ResolveVersion method which resolve
// a git reference and find the project version for it. Tested repository can
// be found here:
//
//	https://github.com/giantswarm/gitrepo-test.
//
func Test_Repo_ResolveVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		inputRef        string
		expectedVersion string
	}{
		{
			name:            "case 0: version tag",
			inputRef:        "v1.0.0",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 1: another version tag",
			inputRef:        "v2.0.0",
			expectedVersion: "2.0.0",
		},
		{
			name:            "case 2: tagged commit",
			inputRef:        "02995edb3e6f14b8f9a83b84e3b8c7c8d9f60f86",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 3: another tagged commit",
			inputRef:        "22b04802cd5ee933de078344fa53a3e37b826913",
			expectedVersion: "2.0.0",
		},
		{
			name:            "case 4: untagged commit without tagged parent",
			inputRef:        "2091354c7b8659f1846a876fbe2032fd1390d569",
			expectedVersion: "0.0.0-2091354c7b8659f1846a876fbe2032fd1390d569",
		},
		{
			name:            "case 5: untagged commit with single tagged parent",
			inputRef:        "5ff7013b7a5f43d39b8da62361cfbfd4d3bf9a50",
			expectedVersion: "1.0.0-5ff7013b7a5f43d39b8da62361cfbfd4d3bf9a50",
		},
		{
			name:            "case 6: another untagged commit with single tagged parent",
			inputRef:        "0c57573cece531f840a167aa0ccc29b178b6de42",
			expectedVersion: "2.0.0-0c57573cece531f840a167aa0ccc29b178b6de42",
		},
		{
			name:            "case 7: untagged commit with multiple tagged parents",
			inputRef:        "c3726de44a2bb1bd898fdbe5632a90841636fa82",
			expectedVersion: "2.0.0-c3726de44a2bb1bd898fdbe5632a90841636fa82",
		},
		{
			name:            "case 8: untagged branch with single tagged parent",
			inputRef:        "branch-of-2.0.0",
			expectedVersion: "2.0.0-3901da4b6b4cf68e3d11a10f60916107828fa9a7",
		},
		{
			name:            "case 9: untagged branch with multiple tagged parents",
			inputRef:        "branch-of-1.0.0",
			expectedVersion: "2.0.0-c3726de44a2bb1bd898fdbe5632a90841636fa82",
		},
		{
			name:            "case 10: resolving complex tree with multiple common parents and long history",
			inputRef:        "complex-tree",
			expectedVersion: "0.0.0-a42e026e60b4c191ffb29430f439ad4b3aced71b",
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

			doneCh := make(chan struct{})
			go func() {
				version, err = repo.ResolveVersion(ctx, tc.inputRef)
				err = microerror.Mask(err)
				close(doneCh)
			}()

			select {
			case <-time.After(15 * time.Second):
				t.Fatalf("timeout after %v", 15*time.Second)
			case <-doneCh:
				if err != nil {
					t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
				}
				if version != tc.expectedVersion {
					t.Errorf("got %q, expected %q\n", version, tc.expectedVersion)
				}
			}
		})
	}
}
