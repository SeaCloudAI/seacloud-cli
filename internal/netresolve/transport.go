package netresolve

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
)

const EnvHostResolve = "SEACLOUD_HOST_RESOLVE"

func NewTransport(base *http.Transport) *http.Transport {
	hosts := parseHostResolve(os.Getenv(EnvHostResolve))
	if len(hosts) == 0 {
		if base != nil {
			return base.Clone()
		}
		return http.DefaultTransport.(*http.Transport).Clone()
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if base != nil {
		transport = base.Clone()
	}
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	dialContext := transport.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{}
		dialContext = dialer.DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err == nil {
			if ip := hosts[strings.ToLower(host)]; ip != "" {
				address = net.JoinHostPort(ip, port)
			}
		}
		return dialContext(ctx, network, address)
	}
	return transport
}

func parseHostResolve(raw string) map[string]string {
	hosts := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		host, ip, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		host = strings.ToLower(strings.TrimSpace(host))
		ip = strings.TrimSpace(ip)
		if host == "" || net.ParseIP(ip) == nil {
			continue
		}
		hosts[host] = ip
	}
	return hosts
}
