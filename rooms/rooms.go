package rooms

import (
	"context"
	"log"
	"log/slog"
	"runtime/debug"

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

// func (r *Rooms) IsBridgeInviteForContact(evt *event.Event) (bool, error) {
// 	// TODO: check if the invite is from a bridge bot but not a bridge room
// 	for _, bridge := range cfg.Bridges {
// 		if bridge.BotName == evt.Sender.String() {
// 			isBridge, err := r.IsBridgeMessage(evt)
// 			if err != nil {
// 				return false, err
// 			}
// 			return !isBridge, nil
// 		}
// 	}

// 	return false, nil
// }

// func (r *Rooms) IsBridgeMessage(evt *event.Event) (bool, error) {
// 	if evt.Type == event.EventMessage {
// 		var UsersDB UsersDB = UsersDB{
// 			username: r.Client.UserID.Localpart(),
// 			filepath: "db/" + r.Client.UserID.Localpart() + ".db",
// 		}

// 		UsersDB.Init()
// 		defer UsersDB.Close()

// 		room, err := UsersDB.FetchRooms(evt.RoomID.String())

// 		if err != nil {
// 			return false, err
// 		}

// 		return room.isBridge, nil
// 	}
// 	return false, nil
// }

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

func (r *Rooms) Save(
	bridgeName,
	contactName,
	deviceId string,
) error {
	roomsDb := GetRoomDb(r.Client)

	if err := roomsDb.Init(); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if err := roomsDb.StoreRoom(r.ID.String(), bridgeName, contactName, deviceId, false); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func parseRooms(client *mautrix.Client) {
	// ? for each room
	// ?   read all room patterns
	// ?   add important room to db. (bridge_bot, contact_messaging)
	ctx := context.Background()
	rooms, err := client.JoinedRooms(ctx)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return
	}

	slog.Debug("User details", "num_rooms", len(rooms.JoinedRooms))

	for _, roomId := range rooms.JoinedRooms {
		room := Rooms{
			Client: client,
			ID:     &roomId,
		}
		members, err := room.GetRoomMembers()

		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return
		}

		slog.Debug("User details", "members_in_room", len(members))

		if len(members) == 4 {
			// ? contact communication room?
			// ? client, bridgeBot, device, some contact
		} else if len(members) == 2 {
			// ? bridge bot room?
		}
	}
}
