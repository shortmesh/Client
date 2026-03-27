package apis

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/devices"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ClientGetDevices represents login or registration data
// @Description Request payload for user login or registration
// @name ClientGetDevices
// @type object
type ClientGetDevices struct {
	Username string `json:"username" example:"john_doe"`
}

// ClientAddDevices represents login or registration data
// @Description Request payload for user login or registration
// @name ClientAddDevices
// @type object
type ClientAddDevices struct {
	Username     string `json:"username" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
}

// ClientRemoveDevices represents login or registration data
// @Description Request payload for user login or registration
// @name ClientRemoveDevices
// @type object
type ClientRemoveDevices struct {
	Username     string `json:"username" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
	DeviceId     string `json:"device_id" example:"john_doe"`
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
func GetDevices(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var clientGetDevices ClientGetDevices

	if err := c.BindJSON(&clientGetDevices); err != nil {
		slog.Error("Invalid request payload", "payload", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}
	fmt.Printf("%v\n", clientGetDevices)

	username, err := utils.SanitizeUsername(clientGetDevices.Username)
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
	}).GetDevices()

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

// AddDevices godoc
// @Summary Adds a new bridge device for a platform
// @Description Adds a new user bridge device for the specified platform
// @Accept  json
// @Produce  json
// @Param   payload body ClientAddDevices true "User Credentials"
// @Success 201 {object} map[string]string "Successfully added device"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /devices [post]
func AddDevices(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var clientAddDevices ClientAddDevices

	if err := c.BindJSON(&clientAddDevices); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	username, err := utils.SanitizeUsername(clientAddDevices.Username)
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

	bridge, err := bridges.LookupBridgeByName(client, clientAddDevices.PlatformName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	err = bridge.AddDevice()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": fmt.Sprintf("Request to create new created for %s", clientAddDevices.PlatformName)})
}

// RemoveDevices godoc
// @Summary Removes a bridge device for a platform
// @Description Removes a user bridge device for the specified platform
// @Accept  json
// @Produce  json
// @Param   payload body ClientRemoveDevices true "User Credentials"
// @Success 201 {object} map[string]string "Successfully removed device"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /devices [delete]
func RemoveDevices(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var clientRemoveDevices ClientRemoveDevices

	if err := c.BindJSON(&clientRemoveDevices); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	username, err := utils.SanitizeUsername(clientRemoveDevices.Username)
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

	if user.Client == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	// client.AccessToken = user.Client.AccessToken

	bridge, err := bridges.LookupBridgeByName(client, clientRemoveDevices.PlatformName)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	err = bridge.RemoveDevice(clientRemoveDevices.DeviceId)
	slog.Debug("Devices", "removing", clientRemoveDevices.DeviceId)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": fmt.Sprintf("Request to remove created for %s", clientRemoveDevices.PlatformName)})
}
