package syncers

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type SyncEventCallback struct {
	ID        string
	EventType string
	Callback  func(evt *event.Event) error
}

type SyncUserCallback func(client *mautrix.Client, pickleKey []byte) error

type SyncWatcher struct {
	Cache    []id.UserID
	SyncUser SyncUserCallback
	Wg       *sync.WaitGroup
}

var syncEventCallbacks = make(map[string]*SyncEventCallback)

func (s *SyncWatcher) Add(user users.Users) error {
	if !slices.Contains(s.Cache, user.Client.UserID) {
		s.Wg.Add(1)
		go func() {
			slog.Debug("SyncWatcher", "Adding", user.Client.UserID)
			s.Cache = append(s.Cache, user.Client.UserID)
			err := s.SyncUser(user.Client, user.PickleKey)
			if err != nil {
				slog.Error(err.Error())
				s.Remove(user)
			}
		}()

	}
	return nil
}

func (s *SyncWatcher) Remove(user users.Users) {
	slog.Debug("SyncWatcher", "Removing", user.Client.UserID)
	removeIndex := -1
	for index, cachedUserId := range s.Cache {
		if cachedUserId == user.Client.UserID {
			removeIndex = index
			break
		}
	}

	if removeIndex != -1 {
		s.Cache[removeIndex] = s.Cache[len(s.Cache)-1]
		s.Cache = s.Cache[:len(s.Cache)-1]
	}

	defer s.Wg.Done()
}

func Sync(client *mautrix.Client, pickleKey []byte) error {
	slog.Debug("Syncing user", "UserID", client.UserID.String(), "DeviceID", client.DeviceID)
	err := ParseRoomSubroutine(client, true, nil)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	cryptoHelper, err := SetupCryptoHelper(client, pickleKey)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	mc := MatrixClient{
		Client:       client,
		CryptoHelper: cryptoHelper,
	}

	mc.Client.Crypto = cryptoHelper

	ch := make(chan *event.Event)
	go func() {
		for {
			evt := <-ch
			if evt == nil {
				continue
			}

			// json, err := json.MarshalIndent(evt, "", "")
			// if err != nil {
			// 	slog.Error(err.Error())
			// 	debug.PrintStack()
			// 	continue
			// }
			// slog.Debug("Incoming message", "message", json)

			// Process incoming from bridges
			go func() {
				for _, syncEventCallback := range syncEventCallbacks {
					go syncEventCallback.Callback(evt)
				}
			}()
		}
	}()

	err = mc.Sync(ch)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func UnRegisterSyncMessageListener(id string) error {
	slog.Debug("UnRegisterSyncMessageListener", "ID", id)
	if _, ok := syncEventCallbacks[id]; !ok {
		return fmt.Errorf("Event not registered")
	}

	delete(syncEventCallbacks, id)
	return nil
}

func RegisterSyncMessageListener(syncEventCallback *SyncEventCallback) error {
	slog.Debug("RegisterSyncMessageListener", "ID", syncEventCallback.ID)
	if _, ok := syncEventCallbacks[syncEventCallback.ID]; ok {
		return fmt.Errorf("Event already synced")
	}

	syncEventCallbacks[syncEventCallback.ID] = syncEventCallback
	return nil
}

func ParseRoomSubroutine(client *mautrix.Client, shouldRepair bool, roomId *id.RoomID) error {
	err := repairBridges(client)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	return nil
}

func repairBridges(client *mautrix.Client) error {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	for _, bridgeConf := range conf.Bridges {
		roomId, err := bridges.GetBotManagementRoom(client, (*id.UserID)(&bridgeConf.BotName))
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}

		if roomId == nil {
			slog.Debug("[+] Repairing bridge", "name", bridgeConf.Name)
			err = bridges.AddBridge(client, bridgeConf)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}
		}
	}

	return nil
}
