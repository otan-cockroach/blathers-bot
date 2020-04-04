# Blathers

## Developing Locally

* (Make sure your have python3.7 and virtualenv installed)[https://cloud.google.com/python/setup].
* Get access! You have two options:
  * Get a GitHub Access Token, then export it: `export BLATHERS_GITHUB_ACCESS_TOKEN=<your token here>`
  * Use the bot token. base64 encode the private key, then export it: `export BLATHERS_GITHUB_PRIVATE_KEY=<your token here>`.
* Now can run the following to start the server
```
source venv/bin/activate
pip3 install requirements.txt
python3 main.py
```
* Then CURL your webhook request to test.
```
curl localhost:5000/webhook -H 'X-Github-Event: <event>' --header "Content-Type:application/json" --data '<data>'
```
* Example data is available in `example_webhooks/`. You can run curl with them:
```
curl localhost:5000/webhook -H 'X-Github-Event: pull_request'  --header "Content-Type:application/json" --data "@./example_webhooks/app_webhook_synchronize"
```

## Blathers Inspiration
> "Eeek! A bug...! Ah, I beg your pardon! I just don't like handling these things much!"

Blathers is an owl who hates bugs.
