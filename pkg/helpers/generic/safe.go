package generic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// ErrBlockedAddress is returned when a URL resolves to a non-public address.
var ErrBlockedAddress = errors.New("address is not allowed")

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

var safeHTTPClient = newSafeHTTPClient()

func newSafeHTTPClient() *http.Client {
	transport := newHTTPClient().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}
}

// readResourceSafe reads an HTTP(S) resource while refusing to connect to
// loopback, private, link-local or otherwise reserved addresses.
func readResourceSafe(uri string) (io.ReadCloser, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return nil, fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}

	if u.Host == "" {
		return nil, errors.New("URL has no host")
	}

	return readHTTPWithClient(u, nil, safeHTTPClient)
}

func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}

	var lastErr error

	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return nil, fmt.Errorf("%w: %s", ErrBlockedAddress, ip.IP)
		}

		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}

	addr = addr.Unmap()

	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
