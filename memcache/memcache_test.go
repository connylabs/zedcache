package memcache

import (
	"os"
	"testing"
	"time"

	gmc "github.com/bradfitz/gomemcache/memcache"
	"github.com/efficientgo/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/connylabs/zedcache/cache"
)

func TestCache(t *testing.T) {
	if v, ok := os.LookupEnv("E2E"); !ok || !(v == "1" || v == "true") {
		t.Skip("To enable this test, set the E2E environment variable to 1 or true")
	}

	e, err := e2e.NewDockerEnvironment("memcache-e2e")
	require.NoError(t, err)
	require.NoError(t, err)
	r := e.Runnable("memcache").WithPorts(
		map[string]int{
			"memcache": 11211,
		}).Init(e2e.StartOptions{
		Image:     "memcached:1.6-alpine",
		Readiness: e2e.NewTCPReadinessProbe("memcache"),
	})
	require.NoError(t, r.Start())
	require.NoError(t, r.WaitReady())

	t.Run("miss", func(t *testing.T) {
		c := connection(t, r)
		_, err := c.Get("key")
		assert.ErrorIs(t, err, cache.ErrCacheMiss)
	})

	t.Run("hit", func(t *testing.T) {
		c := connection(t, r)

		assert.NoError(t, c.Set("key", "value"))
		time.Sleep(time.Second)

		v, err := c.Get("key")

		assert.NoError(t, err)

		assert.Equal(t, "value", v)
	})

	t.Run("del", func(t *testing.T) {
		c := connection(t, r)

		assert.NoError(t, c.Set("key", "value"))
		time.Sleep(time.Second)

		_, err := c.Get("key")

		assert.NoError(t, err)

		assert.NoError(t, c.Del("key"))

		_, err = c.Get("key")
		assert.ErrorIs(t, err, cache.ErrCacheMiss)
	})

	t.Run("hit after overwrite", func(t *testing.T) {
		c := connection(t, r)

		assert.NoError(t, c.Set("key", "value"))
		assert.NoError(t, c.Set("key", "value!"))
		time.Sleep(time.Second)

		v, err := c.Get("key")

		assert.NoError(t, err)

		assert.Equal(t, "value!", v)
	})
}

func connection(t *testing.T, r e2e.Runnable) *MemCache {
	t.Helper()

	conn := gmc.New(r.Endpoint("memcache"))
	t.Cleanup(func() {
		require.NoError(t, conn.DeleteAll())
	})
	return &MemCache{conn}
}
