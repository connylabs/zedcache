package cache

import (
	"errors"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache interface {
	Get(string) (string, error)
	Set(string, string) error
	// Del deletes the given keys.
	// It is not expected to return an error if any of the given keys does not exist.
	// To avoid the "New Enemy" problem, Del() must consistently delete the given keys.
	// Otherwise it is possible for users that were just revoked the access to a resource
	// to still gain access with on old cached token.
	Del(...string) error
}
