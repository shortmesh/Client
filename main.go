package main

import (
	"encoding/json"
	"fmt"
	"os"
	_ "sherlock/matrix/docs"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
	// "maunium.net/go/mautrix/id"
)

type User struct {
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	AccessToken      string `yaml:"access_token"`
	RecoveryKey      string `yaml:"recovery_key"`
	DeviceId         string `yaml:"device_id"`
	HomeServer       string `yaml:"homeserver"`
	HomeServerDomain string `yaml:"homeserver_domain"`
}

func main() {
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
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "--login":
			fmt.Println("[+] Login commencing...")
			password := conf.User.Password

			if _, err := (&MatrixClient{
				Client: client,
			}).Login(password); err != nil {
				panic(err)
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
		return
	}

	client.DeviceID = id.DeviceID(conf.User.DeviceId)
	cryptoHelper, err := SetupCryptoHelper(client)
	if err != nil {
		panic(err)
	}
	mc := MatrixClient{
		Client:       client,
		CryptoHelper: cryptoHelper,
	}

	mc.Client.Crypto = cryptoHelper

	fmt.Printf("[+] DeviceID: %s\n", mc.Client.DeviceID)

	go SyncUser(&mc, user)

	select {}
}

func SyncUser(mc *MatrixClient, user User) {
	var wg sync.WaitGroup
	wg.Add(1)

	ch := make(chan *event.Event)

	go func() {
		for {
			evt := <-ch
			// fmt.Printf("%s\n", evt.Content.AsEncrypted().Ciphertext)
			json, err := json.MarshalIndent(evt, "", "")

			if err != nil {
				panic(err)
			}
			fmt.Printf("%s\n", json)
			// fmt.Printf("%s\n", evt.Type)
		}
	}()

	err := mc.Sync(ch, user.RecoveryKey)
	if err != nil {
		panic(err)
	}
}
