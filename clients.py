#!/usr/bin/env python3

import os
import sys
import requests
import json

def login(username, password):
    url = "http://localhost:8080/login"
    payload = {
        "username" : username,
        "password" : password
    }
    response = requests.post(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print("User created successfully...")
        print(json.dumps(response.json(), indent=4))


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: clients.py [login|] [[username password] | ]")
        exit()
    
    if sys.argv[1] == "--login":
        username = sys.argv[2]
        password = sys.argv[3]
        try:
            login(username, password)
        except Exception as error:
            print(error)