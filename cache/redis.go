package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"

	"github.com/manishagolane/client-nest/config"

	"github.com/manishagolane/client-nest/models"

	"github.com/redis/rueidis"
	"go.uber.org/zap"
)

type Cache struct {
	rDB     rueidis.Client
	queries *models.Queries
	log     *zap.Logger
}

var ErrorNotFound = errors.New("not found")

func NewCache(queries *models.Queries, log *zap.Logger) *Cache {
	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{config.GetString("redis.address")},
		SelectDB:    config.GetInt("redis.db"),
		Password:    config.GetString("redis.password"),
	})
	if err != nil {
		log.Fatal("error creating redis client", zap.Error(err))
		return nil
	}
	cache := &Cache{
		queries: queries,
		rDB:     client,
		log:     log,
	}
	return cache
}

func (cache Cache) GetClient() *rueidis.Client {
	return &cache.rDB
}

func (cache Cache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cmd := cache.rDB.B().Setex().Key(key).Seconds(int64(expiration.Seconds())).Value(string(bytes)).Build()
	return cache.rDB.Do(ctx, cmd).Error()
}

func (cache Cache) GetJSON(ctx context.Context, key string, value interface{}) error {
	cmd := cache.rDB.B().Get().Key(key).Build()
	reader, err := cache.rDB.Do(ctx, cmd).AsReader()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return ErrorNotFound
		}
		return err
	}
	err = json.NewDecoder(reader).Decode(&value)
	if err != nil {
		return err
	}
	return nil
}

func (cache Cache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	cmd := cache.rDB.B().Setex().Key(key).Seconds(int64(expiration.Seconds())).Value(value).Build()
	return cache.rDB.Do(ctx, cmd).Error()
}
func (cache Cache) Get(ctx context.Context, key string) (string, error) {
	cmd := cache.rDB.B().Get().Key(key).Build()
	value, err := cache.rDB.Do(ctx, cmd).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return "", ErrorNotFound
		}
		return "", err
	}
	return value, nil
}

func (cache Cache) SetBool(ctx context.Context, key string, value bool, expiration time.Duration) error {
	valStr := "0"
	if value {
		valStr = "1"
	}
	cmd := cache.rDB.B().Setex().Key(key).Seconds(int64(expiration.Seconds())).Value(valStr).Build()
	return cache.rDB.Do(ctx, cmd).Error()
}

func (cache Cache) GetBool(ctx context.Context, key string) (bool, error) {
	cmd := cache.rDB.B().Get().Key(key).Build()
	value, err := cache.rDB.Do(ctx, cmd).ToString()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return false, ErrorNotFound
		}
		return false, err
	}
	return value == "1", nil
}

func (cache Cache) Delete(ctx context.Context, key string) error {
	cmd := cache.rDB.B().Del().Key(key).Build()
	return cache.rDB.Do(ctx, cmd).Error()
}

func (cache Cache) SetTicketOwner(ctx context.Context, ticketID string, ownerID string, expiration time.Duration) error {
	cacheKey := fmt.Sprintf("ticket:owner:%s", ticketID) // Key format: ticket:owner:<ticketID>
	return cache.Set(ctx, cacheKey, ownerID, expiration)
}

func (cache Cache) GetTicketOwner(ctx context.Context, ticketID string, queries *models.Queries) (string, error) {
	cacheKey := fmt.Sprintf("ticket:owner:%s", ticketID)
	ownerID, err := cache.Get(ctx, cacheKey)
	if err == ErrorNotFound {
		// Fetch from DB if Redis cache is empty
		ticket, err := queries.GetTicketByID(ctx, ticketID)
		if err != nil {
			return "", err
		}
		// Store in Redis for future requests
		cache.SetTicketOwner(ctx, ticketID, ticket.CreatedBy, 24*time.Hour)
		return ticket.CreatedBy, nil
	}
	return ownerID, err
}

func (cache Cache) InvalidateTicketCache(ctx context.Context, ticketID string) error {
	cacheKey := fmt.Sprintf("ticket:owner:%s", ticketID)
	return cache.Delete(ctx, cacheKey) // Remove outdated ownership info
}

func (cache Cache) Close() {
	cache.rDB.Close()
}
