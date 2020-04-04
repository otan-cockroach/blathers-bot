import logging
import json
import jwt
import os
import requests
import sys
import re
import base64
from abc import ABC, abstractmethod
from flask import Flask, abort, request as flask_request
from collections import defaultdict
from datetime import datetime, timedelta

logging.basicConfig(stream=sys.stderr)

application = Flask(__name__)

access_token = ("", datetime.now())
def get_headers(installation_id):
    global access_token
    if installation_id and 'BLATHERS_GITHUB_PRIVATE_KEY' in os.environ:
        if datetime.now() > access_token[1]:
            encoded_jwt = jwt.encode(
                {
                    'iss': 59700,
                    'iat': datetime.utcnow(),
                    'exp': datetime.utcnow() + timedelta(seconds=600),
                },
                base64.b64decode(os.environ['BLATHERS_GITHUB_PRIVATE_KEY']),
                algorithm='RS256',
            ).decode('utf-8')
            jwt_headers = {
                'Authorization': 'Bearer {}'.format(encoded_jwt),
                'Accept': 'application/vnd.github.machine-man-preview+json',
            }
            resp = requests.post(
                'https://api.github.com/app/installations/{}/access_tokens'.format(installation_id),
                headers=jwt_headers,
            )
            assert resp.status_code == 201, resp.content
            ret = resp.json()
            access_token = (ret['token'], datetime.strptime(ret['expires_at'], "%Y-%m-%dT%H:%M:%SZ"))
        return {"Authorization": "token " + access_token[0]}
    assert 'BLATHERS_GITHUB_ACCESS_TOKEN' in os.environ
    return {"Authorization": "token " + os.environ['BLATHERS_GITHUB_ACCESS_TOKEN']}

class Actions(ABC):
    def __init__(self):
        # comments contains a list of comments the bot should post.
        # They are combined into a giant "comment" with paragraph spacing
        # in between.
        self.comments = []

    def add_comment(self, comment):
        self.comments.append(comment)
        return self

    @abstractmethod
    def do(self):
        pass

class PullRequestActions(Actions):
    def __init__(self, pull_request):
        Actions.__init__(self)
        self.comments_url = pull_request['comments_url']
        self.reviewers = defaultdict(list)

    def add_reviewer(self, reviewer, reason):
        self.reviewers[reviewer].append(reason)

    def do(self, installation_id):
        if not self.comments:
            return
        self.comments.append("NOTE: I am experimental.")
        text = '\n\n'.join(self.comments)
        logging.info('posting comment: %s', text)
        resp = requests.post(
            self.comments_url,
            headers=get_headers(installation_id),
            json={
                "body": text,
            },
        )
        assert resp.status_code == 201, resp.content


def is_organization_member(installation_id, user_name):
    resp = requests.get('https://api.github.com/orgs/cockroachdb/members/{}'.format(user_name), headers=get_headers(installation_id))
    if resp.status_code == 204:
        return True
    if resp.status_code == 404:
        return False
    assert False, resp

def list_commits(installation_id, commits_url):
    resp = requests.get(commits_url, headers=get_headers(installation_id))
    if resp.status_code != 200:
        assert False, resp.content
    return resp.json()

issue_regexes = [
    re.compile(reg)
    for reg in [
        r'#(\d+)',
        r'https://github.com/cockroachdb/cockroach/issues/(\d+)',
    ]
]

def get_associated_issues_from_text(text):
    issues = set()
    for reg in issue_regexes:
        for found in reg.finditer(text):
            issues.add(int(found.group(1)))
    return list(issues)

def handle_pull_request(req):
    logging.info('found pull_request %s', json.dumps(req))
    installation_id = req['installation']['id'] if 'installation' in req else None

    if req['action'] not in ('opened', 'synchronize'):
        return json.dumps({'status': 'do not care about this event'})

    if is_organization_member(installation_id, req['pull_request']['user']['login']) and req['pull_request']['user']['login'] != 'otan':
        return json.dumps({'status': 'not handled as person is member of the organization'})

    actions = PullRequestActions(req['pull_request'])
    if req['action'] == 'opened':
        contributor_text_gate = 'Please ensure you have followed the guidelines for [creating a PR]'
        if req['pull_request']['author_association'] == 'FIRST_TIME_CONTRIBUTOR':
            contributor_text_gate = 'Since this is your first PR, please make sure you have read [this guide to creating your first PR]'
        actions.add_comment(
            'Hello external contributor! I am Blathers and I am here to assist to get your PR merged. ' +
            '{}(https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).'.format(
                contributor_text_gate,
            ),
        )
    else:
        actions.add_comment('Thanks for updating your pull request.')

    # Check for action items.
    action_items = []

    # Check commits.
    commits = list_commits(installation_id, req['pull_request']['commits_url'])
    if len(commits) > 1:
        action_items.append('Generally we try and keep pull requests to one commit. [Please squash your commits](https://github.com/wprig/wprig/wiki/How-to-squash-commits), and re-push with `--force`.')
    for commit in commits:
        if 'Release note' not in commit['commit']['message']:
            action_items.append('Please ensure your git commit message contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes).')
            break
    if 'Release note' not in req['pull_request']['body']:
        action_items.append('Please ensure your pull request description contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes) - this can be the same as the one in your commit message.')

    if action_items:
        action_items.append('When CI has completed, please ensure there are no errors that have appeared.')
        actions.add_comment(
            'Before we can accept your PR, please review the following recommendations: {}'.format(
                ''.join(['\n * ' + item for item in action_items]),
            ),
        )
    else:
        actions.add_comment('My owl senses detect that your PR is good for review. Please keep an out for any test failures in CI.')

    # Automatically assign reviewers who can help.
    if len(req['pull_request']['requested_reviewers']) == 0:
        actions.add_comment('''We were unable to automatically find a reviewer for your issue. You can try CCing one of the following members:
* A person you worked with closely on this PR.
* The person who created the ticket, or a [CRDB organization member](https://github.com/orgs/cockroachdb/people) involved with the ticket (author, commenter, etc.).
* Join our [community slack channel](https://cockroa.ch/slack) and ask on #contributors.
* Try find someone else from [here](https://github.com/orgs/cockroachdb/people).''')

    # Apply actions and return.
    actions.do(installation_id)
    return json.dumps({'status': 'handled pull request'})

@application.route('/webhook', methods=['POST'])
def do_webhook():
    return webhook(flask_request)

def webhook(request):
    # TODO: handle secret
    if 'X-GitHub-Event' not in request.headers:
        abort(400, json.dumps({'error': 'no X-GitHub-Event'}))
    event = request.headers['X-GitHub-Event']
    logging.info('got event: %s', event)
    if event == 'ping':
        return json.dumps({'msg': 'pong'})
    elif event == 'pull_request':
        return handle_pull_request(request.get_json())
    return json.dumps({'status': 'event insignificant: {}'.format(event)})

@application.route('/')
def index():
    return json.dumps({'hoo!': 'bugs!'})

if __name__ == '__main__':
    application.run(debug=True, host='0.0.0.0')
