package gitrepo

import (
	"context"
	"strconv"
	"testing"
)

// TestResolveVersion tests ResolveVersion method which resolve a git reference and find the project version for it.
func TestResolveVersion(t *testing.T) {
	testCases := []struct {
		name            string
		inputRef        string
		expectedVersion string
	}{
		{
			name:            "case 0: tag reference",
			inputRef:        "v0.1.0",
			expectedVersion: "0.1.0",
		},
		{
			name:            "case 1: untagged version",
			inputRef:        "2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
			expectedVersion: "0.0.0-2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
		},
		{
			name:            "case 2: tagged version",
			inputRef:        "d1dcd7e42b044858f14ad51ea68e2809c16deb84",
			expectedVersion: "0.1.0",
		},
		{
			name:            "case 3: above tagged version",
			inputRef:        "b62b39c5f762eae26979715599a0a9226547ef5e",
			expectedVersion: "0.1.0-b62b39c5f762eae26979715599a0a9226547ef5e",
		},
		{
			name:            "case 4: branch reference",
			inputRef:        "test-branch",
			expectedVersion: "0.1.0",
		},
	}

	c := Config{
		Dir: "/tmp/gitrepo",
		URL: "git@github.com:giantswarm/gitrepo.git",
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

	for i, test := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			version, err := repo.ResolveVersion(ctx, test.inputRef)
			if err != nil {
				t.Errorf("%#v", err)
			}
			if version != test.expectedVersion {
				t.Errorf("got %q, expected %q\n", version, test.expectedVersion)
			}
		})
	}
}
