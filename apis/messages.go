package apis

import (
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// DeviceSendTextMessage represents payload for sending a text message
// @Description Request payload for sending a text message
// @name DeviceSendTextMessage
// @type object
type DeviceSendTextMessage struct {
	Username     string `json:"username" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
	Contact      string `json:"contact" example:"john_doe"`
	Text         string `json:"text" example:"john_doe"`
}

// SendMessage godoc
// @Summary Sends a text message via bridge device
// @Description Sends a text message using the provided credentials and bridge details
// @Accept  json
// @Produce  json
// @Param   payload body DeviceSendTextMessage true "Send Messages"
// @Success 201 {object} map[string]string "Message sent successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /devices/{deviceId}/message [post]
func SendMessage(c *gin.Context) {

	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var deviceSendTextMessage DeviceSendTextMessage

	if err := c.BindJSON(&deviceSendTextMessage); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	username, err := utils.SanitizeUsername(deviceSendTextMessage.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		"",
	)

	user, err := users.FetchUser(client)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Something wasn't found"})
		return
	}
	client.AccessToken = user.Client.AccessToken
	deviceId := c.Param("deviceId")

	_, err = (&cmd.Controller{
		Client: client,
	}).SendMessage(
		deviceSendTextMessage.PlatformName,
		deviceId,
		deviceSendTextMessage.Contact,
		deviceSendTextMessage.Text,
	)
	c.JSON(http.StatusCreated, gin.H{"status": "Message sent!"})

}
