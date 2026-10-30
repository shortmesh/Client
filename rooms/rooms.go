package rooms

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"runtime/debug"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type RoomType int

const (
	ManagementRoomType RoomType = iota
	ContactRoomType
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

func ExtractMatrixRoomID(text string) (*id.RoomID, error) {
	re := regexp.MustCompile(`!([\w-]+):[\w.-]+`)
	match := re.FindString(text)
	if match == "" {
		return nil, fmt.Errorf("no Matrix room ID found in text")
	}
	return (*id.RoomID)(&match), nil
}

func SaveBridgedId(client *mautrix.Client, bridgedId, roomId, bridgeName string) error {
	roomDb := RoomDB{
		Username: client.UserID.String(),
		Filepath: "db/" + client.UserID.String() + ".db",
	}

	err := roomDb.Init()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = roomDb.Insert(roomId, bridgedId, bridgeName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}

func Find(client *mautrix.Client, roomId string) (bool, error) {
	roomDb := RoomDB{
		Username: client.UserID.String(),
		Filepath: "db/" + client.UserID.String() + ".db",
	}

	err := roomDb.Init()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	_, err = roomDb.Find(roomId)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func GetBridgedId(client *mautrix.Client, bridgedId string) (string, error) {
	roomDb := RoomDB{
		Username: client.UserID.String(),
		Filepath: "db/" + client.UserID.String() + ".db",
	}

	err := roomDb.Init()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	roomId, err := roomDb.FindBridged(bridgedId)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Error(err.Error())
			debug.PrintStack()
			return "", err
		}
		return "", nil
	}
	return *roomId, nil
}

type RoomJoined struct {
	RoomId  string
	Name    string
	Topic   string
	Members map[id.UserID]mautrix.JoinedMember
}

type RoomInfo struct {
	Rooms []RoomJoined
}

func GetRoomMembers(
	client *mautrix.Client,
	roomId *id.RoomID,
) (map[id.UserID]mautrix.JoinedMember, error) {
	ctx := context.Background()
	members, err := client.JoinedMembers(ctx, *roomId)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return members.Joined, nil
}

func GetSumInfo(client *mautrix.Client) (*RoomInfo, error) {
	ctx := context.Background()
	rooms, err := client.JoinedRooms(ctx)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	var roomInfo []RoomJoined
	for _, id := range rooms.JoinedRooms {
		members, err := client.JoinedMembers(ctx, id)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		name, err := GetRoomName(client, &id)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		topic, err := GetRoomTopic(client, &id)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		roomInfo = append(roomInfo, RoomJoined{
			Name:    name,
			Topic:   topic,
			RoomId:  id.String(),
			Members: members.Joined,
		})
	}

	return &RoomInfo{Rooms: roomInfo}, nil
}
