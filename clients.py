#!/usr/bin/env python3

import os
import sys
import requests
import json

def store(username, accessToken, deviceId):
    url = "http://localhost:8080/store"
    payload = { 
        "username" : username, 
        "access_token" : accessToken, 
        "device_id" : deviceId, 
    }
    print(payload)
    response = requests.post(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print(json.dumps(response.json(), indent=4))

def list_devices(username):
    url = "http://localhost:8080/devices"
    payload = { "username" : username, }
    print(payload)
    response = requests.get(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print(json.dumps(response.json(), indent=4))

def login(username, password):
    url = "http://localhost:8080/login"
    payload = {
        "username" : username,
        "password" : password,
    }
    response = requests.post(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print("User created successfully...")
        print(json.dumps(response.json(), indent=4))


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: clients.py --[login|list-devices] [username password|username]")
        exit()
    

    if sys.argv[1] == "--login":
        username = sys.argv[2]
        password = sys.argv[3]
        try:
            login(username, password)
        except Exception as error:
            print(error)

    elif sys.argv[1] == "--list-devices":
        username = sys.argv[2]
        try:
            list_devices(username)
        except Exception as error:
            print(error)

    elif sys.argv[1] == "--store":
        username = sys.argv[2]
        accessToken = sys.argv[3]
        deviceId = sys.argv[4]
        try:
            store(username, accessToken, deviceId)
        except Exception as error:
            print(error)