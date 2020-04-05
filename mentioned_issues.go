package blathers

import (
	"regexp"
	"strconv"
)

// mentionedIssues is an issue that has been mentioned from a
// body of text.
type mentionedIssue struct {
	owner  string
	repo   string
	number int
}

var (
	// numberberRegex captures #1234 and cockroachdb/cockroach#1234
	numberberRegex = regexp.MustCompile(`(([a-zA-Z_-]+)/([a-zA-Z_-]+))?#(\d+)`)
	// fullIssueURLRegex captures URLs directing to issues.
	fullIssueURLRegex = regexp.MustCompile(`github.com/([a-zA-Z_-]+)/([a-zA-Z-_]+)/(pull|issues)/(\d+)`)
)

// findMentionedIssues returns issues that have been mentioned in
// a body of text that belongs to the given owner.
func findMentionedIssues(owner string, defaultRepo string, text string) []mentionedIssue {
	mentionedIssues := make(map[mentionedIssue]struct{})
	for _, group := range numberberRegex.FindAllStringSubmatch(text, -1) {
		o := owner
		if group[2] != "" {
			o = group[2]
			if o != owner {
				continue
			}
		}
		r := defaultRepo
		if group[3] != "" {
			r = group[3]
		}
		number, err := strconv.ParseInt(group[4], 10, 64)
		if err != nil {
			continue
		}
		mentionedIssues[mentionedIssue{owner: o, repo: r, number: int(number)}] = struct{}{}
	}

	for _, group := range fullIssueURLRegex.FindAllStringSubmatch(text, -1) {
		if group[1] != owner {
			continue
		}
		number, err := strconv.ParseInt(group[4], 10, 64)
		if err != nil {
			continue
		}
		mentionedIssues[mentionedIssue{owner: owner, repo: group[2], number: int(number)}] = struct{}{}
	}

	ret := make([]mentionedIssue, 0, len(mentionedIssues))
	for mi := range mentionedIssues {
		ret = append(ret, mi)
	}
	return ret
}
