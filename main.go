package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/apis"
	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/rabbitmq"
	// "maunium.net/go/mautrix/id"
)

func main() {
	var programLevel slog.LevelVar
	programLevel.Set(slog.LevelDebug) // Set initial level to Debug
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &programLevel, // Use the LevelVar
	})
	slog.SetDefault(slog.New(handler))

	go cmd.SyncUsers()
	go RestAPIRoutines()
	go rabbitmq.RabbitMQReceiver()

	select {}
}

func RestAPIRoutines() {
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // use this for security
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.POST("/login", apis.Login)
	router.POST("/auth-url", apis.AuthUrl)
	router.POST("/health", apis.Health)
	router.GET("/:platform/devices", apis.GetPlatformDevices)
	router.POST("/:platform/devices", apis.AddPlatformDevices)
	router.DELETE("/:platform/devices/:deviceId", apis.RemovePlatformDevices)
	router.POST("/:platform/devices/:deviceId/message", apis.SendPlatformMessage)

	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return
	}
	host := cfg.Server.Host
	port := cfg.Server.Port

	tlsCert := cfg.Server.Tls.Crt
	tlsKey := cfg.Server.Tls.Key

	if tlsCert != "" && tlsKey != "" {
		err := router.RunTLS(fmt.Sprintf(":%s", port), tlsCert, tlsKey)
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return
		}
	} else {
		err := router.Run(fmt.Sprintf("%s:%s", host, port))
		if err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
			return
		}
	}
}
