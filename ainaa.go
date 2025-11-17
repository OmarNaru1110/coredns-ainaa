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

var log = clog.NewWithPlugin("ainaa")

type Ainaa struct {
	Next           plugin.Handler
	redisClient    *redis.Client
	dynamodbClient *dynamodb.Client
}

const tableName = "AinaaDomains"

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

	log.Infof("Received DNS query for domain: %s", domain)

	// lookup domain in redis
	if cachedVal, err := getCachedDomain(ctx, a.redisClient, domain); err == nil { // Cache hit
		log.Infof("Cache hit for domain: %s", domain)
		if cachedVal.Status != 0 {
			log.Infof("Domain %s is blocked in cache with status %d", domain, cachedVal.Status)
			resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
			w.WriteMsg(resp)
			return dns.RcodeNameError, nil
		}

		log.Infof("Domain %s is allowed in cache with status %d", domain, cachedVal.Status)
		if cachedVal.IPs != nil {
			log.Infof("Returning cached IPs for domain %s: %v", domain, cachedVal.IPs)
			resp := buildResponse(r, dns.RcodeSuccess, cachedVal.IPs)
			w.WriteMsg(resp)
			return dns.RcodeSuccess, nil
		}

		ips, err := resolver.Lookup(domain)
		if err != nil {
			log.Errorf("Error looking up domain %s: %v", domain, err)
			return dns.RcodeServerFailure, err
		}
		log.Infof("Returning resolved IPs for domain %s: %v", domain, ips)
		resp := buildResponse(r, dns.RcodeSuccess, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	// Cache miss - lookup in DynamoDB
	log.Infof("Cache miss for domain: %s", domain)
	domainRecord, err := getDomain(ctx, a.dynamodbClient, domain)
	if err != nil { // Not found in DynamoDB
		log.Infof("Domain %s not found in DynamoDB", domain)
		ips, err := resolver.Lookup(domain)
		if err != nil {
			log.Errorf("Error looking up domain %s: %v", domain, err)
			return dns.RcodeServerFailure, err
		}
		log.Infof("Returning resolved IPs for domain %s: %v", domain, ips)
		newDomainRec := DomainRecord{
			Domain:    domain,
			CreatedAt: time.Now().UTC(),
		}
		newCachedRec := CachedDomain{}

		if resolver.IsBlockedDomain(ips) {
			log.Infof("Domain %s is identified as blocked", domain)
			envStatus, _ := strconv.Atoi(os.Getenv("STATUS"))
			newDomainRec.Status = envStatus
			newCachedRec.Status = envStatus
			storeDomain(ctx, a.dynamodbClient, newDomainRec)
			storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
			resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
			w.WriteMsg(resp)
			return dns.RcodeNameError, nil
		}

		log.Infof("Domain %s is allowed", domain)
		newDomainRec.Status = 0
		newCachedRec.Status = 0
		storeDomain(ctx, a.dynamodbClient, newDomainRec)
		storeCachedDomain(ctx, a.redisClient, domain, newCachedRec)
		resp := buildResponse(r, dns.RcodeSuccess, ips)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	log.Infof("Domain %s found in DynamoDB with status %d", domain, domainRecord.Status)
	storeCachedDomain(ctx, a.redisClient, domain, CachedDomain{Status: domainRecord.Status, IPs: domainRecord.IPs})

	if domainRecord.Status != 0 {
		log.Infof("Domain %s is blocked in dynamodb with status %d", domain, domainRecord.Status)
		resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
		w.WriteMsg(resp)
		return dns.RcodeNameError, nil
	}

	if domainRecord.IPs != nil {
		log.Infof("Returning IPs from DynamoDB for domain %s: %v", domain, domainRecord.IPs)
		resp := buildResponse(r, dns.RcodeSuccess, domainRecord.IPs)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	ips, err := resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}
	log.Infof("Returning resolved IPs for domain %s: %v", domain, ips)
	resp := buildResponse(r, dns.RcodeSuccess, ips)
	w.WriteMsg(resp)

	return dns.RcodeSuccess, nil
}

func (a Ainaa) Name() string { return "ainaa" }

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
