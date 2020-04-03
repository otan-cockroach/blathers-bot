import logging
import json
import os
import requests
import sys
import re
from abc import ABC, abstractmethod
from flask import Flask, abort, request as flask_request
from collections import defaultdict

assert 'BLATHERS_GITHUB_ACCESS_TOKEN' in os.environ

logging.basicConfig(stream=sys.stderr)

application = Flask(__name__)

headers = {"Authorization": "token " + os.environ['BLATHERS_GITHUB_ACCESS_TOKEN']}

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

    def do(self):
        if not self.comments:
            return
        self.comments.append("NOTE: I am experimental.")
        text = '\n\n'.join(self.comments)
        logging.info('posting comment: %s', text)
        resp = requests.post(
            self.comments_url,
            headers=headers,
            json={
                "body": text,
            },
        )
        if resp.status_code != 201:
            raise Exception('unknown response from comment creation')


def is_organization_member(user_name):
    resp = requests.get('https://api.github.com/orgs/cockroachdb/members/{}'.format(user_name), headers=headers)
    if resp.status_code == 204:
        return True
    if resp.status_code == 404:
        return False
    raise Exception('unknown status code for is_organization_member: {}'.format(resp.status_code))

def list_commits(commits_url):
    resp = requests.get(commits_url, headers=headers)
    if resp.status_code != 200:
        raise Exception('unable to fetch commits: {}'.format(resp))
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
    # Let's only handle opened pull requests for now.
    logging.info('found pull_request %s', json.dumps(req))
    if req['action'] not in ('opened'):
        return json.dumps({'status': 'do not care about this event'})

    if is_organization_member(req['pull_request']['user']['login']) and req['pull_request']['user']['login'] != 'otan':
        return json.dumps({'status': 'not handled as person is member of the organization'})

    actions = PullRequestActions(req['pull_request'])
    if req['action'] == 'opened':
        actions.add_comment(
            'Hello external contributor! I am Blathers and I am here to assist you get your PR merged. ' +
            'If this is your first PR, please make sure you have read [this guide to creating your first PR](https://wiki.crdb.io/wiki/spaces/CRDB/pages/181633464/Your+first+CockroachDB+PR]).',
        )

    # Check for action items.
    action_items = []

    # Check commits.
    commits = list_commits(req['pull_request']['commits_url'])
    if len(commits) > 1:
        action_items.append('Generally we try and keep pull requests to one commit. [Please squash your commits](https://github.com/wprig/wprig/wiki/How-to-squash-commits), and re-push with `--force`.')
    for commit in commits:
        if 'Release note' not in commit['commit']['message']:
            action_items.append('Please ensure your git commit message contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes).')
            break
    if 'Release note' not in req['pull_request']['body']:
        action_items.append('Please ensure your pull request description contains a [release note](https://wiki.crdb.io/wiki/spaces/CRDB/pages/186548364/Release+notes) - this can be the same as the one in your commit message.')

    if action_items:
        actions.add_comment(
            'Before we can accept your PR, please review the following recommendations: {}'.format(
                ''.join(['\n * ' + item for item in action_items]),
            ),
        )

    # Automatically assign reviewers who can help.
    if len(req['pull_request']['requested_reviewers']) == 0:
        actions.add_comment('''We were unable to automatically find a reviewer for your issue. You can try CCing one of the following members:
* A person you worked with closely on this PR.
* The person who created the ticket, or a CRDB organization member involved with the ticket.
* Join our [community slack channel](https://cockroa.ch/slack) and ask on #contributors.
* You can find members of the CockroachDB organization [here](https://github.com/orgs/cockroachdb/people).''')

    # Apply actions and return.
    actions.do()
    return json.dumps({'status': 'handled pull request'})

def handle_issue(req):
    # Let's only handled opened issues for now.
    logging.info('found issue %s', json.dumps(req))
    if req['action'] != 'opened':
        return json.dumps({'status': 'issue is not new'})
    if is_organization_member(req['issue']['user']['login']):
        return json.dumps({'status': 'not handled as person is member of the organization'})
    return json.dumps({'status': 'handled issue'})

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
