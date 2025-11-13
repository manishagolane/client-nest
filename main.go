package main

import (
	"net/http"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/manishagolane/client-nest/auth"
	"github.com/manishagolane/client-nest/cache"
	"github.com/manishagolane/client-nest/clients"
	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/consumers"
	database "github.com/manishagolane/client-nest/db"
	"github.com/manishagolane/client-nest/logger"
	"github.com/manishagolane/client-nest/models"
	"go.uber.org/zap"
)

type App struct {
	poolConn   *pgxpool.Pool
	queries    *models.Queries
	logger     *zap.Logger
	cache      *cache.Cache
	lock       *cache.CacheLock
	server     *http.Server
	clients    *clients.Clients
	jwtManager *auth.JWTManager
	consumers  *consumers.Consumers
}

var wg sync.WaitGroup

func main() {
	config.Init()

	logger := logger.Init()
	defer logger.Sync()

	db := database.Init(logger)

	queries := models.New(db)
	cacheLock := cache.NewCacheLock(logger)
	cache := cache.NewCache(queries, logger)
	appClients := clients.InitializeClients(logger, cache, queries)
	jwtManager := auth.NewJWTManager(logger)
	appConsumers := consumers.InitializeConsumers(logger, appClients, queries)

	app := App{
		poolConn:   db,
		queries:    queries,
		logger:     logger,
		cache:      cache,
		lock:       cacheLock,
		clients:    appClients,
		jwtManager: jwtManager,
		consumers:  appConsumers,
	}

	wg.Add(1)
	// go app.initializeGRPCServer()
	go app.startServer()
	wg.Wait()
}
