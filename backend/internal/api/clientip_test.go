package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP_UntrustedForwardedForIsIgnored(t *testing.T) {
	first := resolvedClientIP(t, nil, "198.51.100.10:443", "203.0.113.1")
	second := resolvedClientIP(t, nil, "198.51.100.10:443", "203.0.113.2")
	if first != "198.51.100.10" || second != first {
		t.Fatalf("untrusted peer changed resolved IP: first=%s second=%s", first, second)
	}
}

func TestRF011_LaIPDelAuditLogNoEsFalsificable(t *testing.T) {
	got := resolvedClientIP(t, nil, "198.51.100.20:8443", "192.0.2.99")
	if got != "198.51.100.20" {
		t.Fatalf("audit IP was spoofable: got %s", got)
	}
}

func TestClientIP_XForwardedForFromTrustedProxyUsesRightmostUntrusted(t *testing.T) {
	trusted := prefixes(t, "10.0.0.0/8", "2001:db8:ffff::/48")
	got := resolvedClientIP(t, trusted, "10.0.0.2:443", "198.51.100.1, 10.0.0.3, 10.0.0.4")
	if got != "198.51.100.1" {
		t.Fatalf("resolved IP = %s, want 198.51.100.1", got)
	}
}

func TestClientIP_MalformedInputsFallBackSafely(t *testing.T) {
	got := resolvedClientIP(t, prefixes(t, "10.0.0.0/8"), "10.0.0.2:443", "198.51.100.1, not-an-ip")
	if got != "10.0.0.2" {
		t.Fatalf("malformed XFF resolved IP = %s, want remote peer", got)
	}

	got = resolvedClientIP(t, prefixes(t, "10.0.0.0/8"), "not-an-address", "198.51.100.1")
	if got != "invalid" {
		t.Fatalf("malformed RemoteAddr resolved IP = %s, want invalid", got)
	}
}

func TestClientIP_HandlesIPv6AndMultipleHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	req.RemoteAddr = "[2001:db8:ffff::2%eth0]:443"
	req.Header.Add("X-Forwarded-For", "2001:db8::1")
	req.Header.Add("X-Forwarded-For", "2001:db8:ffff::3")
	got := clientIPFromRequest(t, prefixes(t, "2001:db8:ffff::/48"), req)
	if got != "2001:db8::1" {
		t.Fatalf("resolved IPv6 = %s, want 2001:db8::1", got)
	}
}

func resolvedClientIP(t *testing.T, trusted []netip.Prefix, remoteAddr, xff string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return clientIPFromRequest(t, trusted, req)
}

func clientIPFromRequest(t *testing.T, trusted []netip.Prefix, req *http.Request) string {
	t.Helper()
	var got netip.Addr
	var ok bool
	h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, ok = ClientIPFrom(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !ok {
		return "invalid"
	}
	return got.String()
}

func prefixes(t *testing.T, values ...string) []netip.Prefix {
	t.Helper()
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, prefix)
	}
	return result
}
