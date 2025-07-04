from flask import Flask, request

app = Flask(__name__)

@app.route('/weatherstation/updateweatherstation.php')
def weather_update():
    args = request.args
    print(f"Weather update received: {args}")
    return 'success'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)

