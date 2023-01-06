package gocache

import (
	"github.com/hashicorp/go-multierror"
	gcache "github.com/patrickmn/go-cache"

	"github.com/connylabs/zedcache/cache"
)

var _ cache.Cache = &Cache{}

func New(c *gcache.Cache) *Cache {
	return &Cache{c}
}

type Cache struct {
	c *gcache.Cache
}

func (c *Cache) Get(key string) (string, error) {
	val, ok := c.c.Get(key)
	if ok {
		t, ok := val.(string)
		if ok {
			return t, nil
		}
	}

	return "", cache.ErrCacheMiss
}

func (c *Cache) Set(key, value string) error {
	c.c.Set(key, value, gcache.NoExpiration)

	return nil
}

func (c *Cache) Del(keys ...string) error {
	g := multierror.Group{}

	for _, k := range keys {
		key := k

		g.Go(func() error {
			c.c.Delete(key)

			return nil
		})
	}

	return g.Wait().ErrorOrNil()
}
