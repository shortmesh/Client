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

// type IncomingMessage struct {
// 	RoomID  id.RoomID
// 	Sender  id.UserID
// 	Content event.Content
// }

/*
This function adds the user to the database and joins the bridge rooms
*/
// func (m *MatrixClient) ProcessActiveSessions(
// 	password string,
// ) error {
// 	log.Println("Processing active sessions for user:", m.Client.UserID.Localpart())
// 	var clientDB ClientDB = ClientDB{
// 		username: m.Client.UserID.Localpart(),
// 		filepath: "db/" + m.Client.UserID.Localpart() + ".db",
// 	}
// 	clientDB.Init()

// 	if m.Client.AccessToken != "" && m.Client.UserID != "" && password != "" {
// 		err := ks.CreateUser(m.Client.UserID.Localpart(), m.Client.AccessToken)
// 		if err != nil {
// 			return err
// 		}

// 		err = clientDB.Store(m.Client.AccessToken, password)

// 		if err != nil {
// 			return err
// 		}
// 	}

// 	for _, entry := range cfg.Bridges {
// 		for name, config := range entry {
// 			bridge := Bridges{
// 				Name:    name,
// 				Client:  m.Client,
// 				BotName: config.BotName,
// 			}

// 			err := bridge.JoinManagementRooms()
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	}

// 	return nil
// }

// func (m *MatrixClient) LoadActiveSessionsByAccessToken(accessToken string) (string, error) {
// 	log.Println("Loading active sessions: ", m.Client.UserID.Localpart(), accessToken)

// 	var clientDB ClientDB = ClientDB{
// 		username: m.Client.UserID.Localpart(),
// 		filepath: "db/" + m.Client.UserID.Localpart() + ".db",
// 	}
// 	clientDB.Init()
// 	exists, err := clientDB.AuthenticateAccessToken(m.Client.UserID.Localpart(), accessToken)

// 	if err != nil {
// 		return "", err
// 	}

// 	if !exists {
// 		return "", fmt.Errorf("access token does not exist")
// 	}

// 	return accessToken, nil
// }

// func (m *MatrixClient) LoadActiveSessions(
// 	password string,
// ) (string, error) {
// 	log.Println("Loading active sessions: ", m.Client.UserID.Localpart(), password)

// 	var clientDB ClientDB = ClientDB{
// 		username: m.Client.UserID.Localpart(),
// 		filepath: "db/" + m.Client.UserID.Localpart() + ".db",
// 	}
// 	clientDB.Init()
// 	exists, err := clientDB.Authenticate(m.Client.UserID.Localpart(), password)

// 	if err != nil {
// 		return "", err
// 	}

// 	if !exists {
// 		return "", nil
// 	}

// 	return clientDB.Fetch()
// }

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

	syncer.OnEventType(event.EventEncrypted, func(ctx context.Context, evt *event.Event) {
		evt, err := m.Client.Crypto.Decrypt(ctx, evt)
		if err != nil {
			panic(err)
		}
		ch <- evt
	})

	// syncer.OnEvent(func(ctx context.Context, evt *event.Event) {
	// 	fmt.Printf("%s\n", evt.Type)
	// })

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

	err := verifyRecoveryKey(m.CryptoHelper.Machine(), recoveryKey)
	if err != nil {
		panic(err)
	}

	return nil
}

// func (m *MatrixClient) SyncAllClients() error {
// 	log.Println("Syncing all clients")
// 	var wg sync.WaitGroup

// 	for {
// 		users, err := ks.FetchAllUsers()

// 		if err != nil {
// 			return err
// 		}

// 		for _, user := range users {
// 			if _, ok := syncingUsers[user.Username]; ok && len(syncingUsers[user.Username]) > 0 {
// 				continue
// 			} else {
// 				syncingUsers[user.Username] = []string{}
// 			}

// 			wg.Add(1)

// 			go func(user Users) {
// 				err := m.syncClient(user) //blocking
// 				if err != nil {
// 					log.Println("Error syncing client:", err)
// 					return
// 				}

// 			}(user)
// 		}

// 		time.Sleep(3 * time.Second)
// 	}
// }

// func (m *MatrixClient) syncClient(user Users) error {
// 	homeServer := cfg.HomeServer
// 	client, err := mautrix.NewClient(
// 		homeServer,
// 		id.NewUserID(user.Username, cfg.HomeServerDomain),
// 		user.AccessToken,
// 	)
// 	mc := MatrixClient{
// 		Client: client,
// 	}
// 	if err != nil {
// 		log.Println("Error creating bridge for user:", err, user.Username)
// 		return err
// 	}

// 	clientDb := ClientDB{
// 		username: user.Username,
// 		filepath: "db/" + user.Username + ".db",
// 	}

