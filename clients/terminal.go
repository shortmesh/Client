package clients

import (
	"fmt"
	"log"
	"log/slog"
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
func authenticate(client *mautrix.Client, password string, pickleKey []byte) {
	if _, err := (&cmd.MatrixClient{
		Client: client,
	}).Login(password); err != nil {
		log.Panic(err)
	}

	slog.Debug("authenticating",
		"deviceId", client.DeviceID,
		"accessToken", client.AccessToken,
		"password", password,
		"pickleKey", pickleKey,
	)

	cryptoHelper, err := cmd.SetupCryptoHelper(client, pickleKey)
	if err != nil {
		panic(err)
	}

	recoverykey, err := cmd.GenerateAndUploadClientKeys(cryptoHelper)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[+] RecoveryKey: %s\n", recoverykey)
}

func TerminalRoutines() {
	conf, err := configs.GetConf()

	if err != nil {
		panic(err)
	}

	username := os.Args[3]
	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		"",
	)

	switch os.Args[2] {
	case "--authenticate":
		fmt.Println("[+] Login commencing...")
		password := os.Args[4]
		pickleKey := os.Args[5] //TODO: b64 should be decoded
		authenticate(client, password, []byte(pickleKey))
	case "--add-device":
		fmt.Println("[+] Adding device commencing...")
		bridgeName := os.Args[3]
		client.AccessToken = os.Args[4]
		addDevice(client, bridgeName)
	case "--add-bridge":
		fmt.Println("[+] Adding bridges commencing...")
		client.AccessToken = os.Args[4]
		addBridges(client)
	case "--send-message":
		client.AccessToken = os.Args[4]
		deviceId := os.Args[5]
		bridgeName := os.Args[6]
		contact := os.Args[7]
		message := os.Args[8]
		fmt.Printf("[+] Sending message: From %s -> %s for %s, %s\n", deviceId, contact, bridgeName, message)
		sendMessage(client, deviceId, bridgeName, contact, message)
	}

}
