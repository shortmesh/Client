package main

import (
	// 	"context"
	// 	"fmt"

	"encoding/json"
	"fmt"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var syncingUsers = make(map[string][]string)
var ClientDevices = make(map[string]map[string][]string)

type EventSubscriber struct {
	Name            string
	EventType       string
	MsgType         *event.MessageType
	ExcludeMsgTypes []event.MessageType
	Callback        func(event *event.Event)
	Since           *time.Time
	// RoomID          id.RoomID
}

var EventSubscribers = make([]EventSubscriber, 0)

type Controller struct {
	Client   *mautrix.Client
	Username string
	// UserID   id.UserID
}

type UserSync struct {
	Name         string
	MsgBridges   []*Bridges
	LoginBridges []*Bridges
	Syncing      bool
	SyncMutex    sync.Mutex
}

var cfg, cfgError = (&Conf{}).getConf()

// var GlobalWebsocketConnection = WebsocketController{
// 	Registry: make([]*WebsocketUnit, 0),
// }

// var GlobalController = Controller{
// 	Client: &mautrix.Client{
// 		UserID:      id.NewUserID(cfg.User.Username, cfg.HomeServerDomain),
// 		AccessToken: cfg.User.AccessToken,
// 	},
// 	Username: cfg.User.Username,
// }

var ks = Keystore{
	filepath: cfg.KeystoreFilepath,
}

// func (c *Controller) CreateProcess(password string) error {
// 	m := MatrixClient{
// 		Client: c.Client,
// 	}
// 	accessToken, err := m.Create(c.Username, password)

// 	if err != nil {
// 		return err
// 	}

// 	m.Client.UserID = id.NewUserID(c.Username, cfg.HomeServerDomain)
// 	m.Client.AccessToken = accessToken
// 	log.Println("[+] Created user: ", c.Username)

// 	err = m.ProcessActiveSessions(password)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (c *Controller) LoginProcess(password string) error {
// 	m := MatrixClient{
// 		Client: c.Client,
// 	}
// 	accessToken, err := m.LoadActiveSessions(password)
// 	if err != nil {
// 		return err
// 	}

// 	if accessToken == "" {
// 		if accessToken, err = m.Login(password); err != nil {
// 			return err
// 		}
// 	}

// 	m.Client.UserID = id.NewUserID(c.Username, cfg.HomeServerDomain)
// 	m.Client.AccessToken = accessToken
// 	err = m.ProcessActiveSessions(password)
// 	if err != nil {
// 		log.Println("Error processing active sessions:", err)
// 	}

// 	return nil
// }

// func (c *Controller) SendMessage(username, message, contact, platform, deviceName string, fileData []byte) error {
// 	formattedUsername, err := cfg.FormatUsername(platform, contact)
// 	if err != nil {
// 		return err
// 	}

// 	clientDb := ClientDB{
// 		username: username,
// 		filepath: "db/" + username + ".db",
// 	}

// 	clientDb.Init()

// 	rooms, err := clientDb.FetchRoomsByMembers(formattedUsername)
// 	if err != nil {
// 		return err
// 	}

// 	log.Println("Fetching rooms for", formattedUsername, rooms, "using device:", deviceName)

// 	if len(rooms) > 1 {
// 		log.Println("Multiple rooms found for", formattedUsername, rooms)
// 		return fmt.Errorf("multiple rooms found for: %s", formattedUsername)
// 	}

// 	if len(rooms) == 0 {
// 		log.Println("No rooms found for", formattedUsername)
// 		return fmt.Errorf("no rooms found for: %s", formattedUsername)
// 	}

// 	room := rooms[0]
// 	if fileData != nil {
// 		uploadResp, err := c.Client.UploadBytesWithName(
// 			context.Background(),
// 			fileData,
// 			"application/pdf",
// 			"shortmesh.pdf",
// 		)
// 		if err != nil {
// 			return err
// 		}
// 		fileMsg := &event.MessageEventContent{
// 			MsgType: event.MsgFile,
// 			Body:    "body",
// 			URL:     id.ContentURIString(uploadResp.ContentURI.String()),
// 			Info: &event.FileInfo{
// 				MimeType: "application/pdf",
// 				Size:     len(fileData),
// 			},
// 			FileName: "shortmesh.pdf",
// 		}
// 		resp, err := c.Client.SendMessageEvent(
// 			context.Background(),
// 			room.ID,
// 			event.EventMessage,
// 			fileMsg,
// 		)
// 		if err != nil {
// 			return err
// 		}
// 		log.Println("Sent PDF to", room.ID, resp.EventID)
// 	} else {
// 		resp, err := c.Client.SendText(
// 			context.Background(),
// 			room.ID,
// 			message,
// 		)
// 		if err != nil {
// 			return err
// 		}
// 		log.Println("Sent message to", room.ID, resp.EventID)
// 	}

// 	return nil
// }

// func (c *Controller) AddDevice(username, platform string) (string, error) {
// 	websocketUrl := ""
// 	if index := GetWebsocketIndex(username, platform); index > -1 {
// 		websocketUrl = GlobalWebsocketConnection.Registry[index].Url
// 	} else {
// 		clientDb := ClientDB{
// 			username: username,
// 			filepath: "db/" + username + ".db",
// 		}
// 		clientDb.Init()

// 		bridges, err := clientDb.FetchBridgeRooms(username)
// 		if err != nil {
// 			return "", err
// 		}

// 		bridge := &Bridges{
// 			Name:   platform,
// 			Client: c.Client,
// 		}

// 		for _, _bridge := range bridges {
// 			if _bridge.Name == platform {
// 				bridge.RoomID = _bridge.RoomID
// 				break
// 			}
// 		}

// 		if bridge.RoomID == "" {
// 			return "", fmt.Errorf("bridge room not found for: %s", platform)
// 		}

// 		ws := Websockets{Bridge: bridge}

// 		websocketUrl = ws.RegisterWebsocket(platform, username)
// 	}

// 	return websocketUrl, nil
// }

func SyncUsers() {
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

	var wg sync.WaitGroup
	wg.Add(1)

	ch := make(chan *event.Event)

	go func() {
		for {
			evt := <-ch
			json, err := json.MarshalIndent(evt, "", "")

			if err != nil {
				panic(err)
			}
			fmt.Printf("%s\n", json)
		}
	}()

	err = mc.Sync(ch, user.RecoveryKey)
	if err != nil {
		panic(err)
	}
}
