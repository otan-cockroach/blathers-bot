package blathers

import "regexp"

var teamToKeyword = map[string][]string{
	"appdev":        {`(?i)\borm\b`, `ORM`, `(?i)django`, `(?i)hibernate`},
	"bulk-io":       {`(?i)backup`, `(?i)restore`, `(?i)changefeed`, `(?i)cdc`},
	"sql-schema":    {`(?i)alter\b`, `(?i)create\s+(table|index|database)`},
	"kv":            {`(?i)\bkv\b`, `(?i)HLC`},
	"sql-features":  {`(?i)sql statement`, `(?i)join`, `(?i)pg_`},
	"vectorized":    {`(?i)vectorize`},
	"observability": {`(?i)admin\s+ui\b`},
	"storage":       {`(?i)rocks\s+db`, `(?i)pebble`},
	"optimizer":     {`(?i)explain(^ your problem)`},
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
}

// findOwnersFromKeywords maps from owner to a list of suspect keywords.
func findOwnersFromKeywords(body string) map[string][]string {
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

	ret := map[string][]string{}
	for team, keywords := range foundTeamToKeywords {
		for _, contact := range teamToContacts[team] {
			ret[contact] = keywords
		}
	}
	return ret
}
