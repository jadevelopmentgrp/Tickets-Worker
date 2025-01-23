package dbclient

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	database "github.com/jadevelopmentgrp/Tickets-Database"
	"github.com/jadevelopmentgrp/Tickets-Worker/config"
	"go.uber.org/zap"
)

var Client *database.Database

func Connect() {
	logger := zap.NewExample()
	defer logger.Sync()

	logger.With(zap.String("service", "database"))

	cfg, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s/%s?pool_max_conns=%d",
		config.Conf.Database.Username,
		config.Conf.Database.Password,
		config.Conf.Database.Host,
		config.Conf.Database.Database,
		config.Conf.Database.Threads,
	))

	if err != nil {
		logger.Fatal("Failed to parse database config", zap.Error(err))
		return
	}

	cfg.ConnConfig.LogLevel = pgx.LogLevelWarn
	cfg.ConnConfig.Logger = NewLogAdapter(logger)

	pool, err := pgxpool.ConnectConfig(context.Background(), cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}

	Client = database.NewDatabase(pool)
}
