package rediscache

import (
	"errors"
	"fmt"

	"github.com/gomodule/redigo/redis"

	"github.com/connylabs/zedcache/cache"
)

var _ cache.Cache = &Cache{}

func New(c redis.Conn) *Cache {
	return &Cache{c}
}

type Cache struct {
	conn redis.Conn
}

func (c *Cache) Get(key string) (string, error) {
	val, err := redis.String(c.conn.Do("GET", key))
	if err != nil {
		fmt.Println(err.Error())
		if errors.Is(err, redis.ErrNil) {
			return "", cache.ErrCacheMiss
		}

		return "", fmt.Errorf("failed to get cached entry for key %q: %w", key, err)
	}
	fmt.Printf("value: %v\n", val)

	return val, nil
}

func (c *Cache) Set(key, value string) error {
	_, err := c.conn.Do("SET", key, value)

	return err
}

func (c *Cache) Del(keys ...string) error {
	eIs := make([]any, len(keys))
	for i := range keys {
		eIs[i] = keys[i]
	}
	_, err := c.conn.Do("DEL", eIs...)

	return err
}
