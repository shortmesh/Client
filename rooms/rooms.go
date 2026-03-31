package rooms

import (
	"context"
	"database/sql"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
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
	for userId := range members.Joined {
		membersList = append(membersList, userId)
	}

	return membersList, nil
}

func IsManagementRoom(client *mautrix.Client, roomId id.RoomID, botName string) (bool, error) {
	members, err := client.JoinedMembers(context.Background(), roomId)
	if err != nil {
		return false, err
	}

	isSpace, err := isSpaceRoom(client, roomId)
	if err != nil {
		return false, err
	}

	if !isSpace {
		if len(members.Joined) == 2 {
			botID := id.UserID(botName)
			if _, ok := members.Joined[botID]; ok {
				if _, ok := members.Joined[client.UserID]; ok {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// IsSpaceRoom checks if the given room is a space
func isSpaceRoom(client *mautrix.Client, roomId id.RoomID) (bool, error) {
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

func (r *Rooms) GetRoomInfo() (string, error) {
	// Get room name
	var nameContent event.RoomNameEventContent
	err := r.Client.StateEvent(context.Background(), *r.ID, event.StateRoomName, "", &nameContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
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

func (r *Rooms) Delete() error {
	roomsDb, err := GetRoomDb(r.Client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if err := roomsDb.Delete(r.ID.String()); err != nil {
		slog.Error(err.Error())
		return err
	}
	slog.Debug("Rooms Delete", "name", r.ID)

	return nil
}

func (r *Rooms) Save(
	bridgeName,
	contactName,
	deviceId string,
	isBridgeBot bool,
) error {
	slog.Debug("Saving room", "bridgeName", bridgeName, "contactName", contactName, "deviceId", deviceId, "isBridgeBot", isBridgeBot)
	roomsDb, err := GetRoomDb(r.Client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if err := roomsDb.Save(r.ID.String(), bridgeName, contactName, deviceId, isBridgeBot); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func IsContactRoom(client *mautrix.Client, members []id.UserID) (bool, error) {
	if len(members) < 3 {
		return false, nil
	}

	// ? contact communication room?
	// ? client, bridgeBot, device, some contact
	isUser := false
	isBridgeBot := false
	// isDevice := false
	isContact := false
	for _, member := range members {
		userType, err := GetTypeUser(client, member)
		if err != nil {
			return false, err
		}
		// slog.Debug("User type", "type", userType, "member", member)
		if userType == -1 {
			continue
		}
		switch userType {
		case users.User:
			isUser = true
		case users.BridgeBot:
			isBridgeBot = true
		// case users.Device:
		// 	isDevice = true
		case users.Contact:
			isContact = true
		}
	}

	slog.Debug(
		"IsContactRoom check",
		"isUser", isUser,
		"isBridgeBot", isBridgeBot,
		"isContact", isContact,
		"userId", members,
	)

	return isUser && isBridgeBot && isContact, nil
}

func ProcessIsContactRoom(client *mautrix.Client, room Rooms, members []id.UserID) error {
	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	var bridgeName string
	var contactName id.UserID
	var deviceName id.UserID
	for _, member := range members {
		userType, err := GetTypeUser(client, member)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		switch userType {
		case users.BridgeBot:
			for _, bridgeConf := range cfg.Bridges {
				if bridgeConf.BotName == member.String() {
					bridgeName = bridgeConf.Name
					break
				}
			}
		case users.Contact:
			contactName = member
		case users.Device:
			deviceName = member
		}
	}

	bridgeNameIsContactsName := false
	for _, bridgeConf := range cfg.Bridges {
		if bridgeName == bridgeConf.Name {
			bridgeNameIsContactsName = bridgeConf.BridgeNameIsContactName
			break
		}
	}

	contactNameStr := contactName.String()
	if bridgeNameIsContactsName {
		displayName, err := client.GetDisplayName(context.Background(), id.UserID(contactName))
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		contactNameStr, err = cfg.FormatUsername(bridgeName, displayName.DisplayName)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
	}

	err = room.Save(bridgeName, contactNameStr, deviceName.String(), false)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	slog.Debug("Room saved", "bridge_name", bridgeName, "contact_name", contactName, "device_name", deviceName)
	return nil
}

func (r *Rooms) GetRoomName() (string, error) {
	var nameContent event.RoomNameEventContent
	err := r.Client.StateEvent(context.Background(), *r.ID, event.StateRoomName, "", &nameContent)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}
	return nameContent.Name, nil
}

func (r *Rooms) SendMessage(message string) error {
	ctx := context.Background()
	content := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}
	_, err := r.Client.SendMessageEvent(ctx, *r.ID, event.EventMessage, content)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func GetTypeUser(client *mautrix.Client, userId id.UserID) (users.UserType, error) {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return -1, err
	}

	if userId == client.UserID {
		return users.User, nil
	}

	for _, bridgeConf := range conf.Bridges {
		if userId == id.UserID(bridgeConf.BotName) {
			return users.BridgeBot, nil
		}
	}

	// extract from template
	deviceId := ""
	fUserId := strings.Split(userId.Localpart(), "_")
	if len(fUserId) > 1 {
		deviceId = fUserId[1]
	}
	isDevice, err := devices.IsDevice(client, deviceId)

	if err != nil {
		return -1, err
	}

	if isDevice {
		return users.Device, nil
	}

	isContact, err := isContact(client, userId.String())
	if err != nil {
		return -1, err
	}

	if isContact {
		return users.Contact, nil
	}

	return -1, nil
}

func isContact(
	client *mautrix.Client,
	contact string,
) (bool, error) {
	roomDb, err := GetRoomDb(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	roomId, err := roomDb.fetchIsContact(contact)

	if err != nil {
		if err == sql.ErrNoRows {
			cfg, err := configs.GetConf()
			if err != nil {
				slog.Error(err.Error())
				debug.PrintStack()
				return false, err
			}

			for _, bridgeConf := range cfg.Bridges {
				matched, err := cfg.CheckUserBridgeBotTemplate(bridgeConf.Name, contact)
				if err != nil {
					slog.Error(err.Error())
					debug.PrintStack()
					return false, err
				}

				if matched {
					return true, err
				}
			}
			return false, nil
		}
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	if roomId == nil {
		return false, nil
	}

	return true, nil
}
