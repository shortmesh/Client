package cmd

import (
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"

	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type SyncUserCallback func(user users.Users) error

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
			err := s.syncUser(user)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
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

func Sync(user users.Users) error {
	slog.Debug("Syncing user", "UserID", user.Client.UserID.String(), "DeviceID", user.Client.DeviceID)
	err := ParseRoomSubroutine(user.Client)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	cryptoHelper, err := SetupCryptoHelper(user.Client, []byte(user.PickleKey))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	mc := MatrixClient{
		Client:       user.Client,
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
				err = (&bridges.Bridges{Client: mc.Client}).SyncCallback(evt)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
				}
			}()
		}
	}()

	err = mc.Sync(user, ch)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}
