package api

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPContextKey struct{}

// ClientIP resolves the peer address and only accepts X-Forwarded-For from trusted proxies.
func ClientIP(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	trusted := append([]netip.Prefix(nil), trustedProxies...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP, ok := resolveClientIP(r, trusted)
			if ok {
				r = r.WithContext(context.WithValue(r.Context(), clientIPContextKey{}, clientIP))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPFrom returns the resolved client IP added by ClientIP.
func ClientIPFrom(ctx context.Context) (netip.Addr, bool) {
	ip, ok := ctx.Value(clientIPContextKey{}).(netip.Addr)
	return ip, ok
}

func resolveClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	peer, ok := parseRemoteAddr(r.RemoteAddr)
	if !ok || !isTrusted(peer, trusted) {
		return peer, ok
	}

	forwarded, ok := parseForwardedFor(r.Header.Values("X-Forwarded-For"))
	if !ok || len(forwarded) == 0 {
		return peer, true
	}
	for index := len(forwarded) - 1; index >= 0; index-- {
		if !isTrusted(forwarded[index], trusted) {
			return forwarded[index], true
		}
	}
	return peer, true
}

func parseRemoteAddr(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	return parseAddress(host)
}

func parseForwardedFor(values []string) ([]netip.Addr, bool) {
	var addresses []netip.Addr
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			address, ok := parseAddress(entry)
			if !ok {
				return nil, false
			}
			addresses = append(addresses, address)
		}
	}
	return addresses, true
}

func parseAddress(value string) (netip.Addr, bool) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if host, _, ok := strings.Cut(value, "%"); ok {
		value = host
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func isTrusted(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
