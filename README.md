# Blathers

## Developing Locally

* Get this code: `go get -u go/src/github.com/otan-cockroach/blathers-bot` 
* Get access! You have two options:
  * Get a GitHub Access Token, then export it: `export BLATHERS_GITHUB_ACCESS_TOKEN=<your token here>`
  * Use the bot token. base64 encode the private key, then export it: `export BLATHERS_GITHUB_PRIVATE_KEY=<your token here>`.
* Optionally set `BLATHERS_GITHUB_CLIENT_ID` if using a different app, and `BLATHERS_GITHUB_APP_SECRET` if the we should check the secret matches for the webhook.
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

## Blathers Inspiration
> "Eeek! A bug...! Ah, I beg your pardon! I just don't like handling these things much!"

Blathers is an owl who hates bugs.
