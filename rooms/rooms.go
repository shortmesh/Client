package rooms

import (
	"context"
	"log/slog"
	"runtime/debug"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Rooms struct {
	Client     *mautrix.Client
	DbFilename string
}

func IsManagementRoom(client *mautrix.Client, roomId id.RoomID, botUsername id.UserID) (bool, error) {
	members, err := client.JoinedMembers(context.Background(), roomId)
	if err != nil {
		return false, err
	}

	isSpace, err := IsSpaceRoom(client, roomId)
	if err != nil {
		return false, err
	}

	if !isSpace {
		if len(members.Joined) == 2 {
			if _, ok := members.Joined[botUsername]; ok {
				if _, ok := members.Joined[client.UserID]; ok {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func IsSpaceRoom(client *mautrix.Client, roomId id.RoomID) (bool, error) {
	var createContent event.CreateEventContent

	err := client.StateEvent(context.Background(), roomId, event.StateCreate, "", &createContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	// Check if "type" field is set to "m.space"
	if createContent.Type == "m.space" {
		return true, nil
	}
	return false, nil
}

func (r *Rooms) CreateRoom(invites []id.UserID, isManagementRoom bool) (id.RoomID, error) {
	resp, err := r.Client.CreateRoom(context.Background(), &mautrix.ReqCreateRoom{
		Invite:   invites,
		IsDirect: true,
		// Preset:     "private_chat",
		Preset:     "trusted_private_chat",
		Visibility: "private",
	})
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	roomId := resp.RoomID
	// * Begins encryption
	_, err = r.Client.SendStateEvent(
		context.Background(),
		roomId,
		event.StateEncryption,
		"",
		&event.EncryptionEventContent{
			Algorithm: id.AlgorithmMegolmV1, // "m.megolm.v1.aes-sha2"
		},
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	return resp.RoomID, nil
}

func GetRoomDb(username string) (*roomsDb, error) {
	roomDb := roomsDb{
		Username: username,
		Filepath: "db/" + username + ".db",
	}

	err := roomDb.Init()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return &roomDb, nil
}

func Delete(roomId, username string) error {
	roomsDb, err := GetRoomDb(username)
	if err != nil {
		return err
	}

	defer roomsDb.connection.Close()

	if err := roomsDb.Delete(roomId); err != nil {
		return err
	}
	slog.Debug("[-] Rooms Delete", "name", roomId)

	return nil
}

func (r *Rooms) Save(
	roomId id.RoomID,
	username id.UserID,
) error {
	slog.Debug("[+] Saving room",
		"roomId", roomId,
		"username", username,
	)

	roomsDb, err := GetRoomDb(r.DbFilename)
	if err != nil {
		return err
	}
	defer roomsDb.connection.Close()

	if err := roomsDb.Save(roomId.String(), username.String()); err != nil {
		return err
	}

	return nil
}

func (r *Rooms) FindConversationRoom(username, botName id.UserID) (*id.RoomID, error) {
	roomsDb, err := GetRoomDb(r.DbFilename)
	if err != nil {
		return nil, err
	}
	defer roomsDb.connection.Close()

	roomIds, err := roomsDb.findUsernames([]string{username.String(), botName.String()})
	for _, roomId := range roomIds {
		joinedMembers, err := r.Client.JoinedMembers(context.Background(), id.RoomID(roomId))
		if err != nil {
			debug.PrintStack()
			return nil, err
		}
		members := joinedMembers.Joined
		if len(members) > 2 && len(members) < 5 {
			_roomId := id.RoomID(roomId)
			return &_roomId, nil
		}
	}
	return nil, nil
}

type RoomSaveData struct {
	User          id.UserID
	BridgeBot     id.UserID
	Contact       id.UserID
	ContactUserId id.UserID
}

func GetRoomName(client *mautrix.Client, roomId *id.RoomID) (string, error) {
	var nameContent event.RoomNameEventContent
	err := client.StateEvent(context.Background(), *roomId, event.StateRoomName, "", &nameContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}
	return nameContent.Name, nil
}

func SendMessage(client *mautrix.Client, roomId id.RoomID, message string) error {
	ctx := context.Background()
	content := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}
	_, err := client.SendMessageEvent(ctx, roomId, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}