// 	clientDb.Init()
// 	bridges, err := clientDb.FetchBridgeRooms(user.Username)
// 	if err != nil {
// 		log.Println("Error fetching bridge rooms for user:", err, user.Username)
// 		return err
// 	}

// 	ch := make(chan *event.Event)
// 	go func() {
// 		for {
// 			evt := <-ch
// 			go m.processIncomingEvents(evt)
// 		}
// 	}()

// 	// insert bridge names into syncingUsers if not already present
// 	go func() {
// 		for _, bridge := range bridges {
// 			bridge.Client = client
// 			// bridge.Client.StateStore = mautrix.NewMemoryStateStore()
// 			if _, ok := syncingUsers[user.Username]; !ok {
// 				syncingUsers[user.Username] = []string{}
// 			}
// 			syncingUsers[user.Username] = append(syncingUsers[user.Username], bridge.Name)
// 			if _, ok := ClientDevices[user.Username]; !ok {
// 				ClientDevices[user.Username] = make(map[string][]string)
// 			} else if _, ok := ClientDevices[user.Username][bridge.Name]; !ok {
// 				ClientDevices[user.Username][bridge.Name] = make([]string, 0)
// 			}

// 			devices, err := bridge.ListDevices()
// 			log.Println("Devices for bridge:", bridge.Name, devices)

// 			if err != nil {
// 				log.Println("Error listing devices for user:", err, user.Username)
// 				continue
// 			}
// 			ClientDevices[user.Username][bridge.Name] = devices

// 			go func(bridge *Bridges) {
// 				bridgeCfg, ok := cfg.GetBridgeConfig(bridge.Name)
// 				if !ok {
// 					log.Println("Bridge config not found for:", bridge.Name)
// 					return
// 				}
// 				bridge.ProcessIncomingLoginDaemon(bridgeCfg)
// 			}(bridge)

// 			go func(bridge *Bridges) {
// 				bridge.CreateContactRooms()
// 				log.Println("Joined member rooms for bridge:", bridge.Name)
// 				// wg.Done()
// 			}(bridge)

// 			go func(bridge *Bridges) {
// 				bridge.GetRoomInvitesDaemon()
// 			}(bridge)
// 		}
// 	}()

// 	err = mc.Sync(ch)

// 	if err != nil {
// 		log.Println("Sync error for user:", err, client.UserID.String())
// 		return err
// 	}

// 	return nil
// }

// func (m *MatrixClient) processIncomingEvents(evt *event.Event) error {
// 	for _, subscriber := range EventSubscribers {
// 		if len(subscriber.ExcludeMsgTypes) > 0 {
// 			for _, excludeMsgType := range subscriber.ExcludeMsgTypes {
// 				if excludeMsgType == evt.Content.AsMessage().MsgType {
// 					continue
// 				}
// 			}
// 		}

// 		if subscriber.MsgType == nil || *subscriber.MsgType == evt.Content.AsMessage().MsgType {
// 			if subscriber.RoomID != "" && subscriber.RoomID != evt.RoomID {
// 				continue
// 			}

// 			if subscriber.Since != nil && evt.Timestamp <= subscriber.Since.UnixMilli() {
// 				continue
// 			}

// 			subscriber.Callback(evt)
// 		}

// 	}

// 	return nil
// }

func GenerateAndUploadClientKeys(cryptoHelper *cryptohelper.CryptoHelper) string {
	ctx := context.Background()
	machine := cryptoHelper.Machine()

	err := machine.SSSS.SetDefaultKeyID(ctx, "")
	if err != nil {
		fmt.Printf("Warning: couldn't clear default key ID: %v\n", err)
	}

	key, err := machine.SSSS.GenerateAndUploadKey(ctx, "f. society.1")
	if err != nil {
		panic(err)
	}

	err = machine.SSSS.SetDefaultKeyID(ctx, key.ID)
	if err != nil {
		panic(err)
	}

	err = machine.FetchCrossSigningKeysFromSSSS(ctx, key)
	if err != nil {
		// If it still fails, the account data on the server is likely corrupted.
		// You may need to manually reset the account's cross-signing via a client like Element.
		panic(err)
	}

	return key.RecoveryKey()
}

// func GenerateAndUploadClientKeys(cryptoHelper *cryptohelper.CryptoHelper) string {
// 	fmt.Println("[+] Generating recovery key...")
// 	ctx := context.Background()

// 	machine := cryptoHelper.Machine()

// 	passphrase := "f. society.1"
// 	key, err := machine.SSSS.GenerateAndUploadKey(ctx, passphrase)
// 	if err != nil {
// 		panic(err)
// 	}

// 	return key.RecoveryKey()
// }

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
