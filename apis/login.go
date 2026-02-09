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
	"github.com/shortmesh/core/users"
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
// @Summary Logs a user into the Matrix server
// @Description Authenticates a user and returns an access token
// @Accept  json
// @Produce  json
// @Param   payload body ClientJsonRequest true "Login Credentials"
// @Success 200 {object} LoginResponse "Successfully logged in"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Login failed"
// @Failure 500 {object} ErrorResponse "Internal server error"
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

	username, err := configs.SanitizeUsername(clientJsonRequest.Username)
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
		debug.PrintStack()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := (&cmd.MatrixClient{Client: client}).Login(password)
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client.AccessToken = accessToken
	// TODO: generate and store recovery key during login
	err = (&users.Users{Client: client, RecoveryKey: "", PickleKey: ""}).Save()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
		return
	}
	slog.Debug("Saved user", "username", client.UserID)

	c.JSON(http.StatusOK, gin.H{
		"username":     client.UserID.String(),
		"access_token": accessToken,
		"status":       "logged in",
	})

}
