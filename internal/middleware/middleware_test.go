package middleware

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestValidRequestID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"short", false},
		{"req_0123456789abcdef", true},
		{"0123456789ABCDEF-xyz_1", true},
		{"0123456789 header injection", false},
		{"0123456789\n", false},
		{makeLongID(65), false},
		{makeLongID(64), true},
	}
	for _, tc := range cases {
		if got := validRequestID(tc.in); got != tc.want {
			t.Errorf("validRequestID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func makeLongID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func TestRateLimitAllowsBurstThenBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rw := &rateWindow{hits: map[string][]time.Time{}, limit: 3, window: time.Minute}
	for i := 0; i < 3; i++ {
		if !rw.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rw.allow("1.2.3.4") {
		t.Fatal("4th request within window should be blocked")
	}
	// 其他 IP 不受影响
	if !rw.allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestMemoryTokenRevoker(t *testing.T) {
	r := &MemoryTokenRevoker{items: map[string]time.Time{}}
	if r.IsRevoked("jti-1") {
		t.Fatal("unknown jti must not be revoked")
	}
	r.Revoke("jti-1", time.Now().Add(time.Hour))
	if !r.IsRevoked("jti-1") {
		t.Fatal("revoked jti must be rejected")
	}
	// 已过期的吊销条目应自动失效
	r.Revoke("jti-2", time.Now().Add(-time.Minute))
	if r.IsRevoked("jti-2") {
		t.Fatal("expired revocation entry should be ignored")
	}
}
