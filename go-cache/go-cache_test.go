package gocache

import (
	"testing"

	gcache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"

	"github.com/connylabs/zedcache/cache"
)

func TestCache(t *testing.T) {
	t.Run("miss", func(t *testing.T) {
		c := New(gcache.New(gcache.NoExpiration, gcache.NoExpiration))
		_, err := c.Get("key")
		assert.ErrorIs(t, err, cache.ErrCacheMiss)
	})

	t.Run("hit", func(t *testing.T) {
		c := New(gcache.New(gcache.NoExpiration, gcache.NoExpiration))

		assert.NoError(t, c.Set("key", "value"))

		v, err := c.Get("key")

		assert.NoError(t, err)

		assert.Equal(t, "value", v)
	})

	t.Run("hit after overwrite", func(t *testing.T) {
		c := New(gcache.New(gcache.NoExpiration, gcache.NoExpiration))

		assert.NoError(t, c.Set("key", "value"))
		assert.NoError(t, c.Set("key", "value!"))

		v, err := c.Get("key")

		assert.NoError(t, err)

		assert.Equal(t, "value!", v)
	})
}
