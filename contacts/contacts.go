package contacts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/creasty/defaults"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/rabbitmq"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type RMQExchanges struct {
	IncomingMessage string `default:"contacts.topic"`
}

type RMQBindingKeys struct {
	IncomingMessage string `default:"contacts.topic.incoming_message"`
}

type Contacts struct {
	Name     string
	Username *id.UserID
}

func CreateContact(client *mautrix.Client, name string, username *id.UserID) error {
	contactsDb, err := getDb(client)
	if err != nil {
		return err
	}

	err = contactsDb.insert(name, username.String())
	if err != nil {
		return err
	}

	return nil
}

func FetchContact(client *mautrix.Client, username *id.UserID) (*Contacts, error) {
	contactsDb, err := getDb(client)
	if err != nil {
		return nil, err
	}

	names, err := contactsDb.fetchName(username.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if len(*names) < 1 {
		return nil, nil
	}

	if len(*names) > 1 {
		return nil, fmt.Errorf("More than 1 name found for contact: %d found", len(*names))
	}

	name := (*names)[0]

	return &Contacts{Name: name, Username: username}, nil
}

func findBot(client *mautrix.Client, roomId *id.RoomID) (*configs.BridgeConfig, error) {
	resp, err := client.JoinedMembers(context.Background(), *roomId)
	if err != nil {
		debug.PrintStack()
		return nil, err
	}

	for member := range resp.Joined {
		bridgeCfg, err := configs.GetBridgeConfigByBotname(member.String())
		if err != nil {
			continue
		}

		if bridgeCfg != nil {
			return bridgeCfg, nil
		}
	}

	return nil, nil
}

func SyncCallback(client *mautrix.Client, evt *event.Event) error {
	bridgeCfg, err := findBot(client, &evt.RoomID)
	if err != nil {
		debug.PrintStack()
		return err
	}
	if bridgeCfg == nil {
		return nil
	}

	// ignore bot messages
	if bridgeCfg.BotName == evt.Sender.String() {
		slog.Info("Incoming message", "status", "ignoring", "reason", "bot", "botName", evt.Sender)
		return nil
	}

	// ignore if device
	isBridgeUser, err := configs.CheckUserBridgeBotTemplate(*bridgeCfg, evt.Sender.String())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	if isBridgeUser {
		possibleUsername, err := configs.ExtractComponentByTemplates(
			bridgeCfg.UsernameTemplate,
			evt.Sender.Localpart(),
		)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		ok, err := devices.IsDevice(client, possibleUsername)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		}
		if ok {
			slog.Info("Incoming message", "status", "ignoring", "reason", "device", "devicveId", evt.Sender)
			return nil
		}
	}

	username := evt.Sender
	contact, err := isContactRoom(client, &username)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	payload, err := getPayload(client, evt, contact)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	if payload == nil {
		return nil
	}

	exchange := RMQExchanges{}
	defaults.Set(&exchange)

	bindingKey := RMQBindingKeys{}
	defaults.Set(&bindingKey)

	slog.Debug("Contact message", "payload", payload)

	queueName := client.UserID.String() + "_incoming_messages"
	err = rabbitmq.Sender(
		client,
		*payload,
		exchange.IncomingMessage,
		bindingKey.IncomingMessage,
		queueName,
	)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	return nil
}

type IncomingMessagePayloadMediaInfo struct {
	Size     float64
	MimeType string
	Width    int
	Height   int
	BlurHash string
}
type IncomingMessagePayloadMedia struct {
	Content []byte
	Info    IncomingMessagePayloadMediaInfo
}

type IncomingMessagePayload struct {
	IsContact bool
	Type      string
	From      string
	To        string
	Message   string
	Media     IncomingMessagePayloadMedia
}

func getPayload(client *mautrix.Client, evt *event.Event, contact *Contacts) (*string, error) {
	from := evt.Sender.String()
	if contact != nil {
		from = contact.Name
	}

	message := evt.Content.AsMessage()
	incomingMessagePayload := IncomingMessagePayload{
		IsContact: contact != nil,
		Type:      string(message.MsgType),
		From:      from,
		To:        client.UserID.String(),
		Message:   evt.Content.AsMessage().Body,
	}

	if message.GetFile() != nil {
		payloadBytes, err := downloadContent(client, evt)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		incomingMessagePayload.Media = IncomingMessagePayloadMedia{
			Content: payloadBytes,
			Info: IncomingMessagePayloadMediaInfo{
				Size:     float64(message.Info.Size),
				MimeType: message.Info.MimeType,
				Width:    message.Info.Width,
				Height:   message.Info.Height,
				BlurHash: message.Info.Blurhash,
			},
		}
	}

	payloadBytes, err := json.Marshal(incomingMessagePayload)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	payload := string(payloadBytes)
	return &payload, nil
}

func isContactRoom(client *mautrix.Client, username *id.UserID) (*Contacts, error) {
	contacts, err := FetchContact(client, username)
	if err != nil {
		debug.PrintStack()
		return nil, err
	}

	return contacts, nil
}

func downloadContent(client *mautrix.Client, evt *event.Event) ([]byte, error) {
	url := string(evt.Content.AsMessage().File.URL)
	slog.Debug("Downloading image", "url", url)

	contentUrl, err := id.ParseContentURI(url)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}
	return client.DownloadBytes(context.Background(), contentUrl)
}
