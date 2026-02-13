#!/usr/bin/env python3

import os
import sys
import requests
import json

def send_message(username, platformName, deviceId, contact, message):
    url = f"http://localhost:8080/api/v1/devices/{deviceId}/message"
    payload = { 
        "username" : username, 
        "platform_name" : platformName, 
        "device_id" : deviceId, 
        "contact" : contact, 
        "text" : message, 
    }
    print(payload)
    response = requests.post(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print(json.dumps(response.json(), indent=4))

def store(username, accessToken, deviceId):
    url = "http://localhost:8080/api/v1/store"
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
    url = "http://localhost:8080/api/v1/devices"
    payload = { "username" : username, }
    print(payload)
    response = requests.get(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print(json.dumps(response.json(), indent=4))

def add_device(username, platform_name):
    url = "http://localhost:8080/api/v1/devices"
    payload = {
        "username" : username,
        "platform_name" : platform_name,
    }
    response = requests.post(url, json=payload)

    response.raise_for_status()

    if response.status_code == 200:
        print("User created successfully...")
        print(json.dumps(response.json(), indent=4))

def login(username, password):
    url = "http://localhost:8080/api/v1/login"
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
        print("Usage: clients.py --[login|list-devices|add-device|store|send-message]")
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

    elif sys.argv[1] == "--add-device":
        username = sys.argv[2]
        platformName = sys.argv[3]
        try:
            add_device(username, platformName)
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

    elif sys.argv[1] == "--send-message":
        username = sys.argv[2]
        platformName = sys.argv[3]
        deviceId = sys.argv[4]
        contact = sys.argv[5]
        message = sys.argv[6]
        try:
            send_message(username, platformName, deviceId, contact, message)
        except Exception as error:
            print(error)