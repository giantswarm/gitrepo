package gitrepo

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// TestResolveVersion tests ResolveVersion method which resolve a git reference
// and find the project version for it. Tested repository can be found here:
//
//	https://github.com/giantswarm/gitrepo-test.
//
func TestResolveVersion(t *testing.T) {
	testCases := []struct {
		name            string
		inputRef        string
		expectedVersion string
	}{
		{
			name:            "case 0: version tag",
			inputRef:        "v0.1.0",
			expectedVersion: "0.1.0",
		},
		{
			name:            "case 1: another version tag",
			inputRef:        "v1.0.0",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 2: tagged commit",
			inputRef:        "d589fbd97ae6f96325df5fbb16a1a5c7b7c7cce5",
			expectedVersion: "0.1.0",
		},
		{
			name:            "case 3: another tagged commit",
			inputRef:        "06b6c8e5945ad8e7bda96e66e9c14e2052abec91",
			expectedVersion: "1.0.0",
		},
		{
			name:            "case 4: untagged commit",
			inputRef:        "9fa9309a0275df54d61e1068f52950e794ce0f7a",
			expectedVersion: "0.0.0-9fa9309a0275df54d61e1068f52950e794ce0f7a",
		},
		{
			name:            "case 5: untagged commit with single tagged parent",
			inputRef:        "4bff3643d8a0b413d6892ca779d8816f73061c01",
			expectedVersion: "0.1.0-4bff3643d8a0b413d6892ca779d8816f73061c01",
		},
		{
			name:            "case 6: another untagged commit with single tagged parent",
			inputRef:        "54b81b007b74747954ba5bd5e8e75a5d039a0530",
			expectedVersion: "1.0.0-54b81b007b74747954ba5bd5e8e75a5d039a0530",
		},
		{
			name:            "case 7: untagged commit with multiple tagged parents",
			inputRef:        "02846e1ac8dd63d7046985f6d8125241d3da81d1",
			expectedVersion: "1.0.0-02846e1ac8dd63d7046985f6d8125241d3da81d1",
		},
		{
			name:            "case 8: untagged branch with single tagged parent",
			inputRef:        "branch-of-1.0.0",
			expectedVersion: "1.0.0-54b81b007b74747954ba5bd5e8e75a5d039a0530",
		},
		{
			name:            "case 9: untagged branch with multiple tagged parents",
			inputRef:        "branch-of-0.1.0",
			expectedVersion: "1.0.0-02846e1ac8dd63d7046985f6d8125241d3da81d1",
		},
	}

	dir := "/tmp/gitrepo-test"
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

			version, err := repo.ResolveVersion(ctx, tc.inputRef)
			if err != nil {
				t.Errorf("%#v", err)
			}
			if version != tc.expectedVersion {
				t.Errorf("got %q, expected %q\n", version, tc.expectedVersion)
			}
		})
	}
}
