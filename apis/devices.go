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
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ClientJsonRequest represents login or registration data
// @Description Request payload for user login or registration
// @name ClientJsonRequest
// @type object
type ClientGetDevices struct {
	Username string `json:"username" example:"john_doe"`
}

// ClientJsonRequest represents login or registration data
// @Description Request payload for user login or registration
// @name ClientJsonRequest
// @type object
type ClientAddDevices struct {
	Username     string `json:"username" example:"john_doe"`
	PlatformName string `json:"platform_name" example:"john_doe"`
}

func GetDevices(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var clientGetDevices ClientGetDevices

	if err := c.ShouldBindQuery(&clientGetDevices); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	username, err := utils.SanitizeUsername(clientGetDevices.Username)
	if err != nil {
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

	devices, err := (&cmd.Controller{
		Client: client,
	}).GetDevices()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

func AddPlatformDevices(c *gin.Context) {
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

	bridge, err := (&bridges.Bridges{Client: client}).LookupBridgeByName(clientAddDevices.PlatformName)
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
