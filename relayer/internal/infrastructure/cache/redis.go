// Package cache provides Redis cache implementation.
package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements domain.CacheRepository using Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache.
func NewRedisCache(addr, password string, db int) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisCache{client: client}
}

// Set sets a value in cache.
func (c *RedisCache) Set(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(context.Background(), key, data, expiration).Err()
}

// Get gets a value from cache.
func (c *RedisCache) Get(key string) (string, error) {
	val, err := c.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Delete deletes a value from cache.
func (c *RedisCache) Delete(key string) error {
	return c.client.Del(context.Background(), key).Err()
}

// Exists checks if a key exists.
func (c *RedisCache) Exists(key string) (bool, error) {
	val, err := c.client.Exists(context.Background(), key).Result()
	return val > 0, err
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// GetWithDefault gets a value from cache with a default value.
func (c *RedisCache) GetWithDefault(key string, defaultValue string) (string, error) {
	val, err := c.Get(key)
	if err != nil {
		return defaultValue, err
	}
	if val == "" {
		return defaultValue, nil
	}
	return val, nil
}

// SetJSON sets a JSON value in cache.
func (c *RedisCache) SetJSON(key string, value interface{}, expiration time.Duration) error {
	return c.Set(key, value, expiration)
}

// GetJSON gets a JSON value from cache.
func (c *RedisCache) GetJSON(key string, dest interface{}) error {
	val, err := c.Get(key)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	return json.Unmarshal([]byte(val), dest)
}

// Increment increments a counter.
func (c *RedisCache) Increment(key string) (int64, error) {
	return c.client.Incr(context.Background(), key).Result()
}

// Decrement decrements a counter.
func (c *RedisCache) Decrement(key string) (int64, error) {
	return c.client.Decr(context.Background(), key).Result()
}

// SetNX sets a value only if the key does not exist.
func (c *RedisCache) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.client.SetNX(context.Background(), key, value, expiration).Result()
}

// GetTTL gets the TTL of a key.
func (c *RedisCache) GetTTL(key string) (time.Duration, error) {
	return c.client.TTL(context.Background(), key).Result()
}
