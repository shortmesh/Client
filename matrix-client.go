package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
)

type SyncingClients struct {
	Users map[string]*UserSync
}

type ClientDB struct {
	connection *sql.DB
	username   string
	filepath   string
}

type MatrixClient struct {
	Client       *mautrix.Client
	CryptoHelper *cryptohelper.CryptoHelper
}

func (m *MatrixClient) Login(password string) (string, error) {
	identifier := mautrix.UserIdentifier{
		Type: mautrix.IdentifierTypeUser,
		User: m.Client.UserID.String(),
	}

	resp, err := m.Client.Login(context.Background(), &mautrix.ReqLogin{
		Type:             mautrix.AuthTypePassword,
		Identifier:       identifier,
		Password:         password,
		StoreCredentials: true,
	})
	if err != nil {
		return "", err
	}
	m.Client.AccessToken = resp.AccessToken
	m.Client.DeviceID = resp.DeviceID

	return resp.AccessToken, nil
}

func Logout(client *mautrix.Client) error {
	// Logout from the session
	_, err := client.Logout(context.Background())
	if err != nil {
		log.Printf("Logout failed: %v\n", err)
	}

	// TODO: delete the session file

	fmt.Println("Logout successful.")
	return err
}

func (m *MatrixClient) Create(username string, password string) (string, error) {
	fmt.Printf("[+] Creating user: %s\n", username)

	_, err := m.Client.RegisterAvailable(context.Background(), username)
	if err != nil {
		return "", err
	}
	// if !available.Available {
	// 	log.Fatalf("Username '%s' is already taken", username)
	// }

	resp, _, err := m.Client.Register(context.Background(), &mautrix.ReqRegister{
		Username: username,
		Password: password,
		Auth: map[string]interface{}{
			"type": "m.login.dummy",
		},
	})

	if err != nil {
		return "", err
	}

	return resp.AccessToken, nil
}

func SetupCryptoHelper(cli *mautrix.Client) (*cryptohelper.CryptoHelper, error) {
	// remember to use a secure key for the pickle key in production

	conf, err := cfg.getConf()
	if err != nil {
		panic(err)
	}
	pickleKeyString := conf.PickleKey
	pickleKey := []byte(pickleKeyString)

	// this is a path to the SQLite database you will use to store various data about your bot
	dbPath := "db/crypto.db"

	helper, err := cryptohelper.NewCryptoHelper(cli, pickleKey, dbPath)
	if err != nil {
		return nil, err
	}

	// initialize the database and other stuff
	err = helper.Init(context.Background())
	if err != nil {
		return nil, err
	}

	return helper, nil
}

func (m *MatrixClient) Sync(ch chan *event.Event, recoveryKey string) error {
	syncer := mautrix.NewDefaultSyncer()
	m.Client.Syncer = syncer
	machine := m.CryptoHelper.Machine()

	syncer.OnEventType(event.EventEncrypted, func(ctx context.Context, evt *event.Event) {
		evt, err := m.Client.Crypto.Decrypt(ctx, evt)
		if err != nil {
			log.Println(err)
		}
		ch <- evt
	})

	// Handle incoming room key events
	syncer.OnEvent(func(ctx context.Context, evt *event.Event) {
		// Let the crypto machine handle all to-device encryption events
		if evt.Type.Class == event.ToDeviceEventType {
			machine.HandleToDeviceEvent(ctx, evt)
			log.Printf("Handled to device...")
		} else {
			(&Rooms{
				Client: m.Client,
				ID:     evt.RoomID,
			}).GetInvites(evt)
		}
		log.Printf("%s\n", evt.Type)
	})

	readyChan := make(chan bool)
	var once sync.Once
	syncer.OnSync(func(ctx context.Context, resp *mautrix.RespSync, since string) bool {
		once.Do(func() {
			close(readyChan)
		})

		return true
	})

	go func() {
		if err := m.Client.Sync(); err != nil {
			panic(err)
		}
	}()

	log.Println("Waiting for sync to receive first event from the encrypted room...")
	<-readyChan
	log.Println("Sync received")

	// err := verifyRecoveryKey(m.CryptoHelper.Machine(), recoveryKey)
	// if err != nil {
	// 	panic(err)
	// }

	return nil
}

func GenerateAndUploadClientKeys(cryptoHelper *cryptohelper.CryptoHelper) string {
	ctx := context.Background()
	machine := cryptoHelper.Machine()

	passphrase := "f.society"
	err := machine.ShareKeys(ctx, 1)
	if err != nil {
		panic(err)
	}

	key, err := machine.SSSS.GenerateAndUploadKey(ctx, passphrase)
	if err != nil {
		panic(err)
	}

	// conf, err := cfg.getConf()
	log.Println("[+] Verifying machine access token: ", cryptoHelper.Machine().Client.AccessToken)
	uiaCallback := func(flows *mautrix.RespUserInteractive) interface{} {
		log.Printf("UIA flows available: %+v", flows)

		// Try using the access token directly
		return map[string]interface{}{
			"type":    mautrix.AuthTypeToken,
			"session": flows.Session,
			"token":   machine.Client.AccessToken,
		}
	}
	recoveryKey, _, err := machine.GenerateAndUploadCrossSigningKeys(ctx, uiaCallback, passphrase)
	if err != nil {
		// If it still fails, the account data on the server is likely corrupted.
		// You may need to manually reset the account's cross-signing via a client like Element.
		panic(err)
	}
	log.Println("[+] Recovery key: ", recoveryKey)

	err = machine.SSSS.SetDefaultKeyID(ctx, key.ID)
	if err != nil {
		panic(err)
	}

	err = machine.SignOwnDevice(ctx, machine.OwnIdentity())
	if err != nil {
		panic(err)
	}

	err = machine.SignOwnMasterKey(ctx)
	if err != nil {
		panic(err)
	}

	return key.RecoveryKey()
}

func verifyRecoveryKey(
	machine *crypto.OlmMachine,
	recoveryKey string,
) error {
	ctx := context.Background()
	keyId, keyData, err := machine.SSSS.GetDefaultKeyData(ctx)
	if err != nil {
		panic(err)
	}

	key, err := keyData.VerifyRecoveryKey(keyId, recoveryKey)
	if err != nil {
		panic(err)
	}

	err = machine.FetchCrossSigningKeysFromSSSS(ctx, key)
	if err != nil {
		panic(err)
	}

	err = machine.SignOwnDevice(ctx, machine.OwnIdentity())
	if err != nil {
		panic(err)
	}

	err = machine.SignOwnMasterKey(ctx)
	if err != nil {
		panic(err)
	}

	return nil
}
