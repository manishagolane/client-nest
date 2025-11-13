package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manishagolane/client-nest/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func (app *App) startServer() {
	defer wg.Done()

	environment := config.GetString("environment")
	port := config.GetString("server.port")
	readTimeout := config.GetInt("server.readTimeout")
	writeTimeout := config.GetInt("server.writeTimeout")

	if environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.UseH2C = true
	err := router.SetTrustedProxies([]string{"127.0.0.1"})
	if err != nil {
		app.logger.Fatal("error setting trusted proxy", zap.Error(err))
		return
	}

	router.Use(requestIdInterceptor())
	router.Use(ginLogger(), ginRecovery())

	app.registerRoutes(router)

	router.HandleMethodNotAllowed = true

	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"status": "method not allowed"})
	})
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"status": "not found"})
	})

	h2s := &http2.Server{}
	app.server = &http.Server{
		Addr:           port,
		Handler:        h2c.NewHandler(router, h2s),
		ReadTimeout:    time.Duration(readTimeout) * time.Second,
		WriteTimeout:   time.Duration(writeTimeout) * time.Second,
		MaxHeaderBytes: 1 << 10,
	}

	app.cleanup()
	app.logger.Info(fmt.Sprintf("Starting server on port %s", port))
	if err = app.server.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			app.logger.Info("shutting down server")
			return
		}
		app.logger.Fatal("failed to start server", zap.Error(err))
	}
}

func (app *App) cleanup() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		app.logger.Info("received shutdown signal, shutting down...")
		app.server.Shutdown(context.Background())
		app.poolConn.Close()
		app.cache.Close()
		app.logger.Sync()
		os.Exit(0)
	}()
}
