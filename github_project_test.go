package blathers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectForTeams(t *testing.T) {
	ctx := context.Background()
	ghClient := srv.getGithubClientFromInstallation(
		ctx,
		installationID(7754752),
	)
	// Ensure all teams point to a valid board.
	for k := range teamToBoards {
		t.Run(k, func(t *testing.T) {
			ret, err := findProjectsForTeams(ctx, ghClient, []string{k})
			require.NoError(t, err)
			require.Greater(t, len(ret), 0)
		})
	}
}
