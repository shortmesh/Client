package apis

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ClientRemoveDevices represents login or registration data
// @Description Request payload for user login or registration
// @name ClientRemoveDevices
// @type object
type GroupInfoRequest struct {
	Username string `json:"username" example:"john_doe"`
	DeviceId string `json:"device_id" example:"john_doe"`
	GroupUrl string `json:"group_url" example:"john_doe"`
}

// GetDevices godoc
// @Summary Fetches all active devices for the user
// @Description Retrieves device information for a user
// @Accept  json
// @Produce  json
// @Param   payload body ClientGetDevices true "User Credentials"
// @Success 200 {object} []devices.Devices "Successfully retrieved devices"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /devices [get]
func GetGroupInfo(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var groupInfoRequest GroupInfoRequest

	if err := c.BindJSON(&groupInfoRequest); err != nil {
		slog.Error("Invalid request payload", "payload", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	username, err := utils.SanitizeUsername(groupInfoRequest.Username)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		"",
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}
	slog.Debug("Fetching devices", "username", username)

	devices := make([]devices.Devices, 0)
	fetchDevices, err := (&cmd.Controller{
		Client: client,
	}).GetGroupInfo()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	if fetchDevices != nil {
		devices = fetchDevices
	}

	c.IndentedJSON(http.StatusOK, devices)
}
