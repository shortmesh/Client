package cmd

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"unicode"

	"github.com/fsnotify/fsnotify"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/contacts"
	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/syncers"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func deleteFile(filePath *string) error {
	if filePath == nil {
		return nil // Or return an error if you expect it to always exist
	}

	err := os.Remove(*filePath)
	if err != nil {
		slog.Error("failed to delete file", "path", *filePath, "error", err)
		return err
	}

	slog.Info("file deleted successfully", "path", *filePath)
	return nil
}

func createTmpFile(fileExtension, fileContent string) (*string, error) {
	data, err := base64.StdEncoding.DecodeString(fileContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	mediaDir := "media"
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		slog.Error("failed to create media directory", "error", err)
		return nil, err
	}

	pattern := fmt.Sprintf("tmp-*.%s", fileExtension)
	tmpFile, err := os.CreateTemp(mediaDir, pattern)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	relativePath := filepath.Join(mediaDir, filepath.Base(tmpFile.Name()))

	return &relativePath, nil
}

func verifyGroupRoomValidity(
	client *mautrix.Client,
	roomId *id.RoomID,
	bridgeName,
	groupUrl,
	groupName string,
) (*id.RoomID, error) {
	ctx := context.Background()
	_, err := client.JoinedMembers(ctx, *roomId)
	if err != nil {
		slog.Error(err.Error())

		// something went out of sync
		resp, err := client.JoinedRooms(ctx)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, err
		}
		// allRooms, err := rooms.GetAllRooms(client)
		// if err != nil {
		// 	slog.Error(err.Error())
		// 	debug.PrintStack()
		// 	return nil, err
		// }
		slog.Debug("Reviewing rooms", "#", len(resp.JoinedRooms), "expecting", groupName)
		for _, room := range resp.JoinedRooms {
			name, err := rooms.GetRoomName(client, &room)
			if err != nil {
				topic, err := rooms.GetRoomTopic(client, &room)
				if err != nil {
					slog.Error(err.Error())
					continue
				}
				name = topic
			}
			name = strings.TrimRightFunc(name, unicode.IsSpace)
			if name == "" {
				continue
			}
			if name == groupName {
				slog.Debug("Attempting repair", "name", groupName, "former", roomId, "current", room)
				err = rooms.DeleteBridgeId(client, groupUrl)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return nil, err
				}
				err = rooms.UpdateBridgeId(
					client,
					room.String(),
					groupUrl,
					bridgeName,
				)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return nil, err
				}

				return &room, nil
			}
		}
		slog.Debug("Reviewing failed", "#expecting", groupName)
		return nil, fmt.Errorf("Failed to validate room: %s - (%s/%s)", roomId, groupName, groupUrl)
	}
	return roomId, nil
}

func noisyRoomIdRequest(
	client *mautrix.Client,
	bridgeCfg *configs.BridgeConfig,
	receiver,
	deviceId,
	homeserver string,
) (*id.RoomID, error) {
	var roomId *id.RoomID

	isUrl := false

	if utils.ExtractE164Contact(receiver) == "" {
		_, err := url.ParseRequestURI(receiver)
		isUrl = err == nil

		slog.Debug("Noisy room not contact", "receiver", receiver)
		roomIdStr, err := rooms.GetBridgedId(client, receiver)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		if roomIdStr != "" {
			newRoomId := id.RoomID(roomIdStr)
			roomId = &newRoomId
			return roomId, nil
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	callbackEventId := client.UserID.String() + bridgeCfg.Name + receiver
	ignoreBotMessage := false
	botUsername := id.UserID(bridgeCfg.BotName)
	mngRoom, err := bridges.GetBotManagementRoom(client, &botUsername)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	syncers.RegisterSyncMessageListener(&syncers.SyncEventCallback{
		ID:        callbackEventId,
		EventType: "m.room.message",
		IgnoreBot: ignoreBotMessage,
		BindRoom:  mngRoom,
		Callback: func(evt *event.Event, user *users.Users) error {
			defer func() {
				syncers.UnRegisterSyncMessageListener(callbackEventId)
				wg.Done()
			}()
			_roomId, err := isContactCallback(client, evt, &receiver, isUrl, bridgeCfg.Name, homeserver)
			if err != nil {
				slog.Error("Message cannot be sent", "error", err.Error())
				return err
			}
			roomId = _roomId

			return nil
		},
	})

	err = bridges.StartConversation(client, bridgeCfg, deviceId, receiver, isUrl)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	wg.Wait()

	return roomId, nil
}

func isContactCallback(
	client *mautrix.Client,
	evt *event.Event,
	receiver *string,
	isUrl bool,
	bridgeName,
	homeserver string,
) (*id.RoomID, error) {
	var roomId *id.RoomID
	if isUrl {
		content := evt.Content.AsMessage().Body
		// TODO: remove this
		var resolvedUrl string
		var groupName string

		if bridgeName == "wa" {
			prefix := "That invite link points at"
			gn, ru, err := utils.ParseInviteInfo(
				content,
				prefix,
			)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return nil, err
			}
			groupName = gn
			resolvedUrl = ru
		} else {
			ru, err := utils.ExtractBracketContent(content)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return nil, err
			}
			resolvedUrl = ru
		}

		resolvedUrl = strings.ReplaceAll(resolvedUrl, "`", "")

		roomIdStr, err := rooms.GetBridgedId(client, resolvedUrl)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		if roomIdStr != "" {
			tmpRoomId := id.RoomID(roomIdStr)
			// TODO: remove this
			if bridgeName == "wa" {
				newRoomId, err := verifyGroupRoomValidity(
					client,
					&tmpRoomId,
					bridgeName,
					resolvedUrl,
					groupName,
				)
				if err != nil {
					slog.Error(err.Error())
					return nil, err
				}
				roomId = newRoomId
			} else {
				roomId = &tmpRoomId
			}
		}
	} else {
		contactUserIds := evt.Content.AsMessage().Mentions.UserIDs
		if len(contactUserIds) != 1 {
			slog.Debug("SendMessage response received", "#ids", len(contactUserIds), "message", evt.Content.AsMessage().Body)
			return nil, fmt.Errorf("Not 1 contact found, found %d", len(contactUserIds))
		}
		err := contacts.CreateContact(client, *receiver, &contactUserIds[0])
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		roomId, err = rooms.ExtractMatrixRoomID(evt.Content.AsMessage().Body)
		if err != nil {
			tmpRoomId, err := utils.ExtractMatrixRoomID(evt.Content.AsMessage().Body)
			if err != nil {
				slog.Debug("SendMessage room failed extraction", "message", evt.Content.AsMessage().Body)
				return nil, err
			}

			newRoomId := id.RoomID(fmt.Sprintf("%s:%s", tmpRoomId, homeserver))
			roomId = &newRoomId
		}
	}

	return roomId, nil
}

