package main

import (
	"fmt"
	"log/slog"
	"os"
	_ "sherlock/matrix/docs"

	"github.com/gin-gonic/gin"
	// "maunium.net/go/mautrix/id"
)

type User struct {
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	AccessToken      string `yaml:"access_token"`
	RecoveryKey      string `yaml:"recovery_key"`
	DeviceId         string `yaml:"device_id"`
	HomeServer       string `yaml:"homeserver"`
	HomeServerDomain string `yaml:"homeserver_domain"`
}

func main() {
	var programLevel slog.LevelVar
	programLevel.Set(slog.LevelDebug) // Set initial level to Debug
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &programLevel, // Use the LevelVar
	})
	slog.SetDefault(slog.New(handler))

	if len(os.Args) > 2 {
		TerminalRoutines()
		return
	}

	if cfgError != nil {
		panic(cfgError)
	}

	go SyncUsers()
	go RestAPIRoutines()
	go RabbitMQReceiver()

	select {}
}

func RestAPIRoutines() {
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.POST("/", APICreate)
	router.POST("/login", APILogin)
	router.POST("/:platform/devices", APIAddDevice)

	host := cfg.Server.Host
	port := cfg.Server.Port

	tlsCert := cfg.Server.Tls.Crt
	tlsKey := cfg.Server.Tls.Key

	if tlsCert != "" && tlsKey != "" {
		router.RunTLS(fmt.Sprintf(":%s", port), tlsCert, tlsKey)
	} else {
		router.Run(fmt.Sprintf("%s:%s", host, port))
	}
}
