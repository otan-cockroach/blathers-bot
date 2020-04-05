package blathers

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// isMember returns whether a member is part of the given organization.
func isMember(
	ctx context.Context, ghClient *github.Client, org string, login string,
) (bool, error) {
	isMember, _, err := ghClient.Organizations.IsMember(ctx, org, login)
	if err != nil {
		return false, fmt.Errorf("failed getting organization member status: %s", err.Error())
	}
	return isMember, err
}

func getOrganizationLogins(
	ctx context.Context, ghClient *github.Client, org string,
) (map[string]struct{}, error) {
	logins := make(map[string]struct{})
	members, _, err := ghClient.Organizations.ListMembers(
		ctx,
		org,
		&github.ListMembersOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("error listing org members: %s", err.Error())
	}
	for _, member := range members {
		logins[member.GetLogin()] = struct{}{}
	}
	return logins, nil
}
