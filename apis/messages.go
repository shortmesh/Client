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

// ClientJsonRequest represents login or registration data
// @Description Request payload for user login or registration
// @name ClientJsonRequest
// @type object
type DeviceSendTextMessage struct {
	Username     string `json:"username" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
	DeviceId     string `json:"device_id" example:"john_doe"`
	Contact      string `json:"contact" example:"john_doe"`
	Text         string `json:"text" example:"john_doe"`
}

// ApiLogin godoc
// @Summary Logs a user into the Matrix server
// @Description Authenticates a user and returns an access token
// @Accept  json
// @Produce  json
// @Param   payload body ClientRemoveDevices true "Login Credentials"
// @Success 200 {object} LoginResponse "Successfully logged in"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Login failed"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /devices [delete]
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

	user, err := users.FetchUser(client, username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Something wasn't found"})
		return
	}
	client.AccessToken = user.Client.AccessToken

	_, err = (&cmd.Controller{
		Client: client,
	}).SendMessage(
		deviceSendTextMessage.PlatformName,
		deviceSendTextMessage.DeviceId,
		deviceSendTextMessage.Contact,
		deviceSendTextMessage.Text,
	)
	c.JSON(http.StatusCreated, gin.H{"status": "Message sent!"})

}
