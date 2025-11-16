package ainaa

import (
	"context"
	"errors"
	"net"
	"time"
)

type DomainRecord struct {
	Domain    string              `json:"domain" dynamodbav:"domain"`
	Status    int                 `json:"status" dynamodbav:"status"`
	CreatedAt time.Time           `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt" dynamodbav:"updatedAt"`
	IPs       map[string][]string `json:"ips" dynamodbav:"ips"`
}

type CachedDomain struct {
	Status int                 `json:"status" redis:"status"`
	IPs    map[string][]string `json:"ips" redis:"ips"`
}

type Resolver interface {
	Lookup(domain string) (map[string][]string, error)
}

type OpenDNSResolver struct{}

func (r *OpenDNSResolver) Lookup(domain string) (map[string][]string, error) {
	openDNSResolvers := []string{
		"208.67.222.222:53", // primary
		"208.67.220.220:53", // secondary
	}
	res := make(map[string][]string)

	for _, resolverAddr := range openDNSResolvers {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, network, resolverAddr)
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ips, err := resolver.LookupIP(ctx, "ip", domain)
		cancel()

		if err == nil {
			for _, ip := range ips {
				if ip.To4() != nil {
					res["A"] = append(res["A"], ip.String())
				} else if ip.To16() != nil {
					res["AAAA"] = append(res["AAAA"], ip.String())
				}
			}
			return res, nil // success
		}
	}
	return nil, errors.New("failed to resolve domain using OpenDNS")
}

func (r *OpenDNSResolver) IsBlockedDomain(ips map[string][]string) bool {
	for _, ip := range blockedIPs {
		for _, resolvedIps := range ips {
			for _, rip := range resolvedIps {
				if rip == ip {
					return true
				}
			}
		}
	}
	return false
}
