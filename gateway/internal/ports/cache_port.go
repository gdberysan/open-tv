package ports

import "time"

type CachePort interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Flush() error
}
