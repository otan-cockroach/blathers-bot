package blathers

import "regexp"

var teamToKeyword = map[string][]string{
	"appdev":        {`(?i)\borm\b`, `ORM`, `(?i)django`, `(?i)hibernate`},
	"bulk-io":       {`(?i)backup`, `(?i)import`, `(?i)export`, `(?i)restore`, `(?i)changefeed`, `(?i)cdc`},
	"sql-schema":    {`(?i)(alter|drop)\s+(table|index|database)`},
	"kv":            {`(?i)\bkv\b`, `(?i)HLC`},
	"sql-features":  {`(?i)sql statement`, `(?i)join`, `(?i)pg_`},
	"vectorized":    {`(?i)vectorize`},
	"observability": {`(?i)admin\s+ui\b`},
	"storage":       {`(?i)rocks\s*db`, `(?i)pebble`},
	"optimizer":     {`(?i)explain(^ your problem)`},
	"cloud":         {`(?i)kubernetes`, `(?i)cloud`},
}

var teamToContacts = map[string][]string{
	"appdev":        {`rafiss`},
	"bulk-io":       {`dt`},
	"sql-schema":    {`lucy-zhang`},
	"kv":            {`nvanbenschoten`},
	"sql-features":  {`jordanlewis`},
	"vectorized":    {`asubiotto`},
	"observability": {`dhartunian`},
	"storage":       {`petermattis`},
	"optimizer":     {`RaduBerinde`},
	"cloud":         {`joshimhoff`},
}

type board struct {
	owner string
	repo  string
	name  string
}

var teamToBoards = map[string]board{
	"appdev":        {"cockroachdb", "", "AppDev"},
	"bulk-io":       {"cockroachdb", "cockroach", "Bulk I/O Backlog"},
	"sql-schema":    {"cockroachdb", "cockroach", "SQL Schema"},
	"kv":            {"cockroachdb", "cockroach", "KV"},
	"sql-features":  {"cockroachdb", "cockroach", "SQL Features"},
	"vectorized":    {"cockroachdb", "cockroach", "SQL Execution"},
	"observability": {"cockroachdb", "cockroach", "Web UI"},
	"storage":       {"cockroachdb", "", "Storage"},
	"optimizer":     {"cockroachdb", "cockroach", "SQL Optimizer"},
}

// findTeamsFromKeywords maps from owner to a list of suspect keywords.
func findTeamsFromKeywords(body string) map[string][]string {
	// Not efficient at all, but w/e.
	foundTeamToKeywords := map[string][]string{}
	for team, keywords := range teamToKeyword {
		for _, keyword := range keywords {
			keywordRe := regexp.MustCompile(keyword)
			matches := keywordRe.FindStringSubmatch(body)
			if len(matches) > 0 {
				foundTeamToKeywords[team] = append(foundTeamToKeywords[team], matches[0])
			}
		}
	}
	return foundTeamToKeywords
}
