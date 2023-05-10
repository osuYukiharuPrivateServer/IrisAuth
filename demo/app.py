from flask import Flask, redirect, request
import requests
import json

app = Flask(__name__)


@app.route('/')
def index():  # put application's code here
    return 'APP Demo of IrisAuth Porject with Python Flask. To use this project, visit localhost:5000/login'

@app.route('/success')
def success():
    return 'Login Successfully'

@app.route('/wrong')
def wrong():
    return 'Something wrong happens'

@app.route('/login')
def login():
    return redirect(
        'http://localhost:7899/login?redirect_url=http://localhost:5000/success&check_url=http://localhost:5000/doublecheck&a=1')  # you might need to change the port here to fit your IrisAuth's config

@app.route('/doublecheck')
def doublecheck():
    if 'uid' and 'redirect_url' in request.args:
        uid = str(request.args['uid'])
        redirect_url = str(request.args['redirect_url'])
        print(uid + ' ' + redirect_url)
        req = requests.get('http://localhost:7899/doublecheck?uid=' + uid)
        req_json = req.json()
        json_str = json.dumps(req_json)
        data = json.loads(json_str)
        print(data)
        if data["status"] == '0':
            print(data)
            return redirect(redirect_url)
        else:
            return redirect('/wrong')
    else:
        return 'Something wrong happens'



if __name__ == '__main__':
    app.run()
