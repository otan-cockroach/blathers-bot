package blathers

import (
	"context"

	"github.com/google/go-github/v32/github"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/opsgenie/opsgenie-go-sdk-v2/schedule"
)

var emailsToGitHub = map[string]string{
	"mvardi@cockroachlabs.com":   "mattcrdb",
	"florence@cockroachlabs.com": "florence-crl",
	"tim@cockroachlabs.com":      "tim-o",
	"ron@cockroachlabs.com":      "roncrdb",
	"ricardo@cockroachlabs.com":  "ricardocrdb",
}

func (srv *blathersServer) FindSupportOncall(
	ctx context.Context, ghClient *github.Client, org string,
) ([]string, error) {
	scheduleCli, err := schedule.NewClient(&client.Config{ApiKey: srv.opsgenieAPIKey})
	if err != nil {
		return nil, wrapf(ctx, err, "could not find opsgenie")
	}
	flat := true
	oncalls, err := scheduleCli.GetOnCalls(ctx, &schedule.GetOnCallsRequest{
		ScheduleIdentifierType: schedule.Name,
		ScheduleIdentifier:     "TSE On Call",
		Flat:                   &flat,
	})
	if err != nil {
		return nil, wrapf(ctx, err, "no oncalls found")
	}
	oncallNames := map[string]struct{}{}
	names := []string{}
	for _, oncall := range oncalls.OnCallRecipients {
		oncallNames[oncall] = struct{}{}
		if name, ok := emailsToGitHub[oncall]; ok {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return names, nil
	}
	orgLogins, err := getOrganizationLogins(ctx, ghClient, org)
	if err != nil {
		return nil, wrapf(ctx, err, "no org logins found for oncall")
	}
	for name, member := range orgLogins {
		if _, ok := oncallNames[member.GetEmail()]; ok {
			names = append(names, name)
		} else if _, ok := oncallNames[member.GetName()]; ok {
			names = append(names, name)
		} else if _, ok := oncallNames[member.GetLogin()]; ok {
			names = append(names, name)
		}
	}
	return names, nil
}
