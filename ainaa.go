package ainaa

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

const (
	tableName = "AinaaDomains"
	name      = "ainaa"
)

var log = clog.NewWithPlugin(name)

type Ainaa struct {
	Next           plugin.Handler
	redisClient    *redis.Client
	dynamodbClient *dynamodb.Client
}

var openDNSBlockedIPs = []string{
	"146.112.61.106",
	"146.112.61.104",
	"::ffff:146.112.61.104",
	"::ffff:9270:3d6a",
}
var blockedIPs = map[string][]string{
	"A":    {"0.0.0.0"},
	"AAAA": {"::"},
}

func (a Ainaa) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	domain := r.Question[0].Name
	domain = domain[:len(domain)-1] // Remove trailing dot
	resolver := OpenDNSResolver{}

	// lookup domain in redis
	if cachedVal, err := getCachedDomain(ctx, a.redisClient, domain); err == nil { // Cache hit
		if cachedVal.Status != 0 {
			resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
			w.WriteMsg(resp)
			return dns.RcodeNameError, nil
		}

		if cachedVal.IPs != nil {
			resp := buildResponse(r, dns.RcodeSuccess, cachedVal.IPs)
			w.WriteMsg(resp)
			return dns.RcodeSuccess, nil
		}

		ips, err := resolver.Lookup(domain)
		if err != nil {
			log.Errorf("Error looking up domain %s: %v", domain, err)
			return dns.RcodeServerFailure, err
		}
		resp := buildResponse(r, dns.RcodeSuccess, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	// Cache miss - lookup in DynamoDB
	domainRecord, err := getDomain(ctx, a.dynamodbClient, domain)
	if err != nil { // Not found in DynamoDB
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
			envStatus, _ := strconv.Atoi(os.Getenv("STATUS"))
			newDomainRec.Status = envStatus
			newCachedRec.Status = envStatus
			storeDomain(ctx, a.dynamodbClient, newDomainRec)
			storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
			resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
			w.WriteMsg(resp)
			return dns.RcodeNameError, nil
		}

		newDomainRec.Status = 0
		newCachedRec.Status = 0
		storeDomain(ctx, a.dynamodbClient, newDomainRec)
		storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
		resp := buildResponse(r, dns.RcodeSuccess, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	storeCachedDomain(ctx, a.redisClient, domain, CachedDomain{Status: domainRecord.Status, IPs: domainRecord.IPs})

	if domainRecord.Status != 0 {
		resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
		w.WriteMsg(resp)
		return dns.RcodeNameError, nil
	}

	if domainRecord.IPs != nil {
		resp := buildResponse(r, dns.RcodeSuccess, domainRecord.IPs)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	ips, err := resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}
	resp := buildResponse(r, dns.RcodeSuccess, ips)
	w.WriteMsg(resp)

	return dns.RcodeSuccess, nil
}

func (a Ainaa) Name() string { return name }

func buildResponse(r *dns.Msg, rcodeStatus int, ips map[string][]string) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true
	resp.Rcode = rcodeStatus
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
