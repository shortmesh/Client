package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"

	"github.com/shortmesh/core/rooms"
	"github.com/shortmesh/core/users"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
)

type MatrixClient struct {
	Client       *mautrix.Client
	CryptoHelper *cryptohelper.CryptoHelper
}

func (m *MatrixClient) Login(password string) error {
	identifier := mautrix.UserIdentifier{
		Type: mautrix.IdentifierTypeUser,
		User: m.Client.UserID.String(),
	}

	resp, err := m.Client.Login(context.Background(), &mautrix.ReqLogin{
		Type:             mautrix.AuthTypePassword,
		Identifier:       identifier,
		Password:         password,
		StoreCredentials: true,
	})
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	m.Client.AccessToken = resp.AccessToken
	m.Client.DeviceID = resp.DeviceID

	return nil
}

func Logout(client *mautrix.Client) error {
	// Logout from the session
	_, err := client.Logout(context.Background())
	if err != nil {
		log.Printf("Logout failed: %v\n", err)
	}

	// TODO: delete the session file

	fmt.Println("Logout successful.")
	return err
}

func SetupCryptoHelper(client *mautrix.Client, pickleKey []byte) (*cryptohelper.CryptoHelper, error) {
	dbPath := fmt.Sprintf("db/%s-crypto.db", client.UserID.Localpart()) // this path needs to change for each user

	helper, err := cryptohelper.NewCryptoHelper(client, pickleKey, dbPath)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	// initialize the database and other stuff
	err = helper.Init(context.Background())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return nil, err
	}

	return helper, nil
}

func (m *MatrixClient) Sync(user users.Users, ch chan *event.Event) error {
	syncer := mautrix.NewDefaultSyncer()
	m.Client.Syncer = syncer
	machine := m.CryptoHelper.Machine()

	syncer.OnEventType(event.EventEncrypted, func(ctx context.Context, evt *event.Event) {
		evt, err := m.Client.Crypto.Decrypt(ctx, evt)
		if err != nil {
			slog.Error(err.Error())
			// debug.PrintStack()
		}
		ch <- evt
	})

	syncer.OnEvent(func(ctx context.Context, evt *event.Event) {
		room := rooms.Rooms{
			Client: m.Client,
			ID:     &evt.RoomID,
		}
		if evt.Type.Class == event.ToDeviceEventType {
			machine.HandleToDeviceEvent(ctx, evt)
		} else if evt.Content.AsMember().Membership == event.MembershipInvite {
			err := getInvites(m.Client, evt)
			if err != nil {
				slog.Error(err.Error())
			}
		} else if evt.Content.AsMember().Membership == event.MembershipLeave {
			err := room.Delete()
			if err != nil {
				slog.Error(err.Error())
			}
		}
		// else if evt.Content.AsMember().Membership == event.MembershipJoin {
		// 	slog.Debug("Sync event", "type", "new user membership")
		// 	if evt.Content.AsMember().
		// 	if err != nil {
		// 		slog.Error(err.Error())
		// 		debug.PrintStack()
		// 	}
		// }
	})

	if err := m.Client.Sync(); err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	return nil
}

func GenerateAndUploadClientKeys(cryptoHelper *cryptohelper.CryptoHelper) (string, error) {
	ctx := context.Background()
	machine := cryptoHelper.Machine()

	// !user should send a passphrase
	passphrase := "f.society"
	err := machine.ShareKeys(ctx, 1)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	key, err := machine.SSSS.GenerateAndUploadKey(ctx, passphrase)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	// conf, err := cfg.getConf()
	log.Println("[+] Verifying machine access token: ", cryptoHelper.Machine().Client.AccessToken)
	uiaCallback := func(flows *mautrix.RespUserInteractive) interface{} {
		log.Printf("UIA flows available: %+v", flows)

		// Try using the access token directly
		return map[string]interface{}{
			"type":    mautrix.AuthTypeToken,
			"session": flows.Session,
			"token":   machine.Client.AccessToken,
		}
	}
	_, _, err = machine.GenerateAndUploadCrossSigningKeys(ctx, uiaCallback, passphrase)
	if err != nil {
		// If it still fails, the account data on the server is likely corrupted.
		// You may need to manually reset the account's cross-signing via a client like Element.
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	err = machine.SSSS.SetDefaultKeyID(ctx, key.ID)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	err = machine.SignOwnDevice(ctx, machine.OwnIdentity())
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	err = machine.SignOwnMasterKey(ctx)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return "", err
	}

	return key.RecoveryKey(), nil
}

func getInvites(client *mautrix.Client, evt *event.Event) error {
	_, err := client.JoinRoomByID(context.Background(), evt.RoomID)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}

	err = ParseRoomSubroutine(client)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return err
	}
	return nil
}
