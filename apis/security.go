package apis

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/configs"
	"golang.org/x/crypto/hkdf"
)

func authenticateBearer(bearer string) (bool, error) {
	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	secret := []byte(conf.MAS_CLIENT_ID)
	salt := []byte(conf.MAS_CLIENT_SECRET)
	info := []byte(conf.API_AUTHENTICATION_INFO)
	keyLen := 32

	hkdf := hkdf.New(sha256.New, secret, salt, info)

	key := make([]byte, keyLen)
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return false, err
	}

	return bytes.Equal(key, []byte(bearer)), nil
}

func SecureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // use this for security
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		bearer, err := extractBearerToken(c)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return
		}

		ok, err := authenticateBearer(bearer)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return
		}

		if ok {
			c.Next()
		}
	}
}

func extractBearerToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("Authorization header is required")
	}

	// Check if it starts with "Bearer "
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("Authorization header must start with 'Bearer '")
	}

	// Extract the token (remove "Bearer " prefix)
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", fmt.Errorf("Bearer token cannot be empty")
	}

	return token, nil
}
