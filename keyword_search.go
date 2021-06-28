package blathers

import "regexp"

//go:generate go run generate/generate_projects.go

type teamInfo struct {
	Owner        string
	Repo         string
	ProjectName  string
	TriageColumn string
	tlabel       string
	keywords     []string
	contacts     []string

	ProjectID      int64
	TriageColumnID int64
}

var TeamInfo = map[string]*teamInfo{
	"bulk-io": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "Bulk I/O Backlog",
		TriageColumn: "Triage",
		tlabel:       "T-bulkio",
		keywords: []string{
			`(?i)backup`, `(?i)import\b`, `(?i)export`, `(?i)cockroach dump`, `(?i)restore`,
			`(?i)userfile`, `(?i)pgdump`},
		contacts: []string{"cockroachdb/bulkio"},
	},
	"cdc": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "CDC",
		TriageColumn: "Triage",
		tlabel:       "T-cdc",
		keywords: []string{
			`(?i)changefeed`, `(?i)changefeeds`, `(?i)changefeedccl`, `(?i)cdc`, `(?i)kafka`,
			`(?i)confluent`, `(?i)avro`, `(?i)kvfeed`, `(?i)schemafeed`},
		contacts: []string{"cockroachdb/cdc"},
	},
	"sql-schema": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "SQL Schema",
		TriageColumn: "Triage",
		tlabel:       "T-sql-schema",
		keywords: []string{
			`(?i)(alter|drop)\s+(table|index|database)`,
			`(?i)(table|database|schema|index|column)\s+descriptor`,
			`(?i)schema change`,
		},
		contacts: []string{"cockroachdb/sql-schema"},
	},
	"kv": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "KV Backlog",
		TriageColumn: "Incoming",
		tlabel:       "T-kv",
		keywords:     []string{`(?i)\bkv\b`, `(?i)HLC`, `(?i)raft`, `(?i)liveness`, `(?i)intents`, `(?i)mvcc`},
		contacts:     []string{"cockroachdb/kv"},
	},
	"sql-experience": {
		Owner:        "cockroachdb",
		ProjectName:  "SQL Experience",
		TriageColumn: "Triage",
		tlabel:       "T-sql-experience",
		keywords: []string{
			`(?i)sql statement`, `(?i)pg_`,
			`(?i)\borm\b`, `ORM`, `(?i)django`, `(?i)hibernate`,
			`(?i)spring`, `(?i)\becto\b`, `(?i)activerecord`, `(?i)rails`, `(?i)sqlalchemy`,
			`(?i)sequelize`, `(?i)jooq`, `(?i)pgjdbc`, `(?i)pgx`, `(?i)libpq`,
			`(?i)cockroach-go`, `(?i)ponyorm`, `(?i)postico`, `(?i)peewee`, `(?i)flyway`,
			`(?i)hasura`, `(?i)liquibase`, `(?i)gorm`, `(?i)alembic`, `(?i)knex`,
			`(?i)prisma`, `(?i)psycopg2`, `(?i)tableplus`,
		},
		contacts: []string{"cockroachdb/sql-experience"},
	},
	"sql-queries": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "SQL Queries",
		TriageColumn: "Triage",
		tlabel:       "T-sql-queries",
		keywords: []string{
			`(?i)optimizer`, `(?i)vectorized`, `(?i)distsql`,
			`(?i)explain(^ your problem)`, `(?i)plan`,
		},
		contacts: []string{"cockroachdb/sql-queries"},
	},
	"observability-inf": {
		Owner:        "cockroachdb",
		Repo:         "cockroach",
		ProjectName:  "Observability Infrastructure",
		TriageColumn: "Backlog",
		tlabel:       "T-observability-inf",
		keywords:     []string{`(?i)prometheus`, `(?i)metrics`},
		contacts:     []string{"dhartunian"},
	},
	"storage": {
		Owner:        "cockroachdb",
		ProjectName:  "Storage",
		TriageColumn: "Incoming",
		tlabel:       "T-storage",
		keywords:     []string{`(?i)rocks\s*db`, `(?i)pebble`, `(?i)lsm`, `(?i)compaction`},
		contacts:     []string{"cockroachdb/storage"},
	},
	"sql-observability": {
		Owner:        "cockroachdb",
		ProjectName:  "SQL Observability",
		TriageColumn: "Triage",
		tlabel:       "T-sql-observability",
		keywords: []string{
			`(?i)admin\s+ui\b`, `(?i)web\s+ui\b`, `(?i)db\s+console`,
			`(?i)statements page`, `(?i)transactions page`,
		},
		contacts: []string{"cockroachdb/sql-observability"},
	},
	"server-and-security": {
		Owner:        "cockroachdb",
		ProjectName:  "DB Server & Security",
		TriageColumn: "To do",
		tlabel:       "T-server-and-security",
		contacts:     []string{"knz"},
	},
}

// map from T-foo to team name
var tlabelToTeam map[string]string

// map from project id to team name
var projectIDToTeam map[int64]string

func init() {
	tlabelToTeam = make(map[string]string)
	projectIDToTeam = make(map[int64]string)
	for name, info := range TeamInfo {
		tlabelToTeam[info.tlabel] = name
		if info.ProjectID != 0 {
			projectIDToTeam[info.ProjectID] = name
		}
	}

}

var teamToLabels = map[string][]string{
	// There is additional logic in handleIssueLabelled to ensure both A- and T- labels
	"bulk-io": {"A-bulkio"},
	"cdc":     {"A-cdc"},
}

// findTeamsFromKeywords maps from owner to a list of suspect keywords.
func findTeamsFromKeywords(body string) map[string][]string {
	// Not efficient at all, but w/e.
	foundTeamToKeywords := map[string][]string{}
	for team, info := range TeamInfo {
		for _, keyword := range info.keywords {
			keywordRe := regexp.MustCompile(keyword)
			matches := keywordRe.FindStringSubmatch(body)
			if len(matches) > 0 {
				foundTeamToKeywords[team] = append(foundTeamToKeywords[team], matches[0])
			}
		}
	}
	return foundTeamToKeywords
}
