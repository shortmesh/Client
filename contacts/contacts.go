package contacts

import "maunium.net/go/mautrix/id"

type Contact struct {
	Name     string
	Username *id.UserID
}

type Contacts struct {
	DbFilename string
}

func (c *Contacts) Save(name, username string) error {
	contactDb, err := getContactDb(c.DbFilename)
	if err != nil {
		return err
	}

	defer contactDb.connection.Close()

	err = contactDb.save(name, username)
	if err != nil {
		return err
	}
	return nil
}

func (c *Contacts) Find(name string) (*Contact, error) {
	contactDb, err := getContactDb(c.DbFilename)
	if err != nil {
		return nil, err
	}
	defer contactDb.connection.Close()

	username, err := contactDb.find(name)
	if err != nil {
		return nil, err
	}

	return &Contact{Name: name, Username: (*id.UserID)(username)}, nil
}
