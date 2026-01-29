package main

import (
	"fmt"
	"log"
	"os"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ?
// TODO
// *
// !

// EXPLORATION:
func addDevice(client *mautrix.Client, bridgeName string) {
	if err := (&Controller{
		Client: client,
	}).AddDevice(bridgeName); err != nil {
		log.Panic(err)
	}
}

// EXPLORATION:
func addBridges(client *mautrix.Client) {
	if err := (&Controller{
		Client: client,
	}).AddBridges(); err != nil {
		log.Panic(err)
	}
}

// EXPLORATION:
func authenticate(client *mautrix.Client) {
	conf, err := cfg.getConf()
	password := conf.User.Password

	if _, err := (&MatrixClient{
		Client: client,
	}).Login(password); err != nil {
		log.Panic(err)
	}

	fmt.Printf("[+] DeviceID: %s\n", client.DeviceID)
	fmt.Printf("[+] AccessToken: %s\n", client.AccessToken)

	cryptoHelper, err := SetupCryptoHelper(client)
	if err != nil {
		panic(err)
	}

	recoverykey := GenerateAndUploadClientKeys(cryptoHelper)
	fmt.Printf("[+] RecoveryKey: %s\n", recoverykey)
}

func TerminalRoutines() {
	conf, err := cfg.getConf()

	if err != nil {
		panic(err)
	}

	user := User{
		Username:         conf.User.Username,
		AccessToken:      conf.User.AccessToken,
		RecoveryKey:      conf.User.RecoveryKey,
		HomeServer:       conf.HomeServer,
		HomeServerDomain: conf.HomeServerDomain,
	}

	client, err := mautrix.NewClient(
		user.HomeServer,
		id.NewUserID(user.Username, user.HomeServerDomain),
		user.AccessToken,
	)

	switch os.Args[2] {
	case "--authenticate":
		fmt.Println("[+] Login commencing...")
		authenticate(client)
	case "--add-device":
		fmt.Println("[+] Adding device commencing...")
		bridgeName := os.Args[3]
		addDevice(client, bridgeName)
	case "--add-bridge":
		fmt.Println("[+] Adding bridges commencing...")
		addBridges(client)
	case "--send-message":
	}

}