func onDatabaseChangeDaemon() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Write {
					if strings.HasSuffix(event.Name, "clients.db") {
						go syncAll("onDatabaseChangeDaemon")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error(err.Error())
				debug.PrintStack()
			}
		}
	}()

	// Add a path to watch (e.g., a directory)
	err = watcher.Add("./db") // Use your desired path
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	// Keep the program running
	<-done
	return nil
}

func syncAll(source string) error {
	slog.Debug("Syncing all users", "source", source)
	fetchedUsers, err := users.FetchAllUsers()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	// slog.Debug("Syncing All", "#users", len(fetchedUsers))

	ignoreBotMessage := true
	for _, user := range fetchedUsers {
		syncers.RegisterSyncMessageListener(&syncers.SyncEventCallback{
			Callback: func(evt *event.Event, user *users.Users) error {
				slog.Debug("Sync responding event", "type", evt.Type, "user", user.Client.UserID)
				/**
				Checks if rooms have the neccessary Ids
				**/
				go func() {
					ok, err := rooms.Find(user.Client, evt.RoomID.String())
					if err != nil {
						slog.Error(err.Error())
					}

					if !ok {
						bridgeCfg, err := bridges.GetBridgeFromRoom(user.Client, &evt.RoomID)
						if err != nil {
							slog.Error(err.Error())
							return
						}
						if bridgeCfg != nil {
							if ignoreBotMessage {
								if bridgeCfg.BotName == evt.Sender.String() {
									slog.Debug("GetBridgeRoom", "reason", "ignoring")
									return
								}
							}
							botUsername := id.UserID(bridgeCfg.BotName)
							roomId, err := bridges.GetBotManagementRoom(user.Client, &botUsername)
							if err != nil {
								slog.Error(err.Error())
								return
							}
							if roomId.String() != evt.RoomID.String() {
								err = bridges.GetId(user.Client, bridgeCfg, &evt.RoomID)
								if err != nil {
									slog.Error(err.Error())
								}
							}
						}
					}
				}()

				/**
				Bridges listener, responsible for outgoing messages
				**/
				go func() {
					err = bridges.SyncCallback(user.Client, evt)
					if err != nil {
						slog.Error(err.Error())
						debug.PrintStack()
					}
				}()

				/**
				Contact listener, responsible for incoming messages
				**/
				go func() {
					err = contacts.SyncCallback(user.Client, evt)
					if err != nil {
						slog.Error(err.Error())
						debug.PrintStack()
					}
				}()

				return nil
			},
			ID:        user.Client.UserID.String(),
			IgnoreBot: ignoreBotMessage,
		})
		err := syncWatcher.Add(user)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	}
	return nil
}

func getRoomInfo(
	client *mautrix.Client,
	bridgeName,
	deviceId,
	groupUrl,
	receiver string,
) (*id.RoomID, *configs.BridgeConfig, error) {
	bridgeCfg, err := configs.GetBridgeConfig(bridgeName)
	if err != nil && err != sql.ErrNoRows {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	entityForSearch := receiver
	if groupUrl != "" {
		groupUrl = strings.ReplaceAll(groupUrl, `\?`, "?")
		u, err := url.Parse(groupUrl)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return nil, nil, err
		}

		u.RawQuery = ""
		entityForSearch = u.String()
	}

	cfg, err := configs.GetConf()
	if err != nil {
		debug.PrintStack()
		return nil, nil, err
	}
	roomId, err := noisyRoomIdRequest(
		client,
		bridgeCfg,
		entityForSearch,
		deviceId,
		cfg.HomeServerDomain,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, nil, err
	}

	return roomId, bridgeCfg, nil
}
