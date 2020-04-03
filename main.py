import logging
import json
from sys import stderr
from flask import Flask

logging.basicConfig(stream=stderr)

application = Flask(__name__)

@application.route('/webhook', methods=['POST'])
def webhook(request):
    # Implement ping
    event = request.headers.get('X-GitHub-Event', 'ping')
    if event == 'ping':
        return json.dumps({'msg': 'pong'})
    return json.dumps({'status': 'done'})

@application.route('/')
def index():
    return json.dumps({'hoo!': 'bugs!'})

if __name__ == '__main__':
    application.run(debug=True, host='0.0.0.0')
