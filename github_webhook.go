package blathers

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v30/github"
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
		log.Printf("[%s] error: %s", r.Header.Get("X-GitHub-Delivery"), err.Error())
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err.Error())
		log.Printf("[%s] error: %s", r.Header.Get("X-GitHub-Delivery"), err.Error())
		return
	}
	switch event := event.(type) {
	case *github.PullRequestEvent:
		err = srv.handlePullRequestWebhook(ctx, event)
	case *github.StatusEvent:
		err = srv.handleStatus(ctx, event)
	case *github.PingEvent:
		fmt.Fprintf(w, "ok")
	}
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err.Error())
		log.Printf("[%s] error: %s", r.Header.Get("X-GitHub-Delivery"), err.Error())
		return
	}
	w.WriteHeader(200)
}

// handleStatus handles the status component of a webhook.
func (srv *blathersServer) handleStatus(ctx context.Context, event *github.StatusEvent) error {
	log.Printf("[Webhook] handling status update (%s, %s)", event.GetContext(), event.GetState())
	handler, ok := statusHandlers[handlerKey{context: event.GetContext(), state: event.GetState()}]
	if !ok {
		return nil
	}
	return handler(ctx, srv, event)
}

type handlerKey struct {
	context string
	state   string
}

var statusHandlers = map[handlerKey]func(ctx context.Context, srv *blathersServer, event *github.StatusEvent) error{
	{"GitHub CI (Cockroach)", "failure"}: func(ctx context.Context, srv *blathersServer, event *github.StatusEvent) error {
		ghClient := srv.getGithubClientFromInstallation(
			ctx,
			installationID(event.Installation.GetID()),
		)

		// So we have a commit SHA, get the PRs.
		opts := &github.PullRequestListOptions{}
		more := true
		numbers := []int{}
		for more && len(numbers) == 0 {
			prs, resp, err := ghClient.PullRequests.ListPullRequestsWithCommit(
				ctx,
				event.GetRepo().GetOwner().GetLogin(),
				event.GetRepo().GetName(),
				event.GetSHA(),
				opts,
			)
			if err != nil {
				return fmt.Errorf("sha %s: error fetching pull requests using ListPRsWithCommit", event.GetSHA())
			}

			for _, pr := range prs {
				if pr.GetState() != "open" {
					continue
				}
				number := pr.GetNumber()
				numbers = append(numbers, number)
				// Only take the first one for now.
				break
			}

			more = resp.NextPage != 0
			if more {
				opts.Page = resp.NextPage
			}
		}

		// The previous API is experimental. Let's find the real one using a hackier way.
		// But this is also experimental....
		if len(numbers) == 0 {
			log.Printf("sha %s: unable to find using ListPRsWithCommit, using fallback", event.GetSHA())
			more = true
			opts = &github.PullRequestListOptions{
				State: "open",
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			}
			for more && len(numbers) == 0 {
				prs, resp, err := ghClient.PullRequests.List(
					ctx,
					event.GetRepo().GetOwner().GetLogin(),
					event.GetRepo().GetName(),
					opts,
				)
				if err != nil {
					return fmt.Errorf("sha %s: error fetching pull requests using List", event.GetSHA())
				}

				for _, pr := range prs {
					if pr.GetHead().GetSHA() == event.GetSHA() {
						numbers = append(numbers, pr.GetNumber())
						// Only take the first one for now.
						break
					}
				}

				more = resp.NextPage != 0
				if more {
					opts.Page = resp.NextPage
				}
			}
		}

		log.Printf("sha %s: found PRs %#v", event.GetSHA(), numbers)
		for _, number := range numbers {
			builder := githubPullRequestIssueCommentBuilder{
				reviewers: make(map[string]struct{}),
				githubIssueCommentBuilder: githubIssueCommentBuilder{
					owner:  event.GetRepo().GetOwner().GetLogin(),
					repo:   event.GetRepo().GetName(),
					number: number,
				},
			}

			builder.addParagraphf(
				":x: The [%s build](%s) has failed on [%s](%s).",
				event.GetContext(),
				event.GetTargetURL(),
				event.GetSHA()[:8],
				event.GetCommit().GetHTMLURL(),
			)

			if err := builder.finish(ctx, ghClient); err != nil {
				return fmt.Errorf("#%d: failed to finish building issue comment: %v", number, err)
			}
			log.Printf("[Webhook][#%d] status updated", number)
		}

		return nil
	},
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

	isMember, err := isOrgMember(
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
		builder.addParagraphf("Before a member of our team reviews your PR, I have a few suggestions for tidying it up for review:\n%s", ais.String())
	}

	if len(event.GetPullRequest().RequestedReviewers) == 0 {
		// If there are no requested reviewers, check whether there are any reviews.
		// If there have been, that means someone is already on it.
		if hasReviews, err := hasReviews(
			ctx,
			ghClient,
			event.GetRepo().GetOwner().GetLogin(),
			event.GetRepo().GetName(),
			event.GetNumber(),
		); err != nil {
			return err
		} else if !hasReviews {
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
			orgMembers, err := getOrganizationLogins(ctx, ghClient, event.GetRepo().GetOwner().GetLogin())
			if err != nil {
				return err
			}
			for author := range participantToReasons {
				if _, ok := orgMembers[author]; !ok || author == event.GetSender().GetLogin() {
					delete(participantToReasons, author)
				}
			}

			if len(participantToReasons) == 0 {
				builder.addParagraph(`I was unable to automatically find a reviewer. You can try CCing one of the following members:
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
				builder.addParagraphf("I have added a few people who may be able to assist in reviewing:\n%s", reviewerReasons.String())
			}
		}
	}

	// We've compiled everything we want to happen. Send the message.
	if err := builder.finish(ctx, ghClient); err != nil {
		return fmt.Errorf("#%d: failed to finish building issue comment: %v", event.GetNumber(), err)
	}
	log.Printf("[Webhook][#%d] completed", event.GetNumber())
	return nil
}
