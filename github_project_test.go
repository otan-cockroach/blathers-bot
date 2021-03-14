package blathers

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

var flagRunGithubConnectionTest = flag.Bool(
	"run-github-connection-test",
	false,
	"Whether to run tests connecting to Github",
)

func TestProjectForTeams(t *testing.T) {
	flag.Parse()
	if !*flagRunGithubConnectionTest {
		return
	}
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
