package contacts

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/creasty/defaults"
	"github.com/shortmesh/core/configs"
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
	slog.Debug("Contact message", "sender", evt.Sender)

	bridgeCfg, err := findBot(client, &evt.RoomID)
	// bridgeCfg, err := configs.GetBridgeConfigByBotname(evt.Sender.String())
	if err != nil {
		debug.PrintStack()
		return err
	}
	if bridgeCfg == nil {
		return nil
	}

	username := evt.Sender
	ok, err := isContactRoom(client, &username)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	if !ok {
		slog.Error("Contact message - not contact", "sender", evt.Sender, "msg", evt.Content.AsMessage().Body)
		return nil
	}

	message := evt.Content.AsMessage().Body
	exchange := RMQExchanges{}
	defaults.Set(&exchange)

	bindingKey := RMQBindingKeys{}
	defaults.Set(&bindingKey)

	slog.Debug("Contact message", "msg", message)

	err = rabbitmq.Sender(
		client,
		message,
		exchange.IncomingMessage,
		bindingKey.IncomingMessage,
	)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	return nil

}

func isContactRoom(client *mautrix.Client, username *id.UserID) (bool, error) {
	contacts, err := FetchContact(client, username)
	if err != nil {
		debug.PrintStack()
		return false, err
	}

	if contacts == nil {
		return false, nil
	}
	return true, nil
}
