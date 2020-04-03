# Blathers

## Developing Locally

(Make sure your have python3.7 and virtualenv installed)[https://cloud.google.com/python/setup]

* Get a GitHub Access Token. Then you can do this:
```
export BLATHERS_GITHUB_ACCESS_TOKEN=<your token here>
```

* Now we can run the following.
```
source venv/bin/activate
pip3 install requirements.txt
python3 main.py
```

* Then CURL your webhook request to test.
```
curl localhost:5000/webhook -H 'X-Github-Event: <event>' --header "Content-Type:application/json" --data '<data>'
```

Then you can try CURL the webhook requests

## Blathers Inspiration
> "Eeek! A bug...! Ah, I beg your pardon! I just don't like handling these things much!"

Blathers is an owl who hates bugs.
