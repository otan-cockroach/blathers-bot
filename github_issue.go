package blathers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v30/github"
)

// githubIssueCommentBuilder handles building a GitHub issue comment.
type githubIssueCommentBuilder struct {
	paragraphs []string

	owner  string
	repo   string
	number int
}

func (icb *githubIssueCommentBuilder) addParagraph(paragraph string) *githubIssueCommentBuilder {
	icb.paragraphs = append(icb.paragraphs, paragraph)
	return icb
}

func (icb *githubIssueCommentBuilder) addParagraphf(
	paragraph string, args ...interface{},
) *githubIssueCommentBuilder {
	icb.paragraphs = append(icb.paragraphs, fmt.Sprintf(paragraph, args...))
	return icb
}

func (icb *githubIssueCommentBuilder) finish(ctx context.Context, ghClient *github.Client) error {
	if len(icb.paragraphs) == 0 {
		return nil
	}
	icb.paragraphs = append(
		icb.paragraphs,
		"<sub>:owl: Hoot! I am a [Blathers](https://github.com/apps/blathers-crl), a bot for [CockroachDB](https://github.com/cockroachdb). I am experimental - my owner is [otan](https://github.com/otan).</sub>",
	)
	body := strings.Join(icb.paragraphs, "\n\n")
	_, _, err := ghClient.Issues.CreateComment(
		ctx,
		icb.owner,
		icb.repo,
		icb.number,
		&github.IssueComment{Body: &body},
	)
	return err
}

// findParticipants finds all participants belonging on the owner
// on a given issue.
// It returns a map of username -> participant text.
// Prioritized as "author" > "assigned" > "commented in issue".
func findParticipants(
	ctx context.Context, ghClient *github.Client, owner string, repo string, issueNum int,
) (map[string]string, error) {
	participants := make(map[string]string)
	addParticipant := func(author, reason string) {
		if _, ok := participants[author]; !ok {
			participants[author] = reason
		}
	}

	// Find author and assigned members of the issue.
	issue, _, err := ghClient.Issues.Get(ctx, owner, repo, issueNum)
	if err != nil {
		// Issue does not exist. We should not error here.
		if err, ok := err.(*github.ErrorResponse); ok && err.Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, wrapf(ctx, err, "error getting participant issue")
	}
	issueRef := fmt.Sprintf("%s/%s#%d", owner, repo, issueNum)
	addParticipant(issue.GetUser().GetLogin(), fmt.Sprintf("author of %s", issueRef))
	for _, assigned := range issue.Assignees {
		addParticipant(assigned.GetLogin(), fmt.Sprintf("assigned to %s", issueRef))
	}

	// Now find anyone who's commented
	opts := &github.IssueListCommentsOptions{}
	more := true
	for more {
		comments, resp, err := ghClient.Issues.ListComments(
			ctx,
			owner,
			repo,
			issueNum,
			opts,
		)
		if err != nil {
			return nil, wrapf(ctx, err, "error getting listing issue comments for findParticipants")
		}

		for _, comment := range comments {
			addParticipant(comment.GetUser().GetLogin(), fmt.Sprintf("commented on %s", issueRef))
		}
		more = resp.NextPage != 0
		if more {
			opts.Page = resp.NextPage
		}
	}

	return participants, nil
}
