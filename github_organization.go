package blathers

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
)

// isMember returns whether a member is part of the given organization.
// TODO(otan): fetch all members of organization and cache that instead.
func isMember(
	ctx context.Context, ghClient *github.Client, org string, login string,
) (bool, error) {
	isMember, _, err := ghClient.Organizations.IsMember(ctx, org, login)
	if err != nil {
		return false, fmt.Errorf("failed getting organization member status: %v", err)
	}
	return isMember, err
}
