# Blathers

## Getting Secrets
Secrets can be viewed [here](https://console.cloud.google.com/functions/edit/us-central1/blathers-bot?project=cockroach-dev-inf) under "Environment variables, networking, timeouts and more".

## Developing Locally

* Get this code: `go get -u go/src/github.com/cockroachlabs/blathers-bot` 
* `base64` encode the private key for the bot, then export it: `export BLATHERS_GITHUB_PRIVATE_KEY=<your token here>`.
* Now can run the following to start the server
```
go run ./serv
```
* Then CURL your webhook request to test.
```
curl localhost:8080/github_webhook -H 'X-Github-Event: <event>' --header "Content-Type:application/json" --data '<data>'
```
* Example data is available in `example_webhooks/`. You can run curl with them:
```
curl localhost:8080/github_webhook -H 'X-Github-Event: pull_request'  --header "Content-Type:application/json" --data "@./example_webhooks/app_webhook_synchronize"
```

## Deployment
After merging your change, click "Deploy" on [here](https://console.cloud.google.com/functions/edit/us-central1/blathers-bot?project=cockroach-dev-inf).

## Re-deliver Payloads
Go [here](https://github.com/settings/apps/blathers-crl/advanced) and click "redeliver" after you find a request you like.

## Blathers Inspiration
> "Eeek! A bug...! Ah, I beg your pardon! I just don't like handling these things much!"

Blathers is an owl who hates bugs.
