package memcached

import (
	"errors"

	gmc "github.com/bradfitz/gomemcache/memcache"
	"github.com/hashicorp/go-multierror"

	"github.com/connylabs/zedcache/cache"
)

type MemCache struct {
	Client *gmc.Client
}

var _ cache.Cache = &MemCache{}

func (mc *MemCache) Get(k string) (string, error) {
	v, err := mc.Client.Get(k)
	if err != nil {
		if errors.Is(err, gmc.ErrCacheMiss) {
			return "", cache.ErrCacheMiss
		} else {
			return "", err
		}
	}

	return string(v.Value), nil
}

func (mc *MemCache) Set(k string, v string) error {
	return mc.Client.Set(&gmc.Item{
		Key:        k,
		Value:      []byte(v),
		Expiration: 0,
	})
}

func (mc *MemCache) Del(keys ...string) error {
	g := multierror.Group{}

	for _, k := range keys {
		key := k

		g.Go(func() error {
			err := mc.Client.Delete(key)
			if errors.Is(err, gmc.ErrCacheMiss) {
				return nil
			}

			return err
		})
	}

	return g.Wait().ErrorOrNil()
}
