package cache

import (
	"time"

	"github.com/manishagolane/client-nest/config"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type CacheLock struct {
	lock *redsync.Redsync
}

func NewCacheLock(logger *zap.Logger) *CacheLock {
	client := goredislib.NewClient(&goredislib.Options{
		Addr:     config.GetString("redis.address"),
		Password: config.GetString("redis.password"),
		DB:       config.GetInt("redis.lockDB"),
	})
	pool := goredis.NewPool(client)
	return &CacheLock{
		lock: redsync.New(pool),
	}
}

func (cl *CacheLock) Mutex(key string) *redsync.Mutex {
	return cl.lock.NewMutex(key, redsync.WithExpiry(time.Duration(config.GetInt("redis.lockExpiry"))*time.Second))
}
