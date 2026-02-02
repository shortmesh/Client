package rooms

import (
	"context"
	"log"
	"log/slog"
	"runtime/debug"

	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Rooms struct {
	Client   *mautrix.Client
	ID       *id.RoomID
	IsBridge bool
	Members  map[string]string
}

func (r *Rooms) GetRoomMembers() ([]id.UserID, error) {
	members, err := r.Client.JoinedMembers(context.Background(), *r.ID)

	if err != nil {
		return nil, err
	}

	var membersList []id.UserID
	for userId, _ := range members.Joined {
		membersList = append(membersList, userId)
	}

	return membersList, nil
}

func (r *Rooms) IsManagementRoom(botName string) (bool, error) {
	members, err := r.Client.JoinedMembers(context.Background(), *r.ID)
	if err != nil {
		return false, err
	}

	isSpace, err := r.IsSpaceRoom()
	if err != nil {
		return false, err
	}

	if !isSpace {
		if len(members.Joined) == 2 {
			botID := id.UserID(botName)
			if _, ok := members.Joined[botID]; ok {
				if _, ok := members.Joined[r.Client.UserID]; ok {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (r *Rooms) GetRoomInfo() (string, error) {
	// Get room name
	var nameContent event.RoomNameEventContent
	err := r.Client.StateEvent(context.Background(), *r.ID, event.StateRoomName, "", &nameContent)
	if err != nil {
		return "", err
	}

	return nameContent.Name, nil
}

func (r *Rooms) GetPowerLevelsUser() (int, error) {
	var powerLevels event.PowerLevelsEventContent
	err := r.Client.StateEvent(context.Background(), *r.ID, event.StatePowerLevels, "", &powerLevels)
	if err != nil {
		return -1, err
	}
	return powerLevels.Users[r.Client.UserID], nil
}

func (r *Rooms) GetPowerLevelsEvents() (int, error) {
	var powerLevels event.PowerLevelsEventContent
	err := r.Client.StateEvent(context.Background(), *r.ID, event.StatePowerLevels, "", &powerLevels)
	if err != nil {
		return -1, err
	}
	return powerLevels.Events[event.EventMessage.String()], nil
}

// IsSpaceRoom checks if the given room is a space
func (r *Rooms) IsSpaceRoom() (bool, error) {
	var createContent event.CreateEventContent

	err := r.Client.StateEvent(context.Background(), *r.ID, event.StateCreate, "", &createContent)
	if err != nil {
		return false, err
	}

	// Check if "type" field is set to "m.space"
	if createContent.Type == "m.space" {
		return true, nil
	}
	return false, nil
}

func (r *Rooms) GetInvites(
	evt *event.Event,
) error {
	if evt.Content.AsMember().Membership == event.MembershipInvite {
		log.Println("Invite received for:", r.ID, evt.Content.AsMember().Membership)
		// if evt.StateKey != nil && *evt.StateKey == r.Client.UserID.String() {
		// 	_, err := r.Client.JoinRoomByID(context.Background(), r.ID)
		// 	if err != nil {
		// 		return err
		// 	}
		// }

		_, err := r.Client.JoinRoomByID(context.Background(), *r.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Rooms) JoinRoom(invites []id.UserID) (id.RoomID, error) {
	resp, err := r.Client.CreateRoom(context.Background(), &mautrix.ReqCreateRoom{
		Invite:   invites,
		IsDirect: true,
		// Preset:     "private_chat",
		Preset:     "trusted_private_chat",
		Visibility: "private",
	})
	if err != nil {
		return "", err
	}

	// * Begins encryption
	_, err = r.Client.SendStateEvent(
		context.Background(),
		resp.RoomID,
		event.StateEncryption,
		"",
		&event.EncryptionEventContent{
			Algorithm: id.AlgorithmMegolmV1, // "m.megolm.v1.aes-sha2"
		},
	)

	if err != nil {
		return "", err
	}

	r.ID = &resp.RoomID
	return resp.RoomID, nil
}

func GetRoomDb(client *mautrix.Client) (*RoomsDB, error) {
	roomDb := RoomsDB{
		Username: client.UserID.Localpart(),
		Filepath: "db/" + client.UserID.Localpart() + ".db",
	}

	err := roomDb.Init()
	if err != nil {
		return nil, err
	}

	return &roomDb, nil
}

func (r *Rooms) Save(
	bridgeName,
	contactName,
	deviceId string,
) error {
	roomsDb, err := GetRoomDb(r.Client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if err := roomsDb.Save(r.ID.String(), bridgeName, contactName, deviceId, false); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func isContactRoom(client *mautrix.Client, roomId *id.RoomID) (bool, error) {
	room := Rooms{
		Client: client,
		ID:     roomId,
	}
	members, err := room.GetRoomMembers()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	slog.Debug("User details", "members_in_room", len(members))

	if len(members) == 4 {
		// ? contact communication room?
		// ? client, bridgeBot, device, some contact
		isUser := false
		isBridgeBot := false
		isDevice := false
		isContact := false
		for _, member := range members {
			userType, err := users.GetTypeUser(client, member)
			if err != nil {
				return false, err
			}
			slog.Debug("User type", "type", userType, "member", member)
			if userType == -1 {
				continue
			}
			switch userType {
			case users.User:
				isUser = true
			case users.BridgeBot:
				isBridgeBot = true
			case users.Device:
				isDevice = true
			case users.Contact:
				isContact = true
			}
		}

		if isUser && isBridgeBot && isDevice && isContact {
			return true, nil
		}
	}
	return false, nil
}

func isBridgeBotRoom(client *mautrix.Client, roomId *id.RoomID) (bool, error) {
	room := Rooms{
		Client: client,
		ID:     roomId,
	}
	members, err := room.GetRoomMembers()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	slog.Debug("User details", "members_in_room", len(members))

	if len(members) == 2 {
		isUser := false
		isBridgeBot := false
		for _, member := range members {
			userType, err := users.GetTypeUser(client, member)
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return false, err
			}
			switch userType {
			case users.User:
				isUser = true
			case users.BridgeBot:
				isBridgeBot = true
			}
		}

		if isUser && isBridgeBot {
			return true, nil
		}
	}
	return false, nil
}

func ParseRoomSubroutine(client *mautrix.Client) error {
	ctx := context.Background()
	rooms, err := client.JoinedRooms(ctx)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("User details", "num_rooms", len(rooms.JoinedRooms))

	for _, roomId := range rooms.JoinedRooms {
		isContactRoom, err := isContactRoom(client, &roomId)
		if err != nil {
			return err
		}

		// TODO: get bridge name
		if isContactRoom {
			slog.Debug("Found contact room", "roomId", roomId)
			room := Rooms{
				Client: client,
				ID:     &roomId,
			}
			members, err := room.GetRoomMembers()

			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}

			var bridgeName id.UserID
			var contactName id.UserID
			var deviceName id.UserID
			for _, member := range members {
				userType, err := users.GetTypeUser(client, member)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return err
				}
				switch userType {
				case users.BridgeBot:
					bridgeName = member
				case users.Contact:
					contactName = member
				case users.Device:
					deviceName = member
				}
			}

			err = room.Save(bridgeName.String(), contactName.String(), deviceName.String())
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return err
			}
			slog.Debug("Room parsed and saved", "BridgeName", bridgeName, "ContactName", contactName, "deviceName", deviceName)
		}

		isBridgeBotRoom, err := isBridgeBotRoom(client, &roomId)
		if err != nil {
			return err
		}

		if isBridgeBotRoom {
			// TODO
		}
	}

	return nil
}
