package blathers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v32/github"
)

func (srv *blathersServer) handleBackports(
	ctx context.Context, ghClient *github.Client, owner, repo string,
	pr *github.PullRequest, backportBranches []string) error {

	builder := &githubPullRequestIssueCommentBuilder{
		reviewers: make(map[string]struct{}),
		githubIssueCommentBuilder: githubIssueCommentBuilder{
			labels: map[string]struct{}{},
			owner:  owner,
			repo:   repo,
			number: pr.GetNumber(),
		},
	}
	builder.addParagraph(`Encountered an error creating backports. Some common things that can go wrong:
1. The backport branch might have already existed.
2. There was a merge conflict.
3. The backport branch contained merge commits.

You might need to create your backport manually using the [backport](https://github.com/benesch/backport) tool.

----`)

	foundErr := false
	for _, branch := range backportBranches {
		if err := srv.handleBackport(ctx, ghClient, builder, owner, repo, pr, branch); err != nil {
			builder.addParagraphf(`Backport to branch %s failed. See errors above.

----`, branch)
			foundErr = true
			writeLogf(ctx, "error handling backport: %s", err.Error())
		}
	}
	if foundErr {
		return builder.finish(ctx, ghClient)
	}
	return nil
}

func (srv *blathersServer) handleBackport(ctx context.Context, ghClient *github.Client,
	builder *githubPullRequestIssueCommentBuilder, owner string, repo string, originalPR *github.PullRequest,
	branchName string) error {
	// The CockroachDB backport label schema looks like: backport-21.1.x.
	// But the CockroachDB release branch schema looks like release-21.1.
	// Strip the .x suffix, and below we'll try with the release- prefix.
	if strings.HasSuffix(branchName, ".x") {
		branchName = branchName[:len(branchName)-2]
	}

	// Backport algorithm from https://stackoverflow.com/questions/53859199/how-to-cherry-pick-through-githubs-api

	targetBranch, _, err := ghClient.Repositories.GetBranch(ctx, owner, repo, branchName)
	if err != nil {
		// Try release-foo.
		branchName = "release-" + branchName
		targetBranch, _, err = ghClient.Repositories.GetBranch(ctx, owner, repo, branchName)
		if err != nil {
			builder.addParagraphf("error getting backport branch %s: %s", branchName, err.Error())
			return err
		}
	}

	commits, _, err := ghClient.PullRequests.ListCommits(ctx, owner, repo,
		originalPR.GetNumber(), &github.ListOptions{})
	if err != nil {
		builder.addParagraphf("error getting PR %d commits: %s", originalPR.GetNumber(), err.Error())
		return err
	}

	newBranchName := fmt.Sprintf("blathers/backport-%s-%d", targetBranch.GetName(), originalPR.GetNumber())
	refName := fmt.Sprintf("refs/heads/%s", newBranchName)

	// Create the backport branch. Point it at the target branch to start with.
	_, _, err = ghClient.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: targetBranch.GetCommit().SHA,
		},
	})
	if err != nil {
		builder.addParagraphf("error creating backport branch %s: %s", refName, err.Error())
		return err
	}
	backportBranchSHA := targetBranch.GetCommit().SHA
	for _, commit := range commits {
		if len(commit.Parents) > 1 {
			builder.addParagraph("can't backport merge commits")
			return err
		}
		parent := commit.Parents[0]

		// Create a temporary commit whose parent is the parent of the commit
		// to cherry-pick.
		// But, set the *contents* of the commit to be the repository as it
		// looks in the target branch.
		tmpCommit, _, err := ghClient.Git.CreateCommit(ctx, owner, repo, &github.Commit{
			Message: github.String("tmp"),
			Tree: &github.Tree{
				SHA: targetBranch.GetCommit().GetCommit().GetTree().SHA,
			},
			Parents: []*github.Commit{parent},
		})
		if err != nil {
			builder.addParagraphf("error creating temp commit with parent %s: %s", *parent.SHA, err.Error())
			return err
		}

		// Set the backport branch to point at the temporary commit.
		_, _, err = ghClient.Git.UpdateRef(ctx, owner, repo, &github.Reference{
			Ref: github.String(refName),
			Object: &github.GitObject{
				SHA: tmpCommit.SHA,
			},
		}, true /* force */)
		if err != nil {
			builder.addParagraphf("error updating backport branch to sha %s: %s", *tmpCommit.SHA, err.Error())
			return err
		}

		// Merge the commit we want into the backport branch, just to get the
		// resultant tree.
		merge, _, err := ghClient.Repositories.Merge(ctx, owner, repo, &github.RepositoryMergeRequest{
			Base: github.String(newBranchName),
			Head: commit.SHA,
		})
		if err != nil {
			builder.addParagraphf("error creating merge commit from %s to %s: %s", *commit.SHA, newBranchName, err.Error())
			builder.addParagraph("you may need to manually resolve merge conflicts with the backport tool.")
			return err
		}

		// Now that we know what the tree should be, create the cherry-pick commit.
		// Note that branchSha is the original from up at the top.
		cherryPick, _, err := ghClient.Git.CreateCommit(ctx, owner, repo, &github.Commit{
			Author:  commit.GetCommit().GetAuthor(),
			Message: commit.Commit.Message,
			Tree:    &github.Tree{SHA: merge.Commit.Tree.SHA},
			Parents: []*github.Commit{{
				SHA: backportBranchSHA,
			}},
		})
		if err != nil {
			builder.addParagraphf("error creating final cherrypick: %s", err.Error())
			return err
		}
		backportBranchSHA = github.String(*cherryPick.SHA)

		// Replace the temp commit with the real commit.
		_, _, err = ghClient.Git.UpdateRef(ctx, owner, repo, &github.Reference{
			Ref: github.String(refName),
			Object: &github.GitObject{
				SHA: cherryPick.SHA,
			},
		}, true /* force */)
		if err != nil {
			builder.addParagraphf("error updating temp commit to sha %s: %s", *cherryPick.SHA, err.Error())
			return err
		}
	}

	// Create the pull request.
	pr, _, err := ghClient.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title:               github.String(fmt.Sprintf("%s: %s", branchName, originalPR.GetTitle())),
		Base:                github.String(branchName),
		Head:                github.String(newBranchName),
		MaintainerCanModify: github.Bool(true),
		Body: github.String(fmt.Sprintf(`Backport %d/%d commits from #%d on behalf of @%s.

/cc @cockroachdb/release

----

%s

----`, len(commits), len(commits), originalPR.GetNumber(), originalPR.GetUser().GetLogin(), originalPR.GetBody())),
	})
	if err != nil {
		builder.addParagraphf("error creating PR, but backport branch %s is ready: %s", newBranchName, err.Error())
		return err
	}

	// Assign the original author to the backport PR, and request the original
	// reviewers.

	requestedReviewers := originalPR.RequestedReviewers
	reviewers := make([]string, len(requestedReviewers))
	for i := range reviewers {
		reviewers[i] = requestedReviewers[i].GetLogin()
	}
	requestedTeams := originalPR.RequestedTeams
	teamReviewers := make([]string, len(requestedTeams))
	for i := range teamReviewers {
		teamReviewers[i] = requestedTeams[i].GetName()
	}

	if len(reviewers) > 0 || len(teamReviewers) > 0 {
		if _, _, err = ghClient.PullRequests.RequestReviewers(ctx, owner, repo, pr.GetNumber(), github.ReviewersRequest{
			Reviewers:     reviewers,
			TeamReviewers: teamReviewers,
		}); err != nil {
			return err
		}
	}

	if _, _, err := ghClient.Issues.AddAssignees(ctx, owner, repo, pr.GetNumber(),
		[]string{originalPR.GetUser().GetLogin()}); err != nil {
		return err
	}

	return nil

}
