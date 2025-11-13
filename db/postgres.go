package db

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/constants"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

func Init(logger *zap.Logger) *pgxpool.Pool {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.GetString("db.user"),
		url.PathEscape(config.GetString("db.password")),
		config.GetString("db.host"),
		config.GetInt("db.port"),
		config.GetString("db.dbname"),
		config.GetString("db.sslmode"))

	ctx := context.WithValue(context.Background(), constants.REQUEST_ID, "dbInit")
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(config.GetInt("db.timeout")))
	defer cancel()

	connCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logger.Fatal("error parsing database URL", zap.Error(err))
	}
	if config.GetString("environment") == "development" {
		connCfg.ConnConfig.LogLevel = pgx.LogLevelWarn
	}

	conn, err := pgxpool.ConnectConfig(ctx, connCfg)
	if err != nil {
		logger.Fatal("error connecting to database", zap.Error(err))
	}

	pgxpool.Connect(ctx, dbURL)

	logger.Info("Connected to database!!")
	return conn
}
