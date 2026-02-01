package clients

import (
	"fmt"
	"log"
	"os"

	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ?
// TODO
// *
// !

// EXPLORATION:
func sendMessage(client *mautrix.Client, deviceId, bridgeName, contact, message string) {
	if _, err := (&cmd.Controller{
		Client: client,
	}).SendMessage(bridgeName, deviceId, contact, message); err != nil {
		log.Panic(err)
	}
}

// EXPLORATION:
func addDevice(client *mautrix.Client, bridgeName string) {
	if err := (&cmd.Controller{
		Client: client,
	}).AddDevice(bridgeName); err != nil {
		log.Panic(err)
	}
}

// EXPLORATION:
func addBridges(client *mautrix.Client) {
	if err := (&cmd.Controller{
		Client: client,
	}).AddBridges(); err != nil {
		log.Panic(err)
	}
}

// EXPLORATION:
func authenticate(client *mautrix.Client) {
	conf, err := configs.GetConf()
	password := conf.User.Password

	if _, err := (&cmd.MatrixClient{
		Client: client,
	}).Login(password); err != nil {
		log.Panic(err)
	}

	fmt.Printf("[+] DeviceID: %s\n", client.DeviceID)
	fmt.Printf("[+] AccessToken: %s\n", client.AccessToken)

	cryptoHelper, err := cmd.SetupCryptoHelper(client)
	if err != nil {
		panic(err)
	}

	recoverykey := cmd.GenerateAndUploadClientKeys(cryptoHelper)
	fmt.Printf("[+] RecoveryKey: %s\n", recoverykey)
}

func TerminalRoutines() {
	conf, err := configs.GetConf()

	if err != nil {
		panic(err)
	}

	user := configs.UsersConfig{
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
		deviceId := os.Args[3]
		bridgeName := os.Args[4]
		contact := os.Args[5]
		message := os.Args[6]
		fmt.Printf("[+] Sending message: From %s -> %s for %s, %s\n", deviceId, contact, bridgeName, message)
		sendMessage(client, deviceId, bridgeName, contact, message)
	}

}
