package blathers

import (
	"context"
	"fmt"

	"github.com/google/go-github/v30/github"
)

func findProjectsForTeams(
	ctx context.Context, ghClient *github.Client, teams []string,
) (map[string]int64, error) {
	type key struct {
		owner string
		repo  string
	}
	type val struct {
		name string
		team string
	}
	searchBy := map[key][]val{}
	for _, team := range teams {
		board, ok := teamToBoards[team]
		if !ok {
			return nil, fmt.Errorf("cannot find board name for %s", board.name)
		}
		k := key{owner: board.owner, repo: board.repo}
		searchBy[k] = append(searchBy[k], val{name: board.name, team: team})
	}
	ret := map[string]int64{}
	for k, vals := range searchBy {
		opts := &github.ProjectListOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}
		more := true
		for more {
			var r []*github.Project
			var resp *github.Response
			var err error
			if k.repo != "" {
				r, resp, err = ghClient.Repositories.ListProjects(ctx, k.owner, k.repo, opts)
			} else {
				r, resp, err = ghClient.Organizations.ListProjects(ctx, k.owner, opts)
			}
			if err != nil {
				return nil, wrapf(ctx, err, "error finding projects")
			}
			for _, proj := range r {
				// Purposefully n^2.
				for _, val := range vals {
					if proj.GetName() == val.name {
						ret[val.team] = proj.GetID()
						break
					}
				}
			}

			more = resp.NextPage != 0
			if more {
				opts.Page = resp.NextPage
			}
		}
	}
	return ret, nil
}
