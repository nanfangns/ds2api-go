package files

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"ds2api/internal/config"
)

const (
	maxRemoteImageURLLen = 8192
	maxRemoteImageBytes  = 32 << 20
	remoteImageTimeout   = 60 * time.Second
)

var blockedImageNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func validPublicImageAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedImageNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func validateRemoteImageURL(raw string) error {
	if len(raw) > maxRemoteImageURLLen {
		return fmt.Errorf("image URL is too long")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("invalid image URL")
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return fmt.Errorf("image URL must use port 80 or 443")
	}
	return nil
}

// Dial resolved public addresses directly, rather than resolving a hostname a
// second time after validation. This also applies to every redirect.
func dialPublicImage(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid image address")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("image host could not be resolved")
	}
	for _, ip := range addresses {
		if !validPublicImageAddress(ip) {
			return nil, fmt.Errorf("image host must resolve to a public address")
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	for _, ip := range addresses {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("image host could not be reached")
}

func fetchRemoteImage(ctx context.Context, raw string) ([]byte, string, error) {
	transport := &http.Transport{DialContext: dialPublicImage, TLSHandshakeTimeout: 10 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   remoteImageTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return fmt.Errorf("too many image redirects")
			}
			return validateRemoteImageURL(req.URL.String())
		},
	}
	return fetchRemoteImageWithClient(ctx, raw, client)
}

func fetchRemoteImageWithClient(ctx context.Context, raw string, client *http.Client) ([]byte, string, error) {
	if err := validateRemoteImageURL(raw); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", fmt.Errorf("invalid image URL")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			config.Logger.Warn("[image] failed to close response", "error", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image")
	}
	if len(data) == 0 || len(data) > maxRemoteImageBytes {
		return nil, "", fmt.Errorf("image must be between 1 byte and %d bytes", maxRemoteImageBytes)
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("URL does not contain a supported image")
	}
	return data, contentType, nil
}
