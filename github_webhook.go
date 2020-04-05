package blathers

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
)

// listBuilder keeps track of action items needed to be done.
// This will be output as a GitHub list.
type listBuilder []string

func (lb listBuilder) String() string {
	return strings.Join([]string(lb), "\n")
}

func (lb listBuilder) add(item string) listBuilder {
	lb = append(lb, fmt.Sprintf("* %s", item))
	return lb
}

func (lb listBuilder) addf(item string, fmts ...interface{}) listBuilder {
	lb = append(lb, fmt.Sprintf("* %s", fmt.Sprintf(item, fmts...)))
	return lb
}

// HandleGithubWebhook handles a Github based webhook.
func (srv *blathersServer) HandleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var payload []byte
	var err error
	// Validate the secret if one is provided.
	if srv.githubAppSecret == "" {
		payload, err = ioutil.ReadAll(r.Body)
	} else {
		payload, err = github.ValidatePayload(r, []byte(srv.githubAppSecret))
	}
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprint(w, err.Error())
		log.Printf("error: %s", err.Error())
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprint(w, err.Error())
		log.Printf("error: %s", err.Error())
		return
	}
	switch event := event.(type) {
	case *github.PullRequestEvent:
		err := srv.handlePullRequestWebhook(ctx, event)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprint(w, err.Error())
			log.Printf("error: %s", err.Error())
			return
		}
	case *github.PingEvent:
		fmt.Fprintf(w, "ok")
	}
	w.WriteHeader(200)
}

// handlePullRequestWebhook handles the pull request component
// of a webhook.
func (srv *blathersServer) handlePullRequestWebhook(
	ctx context.Context, event *github.PullRequestEvent,
) error {
	log.Printf("[Webhook][#%d] handling pull request action: %s", event.GetNumber(), event.GetAction())
	if event.Installation == nil {
		return fmt.Errorf("#%d request does not include installation id: %#v", event.GetNumber(), event)
	}

	// We only care about requests being opened, or new PR updates.
	switch event.GetAction() {
	case "opened", "synchronize":
	default:
		log.Printf("[Webhook][#%d] not an event we care about", event.GetNumber())
		return nil
	}

	ghClient := srv.getGithubClientFromInstallation(
		ctx,
		installationID(event.Installation.GetID()),
	)

	isMember, err := isMember(
		ctx,
		ghClient,
		event.GetRepo().GetOwner().GetLogin(),
		event.GetSender().GetLogin(),
	)
	if err != nil {
		return err
	}

	if isMember && event.GetSender().GetLogin() != "otan" {
		log.Printf("[Webhook][#%d] skipping as member is part of organization", event.GetNumber())
		return nil
	}

	builder := githubPullRequestIssueCommentBuilder{
		reviewers: make(map[string]struct{}),
		githubIssueCommentBuilder: githubIssueCommentBuilder{
			owner:  event.GetRepo().GetOwner().GetLogin(),
			repo:   event.GetRepo().GetName(),
			number: event.GetNumber(),
		},
	}

	// Send guidelines.
	switch event.GetAction() {
	case "opened":
		if event.GetSender().GetLogin() == "otan" {
			builder.addParagraph("Welcome back, creator. Thank you for testing me.")
		} else if event.GetPullRequest().GetAuthorAssociation() == "FIRST_TIME_CONTRIBUTOR" {
			builder.addParagraph("Thank you for contributing your first PR! Please ensure you have read the instructions for [creating your first PR](https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).")
		} else {
			builder.addParagraph("Thank you for contributing to CockroachDB. Please ensure you have followed the guidelines for [creating a PR](https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).")
		}
	case "synchronize":
		builder.addParagraph("Thank you for updating your pull request.")
	}

	// Build a list of action items that we can easily scan for.
	var ais listBuilder

	commits, err := listCommitsInPR(
		ctx,
		ghClient,
		event.GetRepo().GetOwner().GetLogin(),
		event.GetRepo().GetName(),
		event.GetNumber(),
	)
	if err != nil {
		return err
	}
	if len(commits) > 1 {
		ais = ais.add("We generally try and keep pull requests to one commit. [Please squash your commits](https://github.com/wprig/wprig/wiki/How-to-squash-commits), and re-push with `--force`.")
	}
	for _, commit := range commits {
		if !strings.Contains(commit.GetCommit().GetMessage(), "Release note") {
			ais = ais.add("Please ensure your git commit message contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes).")
			break
		}
	}

	if !strings.Contains(event.GetPullRequest().GetBody(), "Release note") {
		ais = ais.add("Please ensure your pull request description contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes) - this can be the same as the one in your commit message.")
	}

	if len(ais) == 0 {
		builder.addParagraph("My owl senses detect your PR is good for review. Please keep an eye out for any test failures in CI.")
	} else {
		ais = ais.add("When CI has completed, please ensure no errors have appeared.")
		builder.addParagraphf("Before we review your PR, we have a few suggestions for tidying it up for review:\n%s", ais.String())
	}

	// TODO(otan): scan for adding reviewers.
	if len(event.GetPullRequest().RequestedReviewers) == 0 {
		mentionedIssues := findMentionedIssues(
			event.GetRepo().GetOwner().GetLogin(),
			event.GetRepo().GetName(),
			event.GetPullRequest().GetBody(),
		)
		participantToReasons := make(map[string][]string)
		for _, iss := range mentionedIssues {
			participantToReason, err := findParticipants(
				ctx,
				ghClient,
				event.GetRepo().GetOwner().GetLogin(),
				event.GetRepo().GetName(),
				iss.number,
			)
			if err != nil {
				return err
			}
			for participant, reason := range participantToReason {
				participantToReasons[participant] = append(participantToReasons[participant], reason)
			}
		}

		// Filter out anyone not in the organization.
		// TODO(otan): batch this by listing organization members instead.
		orgMembers, err := getOrganizationLogins(ctx, ghClient, event.GetRepo().GetOwner().GetLogin())
		if err != nil {
			return err
		}
		for author := range participantToReasons {
			if _, ok := orgMembers[author]; !ok || author == event.GetSender().GetName() {
				delete(participantToReasons, author)
			}
		}

		if len(participantToReasons) == 0 {
			builder.addParagraph(`We were unable to automatically find a reviewer. You can try CCing one of the following members:
* A person you worked with closely on this PR.
* The person who created the ticket, or a [CRDB organization member](https://github.com/orgs/cockroachdb/people) involved with the ticket (author, commenter, etc.).
* Join our [community slack channel](https://cockroa.ch/slack) and ask on #contributors.
* Try find someone else from [here](https://github.com/orgs/cockroachdb/people).`)
		} else {
			var reviewerReasons listBuilder
			for author, reasons := range participantToReasons {
				reviewerReasons = reviewerReasons.addf("@%s (%s)", author, strings.Join(reasons, ", "))
				builder.addReviewer(author)
			}
			builder.addParagraphf("We have added a few people who may be able to assist in reviewing:\n%s", reviewerReasons.String())
		}
	}

	// We've compiled everything we want to happen. Send the message.
	if err := builder.finish(ctx, ghClient); err != nil {
		return fmt.Errorf("#%d: failed to finish building issue comment: %v", event.GetNumber(), err)
	}
	log.Printf("[Webhook][#%d] completed", event.GetNumber())
	return nil
}
