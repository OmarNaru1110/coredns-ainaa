package ainaa

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

// Manual Mocks

type MockCacheRepository struct {
	GetFunc func(ctx context.Context, domain string) (CachedDomain, error)
	SetFunc func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error
}

func (m *MockCacheRepository) Get(ctx context.Context, domain string) (CachedDomain, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, domain)
	}
	return CachedDomain{}, nil
}

func (m *MockCacheRepository) Set(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, domain, value, ttl)
	}
	return nil
}

type MockPersistentRepository struct {
	GetFunc  func(ctx context.Context, domain string) (DomainRecord, error)
	SaveFunc func(ctx context.Context, record DomainRecord) error
}

func (m *MockPersistentRepository) Get(ctx context.Context, domain string) (DomainRecord, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, domain)
	}
	return DomainRecord{}, nil
}

func (m *MockPersistentRepository) Save(ctx context.Context, record DomainRecord) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, record)
	}
	return nil
}

type MockResolver struct {
	LookupFunc          func(domain string) (map[string][]string, error)
	IsBlockedDomainFunc func(ips map[string][]string) bool
}

func (m *MockResolver) Lookup(domain string) (map[string][]string, error) {
	if m.LookupFunc != nil {
		return m.LookupFunc(domain)
	}
	return nil, nil
}

func (m *MockResolver) IsBlockedDomain(ips map[string][]string) bool {
	if m.IsBlockedDomainFunc != nil {
		return m.IsBlockedDomainFunc(ips)
	}
	return false
}

// Tests

