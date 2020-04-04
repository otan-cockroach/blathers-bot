package blathers

import (
	"context"

	"github.com/google/go-github/github"
)

// githubPullRequestIssueCommentBuilder wraps githubIssueCommentBuilder, adding PR-based issue
// only capabilities.
type githubPullRequestIssueCommentBuilder struct {
	githubIssueCommentBuilder
}

func (prb *githubPullRequestIssueCommentBuilder) finish(
	ctx context.Context, ghClient *github.Client,
) error {

	return prb.githubIssueCommentBuilder.finish(ctx, ghClient)
}

// listCommitsInPR lists all commits in a PR.
func listCommitsInPR(
	ctx context.Context, ghClient *github.Client, owner string, repo string, number int,
) ([]*github.RepositoryCommit, error) {
	commits, _, err := ghClient.PullRequests.ListCommits(
		ctx,
		owner,
		repo,
		number,
		&github.ListOptions{},
	)
	return commits, err
}
