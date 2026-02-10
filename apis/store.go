package apis

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
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
// @Summary Logs a user into the Matrix server
// @Description Stores the user's access token to be used for Syncing
// @Accept  json
// @Produce  json
// @Param   payload body ApiStoreRequestJson true "Login Credentials"
// @Success 200 {object} LoginResponse "Successfully stored"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Login failed"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /login [post]
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client, err := mautrix.NewClient(
		conf.HomeServer,
		id.NewUserID(username, conf.HomeServerDomain),
		apiStoreRequestJson.AccessToken,
	)
	if err := c.BindJSON(&apiStoreRequestJson); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	pickleKey, err := utils.GenerateRandomBytes(32)
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

	slog.Debug("Saved user", "username", client.UserID)
}
