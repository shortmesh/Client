package contacts

import (
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

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
