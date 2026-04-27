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
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Equivalent to enabling debug logs
		// AddSource: true,            // Equivalent to log.Lshortfile/log.Llongfile
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	apis.SessionsCacheInit()

	go cmd.BootupSyncUsers()
	go RestAPIRoutines()
	// go rabbitmq.RabbitMQReceiver()

	select {}
}

func RestAPIRoutines() {
	router := gin.Default()

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

	// /docs is registered directly on the router — no middleware
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// All secured routes go in this group
	secured := router.Group("/")
	secured.Use(apis.SecureMiddleware())
	{
		secured.GET(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.GetDevices)
		secured.POST(fmt.Sprintf("/api/v%d/login", apiVersion), apis.Login)
		secured.POST(fmt.Sprintf("/api/v%d/store", apiVersion), apis.Store)
		secured.POST(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.AddDevices)
		secured.POST(fmt.Sprintf("/api/v%d/devices/:deviceId/message", apiVersion), apis.SendMessage)
		secured.DELETE(fmt.Sprintf("/api/v%d/devices", apiVersion), apis.RemoveDevices)
		secured.DELETE(fmt.Sprintf("/api/v%d/users", apiVersion), apis.Delete)
	}

	tlsCert := cfg.Server.Tls.Crt
	tlsKey := cfg.Server.Tls.Key
	if tlsCert != "" && tlsKey != "" {
		if err := router.RunTLS(fmt.Sprintf(":%s", port), tlsCert, tlsKey); err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	} else {
		if err := router.Run(fmt.Sprintf("%s:%s", host, port)); err != nil {
			slog.Error(err.Error())
			debug.PrintStack()
		}
	}
}
