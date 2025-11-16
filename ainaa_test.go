package ainaa

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

func TestAinaa(t *testing.T) {
	a := Ainaa{Next: test.ErrorHandler()}

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	a.ServeDNS(ctx, rec, r)
}
