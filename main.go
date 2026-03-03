package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shortmesh/core/apis"
	"github.com/shortmesh/core/cmd"
	"github.com/shortmesh/core/configs"
	"github.com/shortmesh/core/docs"
	_ "github.com/shortmesh/core/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	// "maunium.net/go/mautrix/id"
)

// @title ShortMesh - Client API
// @version 1.0
func main() {
	var programLevel slog.LevelVar
	programLevel.Set(slog.LevelDebug) // Set initial level to Debug
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &programLevel, // Use the LevelVar
	})
	slog.SetDefault(slog.New(handler))

	go cmd.SyncUsers()
	go RestAPIRoutines()
	// go rabbitmq.RabbitMQReceiver()

	select {}
}

func RestAPIRoutines() {
	router := gin.Default()

	// Add CORS middleware
	router.Use(apis.SecureMiddleware())

	cfg, err := configs.GetConf()
	if err != nil {
		slog.Error(err.Error())
		debug.PrintStack()
		return
	}

	apiVersion := cfg.ApiVersion
	host := cfg.Server.Host
	port := cfg.Server.Port

	docs.SwaggerInfo.Title = "ShortMesh - Client API"
	docs.SwaggerInfo.Version = strconv.Itoa(apiVersion)
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%s", host, port)
	docs.SwaggerInfo.BasePath = fmt.Sprintf("/api/v%d", apiVersion)

	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.GetDevices)

	router.POST(fmt.Sprintf("/api/v%d/login", apiVersion), apis.Login)
	router.POST(fmt.Sprintf("/api/v%d/store", apiVersion), apis.Store)
	router.POST(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.AddDevices)
	router.POST(fmt.Sprintf("/api/v%d/devices/:deviceId/message", apiVersion), apis.SendMessage)

	router.DELETE(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.RemoveDevices)
	router.DELETE(fmt.Sprintf("/api/v%d/users", apiVersion), apis.Delete)

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
