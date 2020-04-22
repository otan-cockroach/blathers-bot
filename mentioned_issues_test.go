package blathers

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindMentionedIssues(t *testing.T) {
	testCases := []struct {
		owner    string
		repo     string
		text     string
		expected []mentionedIssue
	}{
		{
			owner: "cockroachdb",
			repo:  "cockroach",
			text:  `Hello! I have mentioned issue #12345, cockroachdb/asdf#1234, ignoreme/abc#4555 as well as issue github.com/cockroachdb/blobby/issues/34567, ignore github.com/notcockroachdb/somethingelse/4444 and PR https://github.com/cockroachdb/lobby/pull/118999 and duplicate https://github.com/cockroachdb/cockroach/pull/12345.`,
			expected: []mentionedIssue{
				{"cockroachdb", "cockroach", 12345},
				{"cockroachdb", "blobby", 34567},
				{"cockroachdb", "lobby", 118999},
				{"cockroachdb", "asdf", 1234},
			},
		},
		{
			owner: "cockroachdb",
			repo:  "cockroach",
			text:  "```ignore me #1``` accept me #2 ```ignore me #3```",
			expected: []mentionedIssue{
				{"cockroachdb", "cockroach", 2},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.text, func(t *testing.T) {
			res := findMentionedIssues(tc.owner, tc.repo, tc.text)
			sort.Slice(res, func(i, j int) bool {
				if res[i].owner == res[j].owner {
					if res[i].repo == res[j].repo {
						return res[i].number < res[j].number
					}
					return res[i].repo < res[j].repo
				}
				return res[i].owner < res[j].owner
			})
			sort.Slice(tc.expected, func(i, j int) bool {
				if tc.expected[i].owner == tc.expected[j].owner {
					if tc.expected[i].repo == tc.expected[j].repo {
						return tc.expected[i].number < tc.expected[j].number
					}
					return tc.expected[i].repo < tc.expected[j].repo
				}
				return tc.expected[i].owner < tc.expected[j].owner
			})
			require.Equal(t, tc.expected, res)
		})
	}
}
