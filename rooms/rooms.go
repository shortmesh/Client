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

func GetRoomTopic(client *mautrix.Client, roomId *id.RoomID) (string, error) {
	var topicContent event.TopicEventContent
	err := client.StateEvent(context.Background(), *roomId, event.StateTopic, "", &topicContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}
	return topicContent.Topic, nil
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
