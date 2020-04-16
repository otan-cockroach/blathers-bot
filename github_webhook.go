package blathers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v30/github"
)

// blacklistedLogins contains a list of organization members to
// ignore in certain situations.
var blacklistedLogins = map[string]struct{}{
	"cockroach-teamcity":  struct{}{},
	"cockroach-oncall":    struct{}{},
	"cockroach-roachdash": struct{}{},
	"crl-monitor-roach":   struct{}{},
}

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
	ctx := WithRequestID(context.Background(), r.Header.Get("X-Github-Delivery"))
	t := time.Now()
	defer func() {
		writeLogf(ctx, "time: %s", time.Now().Sub(t))
	}()

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
		writeLogf(ctx, "validate error: %s", err.Error())
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err.Error())
		writeLogf(ctx, "parse error: %s", err.Error())
		return
	}
	switch event := event.(type) {
	case *github.IssuesEvent:
		if event.Installation == nil {
			w.WriteHeader(400)
			writeLogf(ctx, "no installation")
			return
		}
		err = srv.handleIssuesWebhook(ctx, event)
	case *github.PullRequestEvent:
		if event.Installation == nil {
			w.WriteHeader(400)
			writeLogf(ctx, "no installation")
			return
		}
		err = srv.handlePullRequestWebhook(ctx, event)
	case *github.StatusEvent:
		if event.Installation == nil {
			w.WriteHeader(400)
			writeLogf(ctx, "no installation")
			return
		}
		err = srv.handleStatusWebhook(ctx, event)
	case *github.PingEvent:
		fmt.Fprintf(w, "ok")
	}
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err.Error())
		writeLogf(ctx, "[%s] error: %s", r.Header.Get("X-GitHub-Delivery"), err.Error())
		return
	}
	w.WriteHeader(200)
}

// handleStatusWebhook handles the status component of a webhook.
func (srv *blathersServer) handleStatusWebhook(
	ctx context.Context, event *github.StatusEvent,
) error {
	ctx = WithDebuggingPrefix(ctx, fmt.Sprintf("[Status][%s]", event.GetSHA()))
	writeLogf(ctx, "handling status update (%s, %s)", event.GetContext(), event.GetState())
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
				return wrapf(ctx, err, "error fetching pull requests using ListPRsWithCommit")
			}

			for _, pr := range prs {
				if pr.GetState() != "open" {
					continue
				}
				if pr.GetHead().GetSHA() != event.GetSHA() {
					writeLogf(ctx, "aborting - PR no longer has head - new head is %s", pr.GetHead().GetSHA())
					return nil
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
			writeLogf(ctx, "sha %s: unable to find using ListPRsWithCommit, using fallback", event.GetSHA())
			opts = &github.PullRequestListOptions{
				State:     "open",
				Sort:      "updated",
				Direction: "desc",
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			}
			// Only process one page.
			prs, _, err := ghClient.PullRequests.List(
				ctx,
				event.GetRepo().GetOwner().GetLogin(),
				event.GetRepo().GetName(),
				opts,
			)
			if err != nil {
				return wrapf(ctx, err, "error fetching pull requests using List")
			}

			for _, pr := range prs {
				if pr.GetHead().GetSHA() == event.GetSHA() {
					numbers = append(numbers, pr.GetNumber())
					// Only take the first one for now.
					break
				}
			}
		}

		writeLogf(ctx, "sha %s: found PRs %#v", event.GetSHA(), numbers)
		for _, number := range numbers {
			// Build the message to send.
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
				return wrapf(ctx, err, "#%d: failed to finish building issue comment", number)
			}
			writeLogf(ctx, "#%d: status updated", number)
		}
		writeLogf(ctx, "complete")
		return nil
	},
}

