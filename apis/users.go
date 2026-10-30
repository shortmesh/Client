package apis

import (
	"fmt"
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

// ApiDeleteUserRequest represents User access token delete data
// @Description Request payload for deleting user data for syncing
// @name ApiDeleteUserRequest
// @type object
type ApiDeleteUserRequest struct {
	Username string `json:"username" example:"john_doe"`
}

// Store godoc
// @Summary Delete user credentials for syncing
// @Description Delete the user's access token to be used for Syncing
// @Accept  json
// @Produce  json
// @Param   payload body ApiDeleteUserRequest true "Delete Credentials"
// @Success 200 {string} string "User deleted!" "Successfully deleted"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /users [delete]
func Delete(c *gin.Context) {
	conf, err := configs.GetConf()
	var apiDeleteUserRequest ApiDeleteUserRequest

	if err := c.BindJSON(&apiDeleteUserRequest); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	username, err := utils.SanitizeUsername(apiDeleteUserRequest.Username)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		"",
	)

	err = users.RemoveUser(client)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = utils.DeleteFilesWithPattern("./db", fmt.Sprintf("%s*.db*", client.UserID))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
	}

	c.JSON(http.StatusOK, gin.H{"status": "User deleted!"})
}

// ClientRemoveDevices represents login or registration data
// @Description Request payload for user login or registration
// @name ClientRemoveDevices
// @type object
type GroupInfoRequest struct {
	Username     string `json:"username" example:"john_doe"`
	DeviceId     string `json:"device_id" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
	GroupUrl     string `json:"group_url" example:"john_doe"`
}

// GetDevices godoc
// @Summary Fetches all active devices for the user
// @Description Retrieves device information for a user
// @Accept  json
// @Produce  json
// @Param   payload body ClientGetDevices true "User Credentials"
// @Success 200 {object} []string "Successfully retrieved devices"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /group/info [get]
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

	members, err := (&cmd.Controller{
		Client: client,
	}).GetRoomInfo(
		groupInfoRequest.DeviceId,
		groupInfoRequest.PlatformName,
		groupInfoRequest.GroupUrl,
	)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	c.IndentedJSON(http.StatusOK, members)
}

// GetDevices godoc
// @Summary Fetches all active devices for the user
// @Description Retrieves device information for a user
// @Accept  json
// @Produce  json
// @Param   payload body ClientGetDevices true "User Credentials"
// @Success 200 {object} rooms.RoomInfo "Successfully retrieved devices"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /info [get]
func GetInfo(c *gin.Context) {
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

	info, err := (&cmd.Controller{
		Client: client,
	}).GetInfo()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	c.IndentedJSON(http.StatusOK, info)
}
