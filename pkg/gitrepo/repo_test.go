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
