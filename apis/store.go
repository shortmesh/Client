package apis

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/bridges"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/users"
	"github.com/shortmesh/core/utils"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// ApiStoreRequestJson represents User access token storage data
// @Description Request payload for storing user data for syncing
// @name ApiStoreRequestJson
// @type object
type ApiStoreRequestJson struct {
	Username    string `json:"username" example:"john_doe"`
	AccessToken string `json:"access_token" example:""`
	DeviceId    string `json:"device_id" example:""`
}

// Store godoc
// @Summary Store user credentials for syncing
// @Description Stores the user's access token to be used for Syncing
// @Accept  json
// @Produce  json
// @Param   payload body ApiStoreRequestJson true "Login Credentials"
// @Success 200 {string} string "User stored!" "Successfully stored"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /store [post]
func Store(c *gin.Context) {
	conf, err := configs.GetConf()
	var apiStoreRequestJson ApiStoreRequestJson

	if err := c.BindJSON(&apiStoreRequestJson); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	username, err := utils.SanitizeUsername(apiStoreRequestJson.Username)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		apiStoreRequestJson.AccessToken,
	)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client.DeviceID = id.DeviceID(apiStoreRequestJson.DeviceId)

	pickleKey, err := utils.GenerateRandomBytes(32)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	err = (&users.Users{
		Client:      client,
		RecoveryKey: "",
		PickleKey:   pickleKey,
	}).Save()

	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	for _, bridgeConf := range conf.Bridges {
		slog.Debug("Creating bridge room", "bridge_name", bridgeConf.Name)
		(&bridges.Bridges{
			BridgeConfig: bridgeConf,
			Client:       client,
		}).JoinManagementRooms()
	}

	c.JSON(http.StatusOK, "User stored!")
}
