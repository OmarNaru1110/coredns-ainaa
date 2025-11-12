package ainaa

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestSetup_ArgsFail(t *testing.T) {
	c := caddy.NewTestController("dns", `ainaa arg`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected an error, but got none")
	}
}

func TestSetup_RedisFail(t *testing.T) {
	t.Setenv("REDIS_ADDR", "fake")
	c := caddy.NewTestController("dns", `ainaa`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected an error, but got none")
	}
}

func TestSetup_DynamoDBFail(t *testing.T) {
	t.Setenv("REDIS_ADDR", "localhost:6379")
	c := caddy.NewTestController("dns", `ainaa`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected an error, but got none")
	}
}
func TestSetup_Success(t *testing.T) {
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("AWS_ACCESS_KEY_ID", "fake")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fake")
	t.Setenv("AWS_REGION", "us-west-2")
	c := caddy.NewTestController("dns", `ainaa`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}
}
