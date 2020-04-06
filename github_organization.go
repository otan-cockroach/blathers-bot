package blathers

import (
	"context"

	"github.com/google/go-github/v30/github"
)

// isOrgMember returns whether a member is part of the given organization.
func isOrgMember(
	ctx context.Context, ghClient *github.Client, org string, login string,
) (bool, error) {
	isMember, _, err := ghClient.Organizations.IsMember(ctx, org, login)
	if err != nil {
		return false, wrapf(ctx, err, "failed getting organization member status")
	}
	return isMember, err
}

// TODO: cache this.
func getOrganizationLogins(
	ctx context.Context, ghClient *github.Client, org string,
) (map[string]struct{}, error) {
	logins := make(map[string]struct{})
	opts := &github.ListMembersOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	more := true
	for more {
		members, resp, err := ghClient.Organizations.ListMembers(
			ctx,
			org,
			opts,
		)
		if err != nil {
			return nil, wrapf(ctx, err, "error listing org members")
		}
		for _, member := range members {
			logins[member.GetLogin()] = struct{}{}
		}
		more = resp.NextPage != 0
		if more {
			opts.Page = resp.NextPage
		}
	}
	return logins, nil
}
