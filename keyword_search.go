package blathers

import "regexp"

var teamToKeyword = map[string][]string{
	"appdev": {`(?i)\borm\b`, `ORM`, `(?i)django`, `(?i)hibernate`,
		`(?i)spring`, `(?i)\becto\b`, `(?i)activerecord`, `(?i)rails`, `(?i)sqlalchemy`,
		`(?i)sequelize`, `(?i)jooq`, `(?i)pgjdbc`, `(?i)pgx`, `(?i)libpq`,
		`(?i)cockroach-go`, `(?i)ponyorm`, `(?i)postico`, `(?i)peewee`, `(?i)flyway`,
		`(?i)hasura`, `(?i)liquibase`, `(?i)gorm`, `(?i)alembic`, `(?i)knex`,
		`(?i)prisma`, `(?i)psycopg2`, `(?i)tableplus`,
	},
	"bulk-io":       {`(?i)backup`, `(?i)import`, `(?i)export`, `(?i)cockroach dump`, `(?i)restore`, `(?i)changefeed`, `(?i)cdc`},
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
	"bulk-io":       {`cockroachdb/bulk-io`},
	"sql-schema":    {`lucy-zhang`},
	"kv":            {`nvanbenschoten`},
	"sql-features":  {`solongordon`},
	"vectorized":    {`asubiotto`},
	"observability": {`dhartunian`},
	"storage":       {`petermattis`},
	"optimizer":     {`RaduBerinde`},
	"cloud":         {`joshimhoff`},
}

type board struct {
	owner  string
	repo   string
	name   string
	column string
}

var teamToBoards = map[string]board{
	"appdev":        {"cockroachdb", "", "AppDev", "Backlog"},
	"bulk-io":       {"cockroachdb", "cockroach", "Bulk I/O Backlog", "Triage"},
	"sql-schema":    {"cockroachdb", "cockroach", "SQL Schema", "Triage"},
	"kv":            {"cockroachdb", "cockroach", "KV", "Incoming"},
	"sql-features":  {"cockroachdb", "cockroach", "SQL Features", "Triage"},
	"vectorized":    {"cockroachdb", "cockroach", "SQL Execution", "Triage"},
	"observability": {"cockroachdb", "cockroach", "Web UI", "Landing Pad"},
	"storage":       {"cockroachdb", "", "Storage", "Incoming"},
	"optimizer":     {"cockroachdb", "cockroach", "SQL Optimizer", "Triage"},
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
