package blathers

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// githubPullRequestIssueCommentBuilder wraps githubIssueCommentBuilder, adding PR-based issue
// only capabilities.
type githubPullRequestIssueCommentBuilder struct {
	reviewers map[string]struct{}
	githubIssueCommentBuilder
}

func (prb *githubPullRequestIssueCommentBuilder) addReviewer(reviewer string) {
	prb.reviewers[reviewer] = struct{}{}
}

func (prb *githubPullRequestIssueCommentBuilder) finish(
	ctx context.Context, ghClient *github.Client,
) error {
	err := prb.githubIssueCommentBuilder.finish(ctx, ghClient)
	if err != nil {
		return err
	}

	if len(prb.reviewers) > 0 {
		reviewers := make([]string, 0, len(prb.reviewers))
		for reviewer := range prb.reviewers {
			reviewers = append(reviewers, reviewer)
		}

		_, _, err = ghClient.PullRequests.RequestReviewers(
			ctx,
			prb.owner,
			prb.repo,
			prb.number,
			github.ReviewersRequest{
				Reviewers: reviewers,
			},
		)
		if err != nil {
			return fmt.Errorf("error adding reviewers: %s", err.Error())
		}
	}
	return nil
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
