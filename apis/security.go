package apis

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/configs"
)

const (
	MaxSkewSeconds = 30
)

func VerifyRequest(id, method, path, timestamp, nonce, body, receivedSignature string) (bool, error) {
	epoch, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid epoch timestamp")
	}

	ts := time.Unix(epoch, 0)

	if math.Abs(time.Since(ts).Seconds()) > MaxSkewSeconds {
		return false, fmt.Errorf("request expired or clock desync")
	}

	canonicalString := id + method + path + timestamp + nonce + body

	conf, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return false, err
	}

	h := hmac.New(sha256.New, []byte(conf.MAS_CLIENT_SECRET))
	h.Write([]byte(canonicalString))
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	slog.Debug("Verify request",
		"id", id,
		"method", method,
		"path", path,
		"timestamp", timestamp,
		"nonce", nonce,
		"body", body,
		"signature", expectedSignature,
		"received", receivedSignature,
	)

	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature)), nil
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

		method := c.Request.Method
		path := c.Request.URL.Path

		id := c.GetHeader("X-ShortMesh-ID")
		timestamp := c.GetHeader("X-ShortMesh-Timestamp")
		nonce := c.GetHeader("X-ShortMesh-Nonce")
		signature := c.GetHeader("X-ShortMesh-Signature")

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "Could not read body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		ok, err := VerifyRequest(
			id,
			method,
			path,
			timestamp,
			nonce,
			string(bodyBytes),
			signature,
		)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			c.AbortWithStatusJSON(400, gin.H{"error": err.Error()})
			return
		}

		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
		}

		c.Next()
	}
}