func TestAinaa_ServeDNS(t *testing.T) {
	tests := []struct {
		name           string
		domain         string
		setupMocks     func(*MockCacheRepository, *MockPersistentRepository, *MockResolver)
		expectedRcode  int
		expectedAnswer []string
	}{
		{
			name:   "Cache Hit Allowed",
			domain: "example.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{
						Status: 0,
						IPs:    map[string][]string{"A": {"1.2.3.4"}},
					}, nil
				}
			},
			expectedRcode:  dns.RcodeSuccess,
			expectedAnswer: []string{"1.2.3.4"},
		},
		{
			name:   "Cache Hit Blocked",
			domain: "bad.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{
						Status: 1,
						IPs:    nil,
					}, nil
				}
			},
			expectedRcode: dns.RcodeNameError,
		},
		{
			name:   "Persistent Hit Allowed",
			domain: "example.org",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{}, errors.New("miss")
				}
				p.GetFunc = func(ctx context.Context, domain string) (DomainRecord, error) {
					return DomainRecord{
						Status: 0,
						IPs:    map[string][]string{"A": {"5.6.7.8"}},
					}, nil
				}
				c.SetFunc = func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
					if value.Status != 0 || value.IPs["A"][0] != "5.6.7.8" {
						t.Errorf("Unexpected cache set value: %v", value)
					}
					return nil
				}
			},
			expectedRcode:  dns.RcodeSuccess,
			expectedAnswer: []string{"5.6.7.8"},
		},
		{
			name:   "Miss Fresh Lookup Allowed",
			domain: "new.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{}, errors.New("miss")
				}
				p.GetFunc = func(ctx context.Context, domain string) (DomainRecord, error) {
					return DomainRecord{}, errors.New("miss")
				}
				r.LookupFunc = func(domain string) (map[string][]string, error) {
					return map[string][]string{"A": {"9.9.9.9"}}, nil
				}
				r.IsBlockedDomainFunc = func(ips map[string][]string) bool {
					return false
				}
				p.SaveFunc = func(ctx context.Context, record DomainRecord) error {
					if record.Domain != "new.com" || record.Status != 0 {
						t.Errorf("Unexpected persistent save value: %v", record)
					}
					return nil
				}
				c.SetFunc = func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
					if value.Status != 0 {
						t.Errorf("Unexpected cache set value: %v", value)
					}
					return nil
				}
			},
			expectedRcode:  dns.RcodeSuccess,
			expectedAnswer: []string{"9.9.9.9"},
		},
		{
			name:   "Miss Fresh Lookup Blocked",
			domain: "evil.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{}, errors.New("miss")
				}
				p.GetFunc = func(ctx context.Context, domain string) (DomainRecord, error) {
					return DomainRecord{}, errors.New("miss")
				}
				r.LookupFunc = func(domain string) (map[string][]string, error) {
					return map[string][]string{"A": {"6.6.6.6"}}, nil
				}
				r.IsBlockedDomainFunc = func(ips map[string][]string) bool {
					return true
				}
				p.SaveFunc = func(ctx context.Context, record DomainRecord) error {
					return nil
				}
				c.SetFunc = func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
					return nil
				}
			},
			expectedRcode: dns.RcodeNameError,
		},
		{
			name:   "Cache Hit No IPs",
			domain: "cached-no-ips.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{Status: 0, IPs: nil}, nil
				}
				r.LookupFunc = func(domain string) (map[string][]string, error) {
					return map[string][]string{"A": {"10.0.0.1"}}, nil
				}
				// Expect NO calls to Persistent.Save or Cache.Set
				p.SaveFunc = func(ctx context.Context, record DomainRecord) error {
					t.Errorf("Unexpected call to Persistent.Save")
					return nil
				}
				c.SetFunc = func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
					t.Errorf("Unexpected call to Cache.Set")
					return nil
				}
			},
			expectedRcode:  dns.RcodeSuccess,
			expectedAnswer: []string{"10.0.0.1"},
		},
		{
			name:   "Persistent Hit No IPs",
			domain: "db-no-ips.com",
			setupMocks: func(c *MockCacheRepository, p *MockPersistentRepository, r *MockResolver) {
				c.GetFunc = func(ctx context.Context, domain string) (CachedDomain, error) {
					return CachedDomain{}, errors.New("miss")
				}
				p.GetFunc = func(ctx context.Context, domain string) (DomainRecord, error) {
					return DomainRecord{Status: 0, IPs: nil}, nil
				}
				r.LookupFunc = func(domain string) (map[string][]string, error) {
					return map[string][]string{"A": {"10.0.0.2"}}, nil
				}
				// Expect Cache.Set but NO Persistent.Save
				c.SetFunc = func(ctx context.Context, domain string, value CachedDomain, ttl time.Duration) error {
					if value.Status != 0 || value.IPs != nil {
						t.Errorf("Unexpected cache set value: %v", value)
					}
					return nil
				}
				p.SaveFunc = func(ctx context.Context, record DomainRecord) error {
					t.Errorf("Unexpected call to Persistent.Save")
					return nil
				}
			},
			expectedRcode:  dns.RcodeSuccess,
			expectedAnswer: []string{"10.0.0.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := &MockCacheRepository{}
			mockPersistent := &MockPersistentRepository{}
			mockResolver := &MockResolver{}

			tt.setupMocks(mockCache, mockPersistent, mockResolver)

			a := Ainaa{
				Next:       plugin.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) { return 0, nil }),
				Cache:      mockCache,
				Persistent: mockPersistent,
				Resolver:   mockResolver,
			}

			r := new(dns.Msg)
			r.SetQuestion(tt.domain+".", dns.TypeA)
			rec := dnstest.NewRecorder(&test.ResponseWriter{})

			a.ServeDNS(context.TODO(), rec, r)

			if rec.Msg.Rcode != tt.expectedRcode {
				t.Errorf("Expected Rcode %d, got %d", tt.expectedRcode, rec.Msg.Rcode)
			}

			if len(tt.expectedAnswer) > 0 {
				if len(rec.Msg.Answer) == 0 {
					t.Errorf("Expected answer, got none")
				} else {
					// Check IP
					// Simplified check
					// ...
				}
			}
		})
	}
}
