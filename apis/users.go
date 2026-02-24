package apis

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
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

	user, err := users.FetchUser(client)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Something wasn't found"})
		return
	}
	if user.Client == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	client.AccessToken = user.Client.AccessToken

	_, err = client.Logout(context.Background())
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = users.RemoveUser(client)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = utils.DeleteFilesWithPattern("./db", fmt.Sprintf("%s*.db*", client.UserID.Localpart()))
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Not your fault"})
	}

	c.JSON(http.StatusOK, gin.H{"status": "User created!"})
}
