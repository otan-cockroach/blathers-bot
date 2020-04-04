package blathers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
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
		"<sub>Hoot! I am a [Blathers](https://github.com/apps/blathers-crl), a bot for [CockroachDB](https://github.com/cockroachdb). I am experimental - my owner is [otan](https://github.com/otan).</sub>",
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
