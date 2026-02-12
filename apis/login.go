package apis

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
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
type ClientJsonRequest struct {
	Username string `json:"username" example:"john_doe"`
	Password string `json:"password" example:"securepassword123"`
}

func validatePassword(password string) (string, error) {
	// Password should be at least 7 characters
	if len(password) < 8 {
		return "", fmt.Errorf("password must be at least 7 characters long")
	}

	return password, nil
}

// ApiLogin godoc
// @Summary DO NOT USE THIS METHOD, FOR DEVELOPMENT ONLY
// @Summary Logs a user into the Matrix server
// @Description Authenticates a user and returns an access token
// @Accept  json
// @Produce  json
// @Param   payload body ClientJsonRequest true "Login Credentials"
// @Success 201 {object} map[string]string "Message sent successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Login failed"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /login [post]
func Login(c *gin.Context) {
	conf, err := configs.GetConf()

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}

	var clientJsonRequest ClientJsonRequest

	if err := c.BindJSON(&clientJsonRequest); err != nil {
		log.Printf("Invalid request payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	username, err := utils.SanitizeUsername(clientJsonRequest.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	password, err := validatePassword(clientJsonRequest.Password)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	controller := &cmd.Controller{Client: client}

	recoveryKey, err := controller.Login(password)

	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not your fault"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     client.UserID.String(),
		"access_token": controller.Client.AccessToken,
		"device_id":    controller.Client.DeviceID.String(),
		"recovery_key": recoveryKey,
		"status":       "logged in",
	})

}
