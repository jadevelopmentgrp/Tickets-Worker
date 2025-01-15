package redis

import (
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	redsyncredis "github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/jadevelopmentgrp/Tickets-Worker/config"
)

var (
	Client *redis.Client
	rs     *redsync.Redsync
)

var ErrNil = redis.Nil

func Connect() error {
	Client = redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         config.Conf.Redis.Address,
		Password:     config.Conf.Redis.Password,
		PoolSize:     config.Conf.Redis.Threads,
		MinIdleConns: config.Conf.Redis.Threads,
	})

	pool := redsyncredis.NewPool(Client)
	rs = redsync.New(pool)

	return nil
}