// handleIssuesWebhook handles the pull request component
// of a webhook.
func (srv *blathersServer) handleIssuesWebhook(
	ctx context.Context, event *github.IssuesEvent,
) error {
	ctx = WithDebuggingPrefix(ctx, fmt.Sprintf("[Webhook][Issue #%d]", event.Issue.GetNumber()))
	writeLogf(ctx, "handling issues action: %s", event.GetAction())

	switch event.GetAction() {
	case "opened":
	default:
		writeLogf(ctx, "not an event we care about")
		return nil
	}

	if event.Issue.PullRequestLinks != nil {
		writeLogf(ctx, "ignoring pull requests")
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

	if isMember {
		writeLogf(ctx, "skipping as member is part of organization")
		return nil
	}

	builder := githubIssueCommentBuilder{
		labels: map[string]struct{}{},
		owner:  event.GetRepo().GetOwner().GetLogin(),
		repo:   event.GetRepo().GetName(),
		number: event.Issue.GetNumber(),
		id:     event.GetIssue().GetID(),
	}
	builder.addLabel("O-community")

	body := event.GetIssue().GetBody()
	builder.addParagraph("Hello, I am Blathers. I am here to help you get the issue triaged.")
	if strings.Contains(body, "Describe the problem") {
		builder.addParagraph("Hoot - a bug! Though bugs are the bane of my existence, rest assured the wretched thing will get the best of care here.")
		builder.addLabel("C-bug")
	} else if strings.Contains(body, "Is your feature request related to a problem? Please describe.") {
		builder.addLabel("C-enhancement")
	} else if strings.Contains(body, "What is your situation?") {
		builder.addLabel("C-investigation")
	} else {
		builder.addParagraph("It looks like you have not filled out the issue in the format of any of our templates. To best assist you, we advise you to use one of these [templates](https://github.com/cockroachdb/cockroach/tree/master/.github/ISSUE_TEMPLATE).")
	}

	participantToReasons, err := findRelevantUsersFromAttachedIssues(
		ctx,
		ghClient,
		event.GetRepo().GetOwner().GetLogin(),
		event.GetRepo().GetName(),
		event.Issue.GetNumber(),
		event.GetIssue().GetBody(),
		event.GetSender().GetLogin(),
	)
	if err != nil {
		return wrapf(ctx, err, "failed to find relevant users")
	}

	// If we haven't found anything by issues, fallback to trying to use arbitrary keywords.
	if len(participantToReasons) == 0 {
		writeLogf(ctx, "failed to find any related issues; trying keywords")
		teamsToKeywords := findTeamsFromKeywords(event.GetIssue().GetBody())
		if len(teamsToKeywords) > 0 {
			var teams []string
			for team := range teamsToKeywords {
				teams = append(teams, team)
			}
			teamsToProjects, err := findProjectsForTeams(ctx, ghClient, teams)
			if err != nil {
				return wrapf(ctx, err, "error finding relevant projects")
			}
			for team, keywords := range teamsToKeywords {
				// TODO: add projects.
				if projectID, ok := teamsToProjects[team]; ok {
					builder.addProject(projectID)
				}
				for _, owner := range teamToContacts[team] {
					participantToReasons[owner] = append(
						participantToReasons[owner],
						fmt.Sprintf("found keywords: %s", strings.Join(keywords, ",")),
					)
				}
			}
		}
	}

	if len(participantToReasons) == 0 {
		// TODO(otan): proper fallback to an oncall rotation.
		builder.addLabel("X-blathers-untriaged")
		builder.addParagraph(`I was unable to automatically find someone to ping. We will get back to you soon. However, if we have not gotten back to your issue within a few business days, you can try the following:
* Join our [community slack channel](https://cockroa.ch/slack) and ask on #cockroachdb.
* Try find someone from [here](https://github.com/orgs/cockroachdb/people) if you know they worked closely on the area and cc them.`)
	} else {
		builder.addLabel("X-blathers-triaged")
		var assignedReasons listBuilder
		for author, reasons := range participantToReasons {
			assignedReasons = assignedReasons.addf("@%s (%s)", author, strings.Join(reasons, ", "))
			// builder.addAssignee(author)
		}
		builder.addParagraphf("I have CC'd a few people who may be able to assist you:\n%s", assignedReasons.String())
	}

	builder.setMustComment(true)
	return builder.finish(ctx, ghClient)
}

// handlePullRequestWebhook handles the pull request component
// of a webhook.
func (srv *blathersServer) handlePullRequestWebhook(
	ctx context.Context, event *github.PullRequestEvent,
) error {
	ctx = WithDebuggingPrefix(ctx, fmt.Sprintf("[Webhook][PR #%d]", event.GetNumber()))
	writeLogf(ctx, "handling pull request action: %s", event.GetAction())

	// We only care about requests being opened, or new PR updates.
	switch event.GetAction() {
	case "opened", "synchronize":
	default:
		writeLogf(ctx, "not an event we care about")
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

	if isMember {
		writeLogf(ctx, "skipping as member is part of organization")
		return nil
	}

	if _, isBlacklistedLogin := blacklistedLogins[event.GetSender().GetLogin()]; isBlacklistedLogin {
		writeLogf(ctx, "skipping as member %s is blacklisted", event.GetSender().GetLogin())
		return nil
	}

	builder := githubPullRequestIssueCommentBuilder{
		reviewers: make(map[string]struct{}),
		githubIssueCommentBuilder: githubIssueCommentBuilder{
			labels: map[string]struct{}{},
			owner:  event.GetRepo().GetOwner().GetLogin(),
			repo:   event.GetRepo().GetName(),
			number: event.GetNumber(),
		},
	}

	// Send guidelines.
	if event.GetSender().GetLogin() == "otan" {
		builder.addParagraph("Welcome back, creator. Thank you for testing me.")
	} else {
		switch event.GetAction() {
		case "opened":
			builder.setMustComment(true)
			if event.GetPullRequest().GetAuthorAssociation() == "FIRST_TIME_CONTRIBUTOR" {
				builder.addParagraph("Thank you for contributing your first PR! Please ensure you have read the instructions for [creating your first PR](https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).")
			} else {
				builder.addParagraph("Thank you for contributing to CockroachDB. Please ensure you have followed the guidelines for [creating a PR](https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).")
			}
		case "synchronize":
			builder.addParagraph("Thank you for updating your pull request.")
		}
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
		builder.setMustComment(true)
		ais = ais.add("When CI has completed, please ensure no errors have appeared.")
		builder.addParagraphf("Before a member of our team reviews your PR, I have a few suggestions for tidying it up for review:\n%s", ais.String())
	}

	if len(event.GetPullRequest().RequestedReviewers) == 0 && len(event.GetPullRequest().RequestedTeams) == 0 {
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
			participantToReasons, err := findRelevantUsersFromAttachedIssues(
				ctx,
				ghClient,
				event.GetRepo().GetOwner().GetLogin(),
				event.GetRepo().GetName(),
				event.GetNumber(),
				event.GetPullRequest().GetBody(),
				event.GetSender().GetLogin(),
			)
			if err != nil {
				return wrapf(ctx, err, "failed to find relevant users")
			}
			builder.setMustComment(true)
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
		return wrapf(ctx, err, "failed to finish building issue comment")
	}
	writeLogf(ctx, "completed all checks")
	return nil
}
