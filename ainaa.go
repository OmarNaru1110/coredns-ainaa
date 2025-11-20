package ainaa

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
)

const (
	tableName = "AinaaDomains"
	name      = "ainaa"
)

type Ainaa struct {
	Next       plugin.Handler
	Cache      CacheRepository
	Persistent PersistentRepository
	Resolver   Resolver
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

	log.Debugf("Received query for domain: %s", domain)

	// 1. Check Cache
	if cachedVal, err := a.Cache.Get(ctx, domain); err == nil {
		return a.handleCacheHit(w, r, domain, cachedVal)
	}

	// 2. Check Persistent Storage
	log.Debugf("Cache miss for domain: %s, looking up in Persistent Storage", domain)
	if domainRecord, err := a.Persistent.Get(ctx, domain); err == nil {
		return a.handlePersistentHit(ctx, w, r, domain, domainRecord)
	}

	// 3. Handle Miss (Fresh Lookup)
	log.Debugf("Domain %s not found in Persistent Storage, performing fresh lookup", domain)
	return a.handleMiss(ctx, w, r, domain)
}

func (a Ainaa) handleCacheHit(w dns.ResponseWriter, r *dns.Msg, domain string, cachedVal CachedDomain) (int, error) {
	log.Debugf("Cache hit for domain: %s with status: %d", domain, cachedVal.Status)
	if cachedVal.Status != 0 {
		log.Debugf("Domain %s is blocked with status: %d", domain, cachedVal.Status)
		resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
		w.WriteMsg(resp)
		return dns.RcodeNameError, nil
	}

	log.Debugf("Cache hit for domain: %s with IPs: %v", domain, cachedVal.IPs)
	if cachedVal.IPs != nil {
		log.Debugf("Serving cached IPs for domain: %s", domain)
		resp := buildResponse(r, dns.RcodeSuccess, cachedVal.IPs)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	log.Debugf("No IPs cached for domain: %s, performing fresh lookup", domain)
	ips, err := a.Resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}
	resp := buildResponse(r, dns.RcodeSuccess, ips)
	w.WriteMsg(resp)
	return dns.RcodeSuccess, nil
}

func (a Ainaa) handlePersistentHit(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string, domainRecord DomainRecord) (int, error) {
	log.Debugf("Domain %s found in Persistent Storage with status: %d", domain, domainRecord.Status)

	if domainRecord.Status != 0 {
		// Update Cache with blocked status
		a.Cache.Set(ctx, domain, CachedDomain{Status: domainRecord.Status, IPs: nil})
		log.Debugf("Domain %s is blocked with status: %d", domain, domainRecord.Status)
		resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
		w.WriteMsg(resp)
		return dns.RcodeNameError, nil
	}

	if domainRecord.IPs != nil {
		// Update Cache with IPs
		a.Cache.Set(ctx, domain, CachedDomain{Status: domainRecord.Status, IPs: domainRecord.IPs})
		log.Debugf("Serving Persistent Storage IPs for domain: %s", domain)
		resp := buildResponse(r, dns.RcodeSuccess, domainRecord.IPs)
		w.WriteMsg(resp)
		return dns.RcodeSuccess, nil
	}

	log.Debugf("Performing fresh lookup for domain: %s", domain)
	ips, err := a.Resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}

	// Update Cache with status only (no IPs)
	a.Cache.Set(ctx, domain, CachedDomain{Status: domainRecord.Status, IPs: nil})

	resp := buildResponse(r, dns.RcodeSuccess, ips)
	w.WriteMsg(resp)

	return dns.RcodeSuccess, nil
}

func (a Ainaa) handleMiss(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, domain string) (int, error) {
	ips, err := a.Resolver.Lookup(domain)
	if err != nil {
		log.Errorf("Error looking up domain %s: %v", domain, err)
		return dns.RcodeServerFailure, err
	}

	newDomainRec := DomainRecord{
		Domain:    domain,
		CreatedAt: time.Now().UTC(),
	}
	newCachedRec := CachedDomain{}

	isBlocked := false
	if resolver, ok := a.Resolver.(interface {
		IsBlockedDomain(map[string][]string) bool
	}); ok {
		isBlocked = resolver.IsBlockedDomain(ips)
	}

	if isBlocked {
		log.Debugf("Domain %s is blocked based on resolver lookup", domain)
		envStatus, _ := strconv.Atoi(os.Getenv("STATUS"))
		newDomainRec.Status = envStatus
		newCachedRec.Status = envStatus
		a.Persistent.Save(ctx, newDomainRec)
		a.Cache.Set(ctx, domain, newCachedRec)
		resp := buildResponse(r, dns.RcodeNameError, blockedIPs)
		w.WriteMsg(resp)
		return dns.RcodeNameError, nil
	}

	log.Debugf("Domain %s is allowed, storing in database and cache", domain)
	newDomainRec.Status = 0
	newCachedRec.Status = 0
	a.Persistent.Save(ctx, newDomainRec)
	a.Cache.Set(ctx, domain, newCachedRec)
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
