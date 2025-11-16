package ainaa

import (
	"context"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

var log = clog.NewWithPlugin("ainaa")

type Ainaa struct {
	Next           plugin.Handler
	redisClient    *redis.Client
	dynamodbClient *dynamodb.Client
}

const tableName = "AinaaDomains"

var blockedIPs = []string{
	"146.112.61.106",
	"146.112.61.104",
	"::ffff:146.112.61.104",
	"::ffff:9270:3d6a",
}

func (a Ainaa) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	domain := r.Question[0].Name
	domain = domain[:len(domain)-1] // Remove trailing dot
	resolver := OpenDNSResolver{}

	// lookup domain in redis
	if cachedVal, err := getCachedDomain(ctx, a.redisClient, domain); err == nil { // Cache hit

		if cachedVal.Status != 0 {
			return dns.RcodeNameError, nil
		}

		if cachedVal.IPs != nil {
			resp := buildResponse(r, cachedVal.IPs)
			w.WriteMsg(resp)
			return dns.RcodeSuccess, nil
		}

		ips, err := resolver.Lookup(domain)
		if err != nil {
			log.Errorf("Error looking up domain %s: %v", domain, err)
			return dns.RcodeServerFailure, err
		}
		resp := buildResponse(r, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	// Cache miss - lookup in DynamoDB
	domainRecord, err := getDomain(ctx, a.dynamodbClient, domain)
	if err != nil {
		ips, err := resolver.Lookup(domain)
		if err != nil {
			log.Errorf("Error looking up domain %s: %v", domain, err)
			return dns.RcodeServerFailure, err
		}
		newDomainRec := DomainRecord{
			Domain:    domain,
			CreatedAt: time.Now().UTC(),
		}
		newCachedRec := CachedDomain{}

		if resolver.IsBlockedDomain(ips) {
			//////////////////////////////////////////////////////////
			newDomainRec.Status = 1 // 1 or 2?, make sure to update this
			newCachedRec.Status = 1 // 1 or 2?, make sure to update this
			//////////////////////////////////////////////////////////
			storeDomain(ctx, a.dynamodbClient, newDomainRec)
			storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
			return dns.RcodeNameError, nil
		}
		newDomainRec.Status = 0
		newCachedRec.Status = 0
		storeDomain(ctx, a.dynamodbClient, newDomainRec)
		storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
		resp := buildResponse(r, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	storeCachedDomain(ctx, a.redisClient, domain, CachedDomain{Status: domainRecord.Status, IPs: domainRecord.IPs})

	if domainRecord.Status != 0 {
		return dns.RcodeNameError, nil
	}

	if domainRecord.IPs != nil {
		resp := buildResponse(r, domainRecord.IPs)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	ips, err := resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}
	resp := buildResponse(r, ips)
	w.WriteMsg(resp)

	return dns.RcodeSuccess, nil
}

func (a Ainaa) Name() string { return "ainaa" }

func buildResponse(r *dns.Msg, ips map[string][]string) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true
	resp.Rcode = dns.RcodeSuccess
	for _, ip := range ips["A"] {
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   r.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(ip),
		})
	}
	for _, ip := range ips["AAAA"] {
		resp.Answer = append(resp.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   r.Question[0].Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			AAAA: net.ParseIP(ip),
		})
	}
	return resp
}
