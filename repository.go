package ainaa

import (
	"context"
	"time"
)

// CacheRepository defines the interface for caching operations.
type CacheRepository interface {
	Get(ctx context.Context, domain string) (CachedDomain, error)
	Set(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error
}

// PersistentRepository defines the interface for persistent storage operations.
type PersistentRepository interface {
	Get(ctx context.Context, domain string) (DomainRecord, error)
	Save(ctx context.Context, record DomainRecord) error
}
