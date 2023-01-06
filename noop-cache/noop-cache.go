package noopcache

import "github.com/connylabs/zedcache/cache"

type Noopcache struct{}

var _ cache.Cache = &Noopcache{}

func (*Noopcache) Get(string) (string, error) {
	return "", cache.ErrCacheMiss
}

func (*Noopcache) Set(string, string) error {
	return nil
}

func (*Noopcache) Del(...string) error {
	return nil
}
