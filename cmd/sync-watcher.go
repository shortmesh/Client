package cmd

import (
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type SyncUserCallback func(client *mautrix.Client, pickleKey []byte) error

type SyncWatcher struct {
	cache    []id.UserID
	syncUser SyncUserCallback
	wg       *sync.WaitGroup
}

var syncWatcher SyncWatcher

func (s *SyncWatcher) Add(user users.Users) error {
	if !slices.Contains(s.cache, user.Client.UserID) {
		s.wg.Add(1)
		go func() {
			slog.Debug("SyncWatcher", "Adding", user.Client.UserID)
			s.cache = append(s.cache, user.Client.UserID)
			err := s.syncUser(user.Client, user.PickleKey)
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
	for index, cachedUserId := range s.cache {
		if cachedUserId == user.Client.UserID {
			removeIndex = index
			break
		}
	}

	if removeIndex != -1 {
		s.cache[removeIndex] = s.cache[len(s.cache)-1]
		s.cache = s.cache[:len(s.cache)-1]
	}

	defer s.wg.Done()
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
				err = bridges.SyncCallback(mc.Client, evt)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
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
