package edgediscovery

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

// DialThroughProxy establishes a TCP connection to the target address through a proxy.
// Supported proxy schemes: socks5, socks5h, http, https.
// For SOCKS5: socks5://host:port or socks5://user:password@host:port
// For HTTP:   http://host:port or http://user:password@host:port
func DialThroughProxy(ctx context.Context, network, addr, proxyURL string) (net.Conn, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
	}

	switch u.Scheme {
	case "socks5", "socks5h":
		return dialSOCKS5(ctx, network, addr, u)
	case "http", "https":
		return dialHTTPConnect(ctx, network, addr, u)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (supported: socks5, http)", u.Scheme)
	}
}

// dialSOCKS5 connects through a SOCKS5 proxy using golang.org/x/net/proxy.
func dialSOCKS5(ctx context.Context, network, addr string, u *url.URL) (net.Conn, error) {
	// Use proxy.FromURL which handles socks5/socks5h and auth
	dialer, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Try context-aware dialing first
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		conn, err := cd.DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 dial to %s via %s failed: %w", addr, u.Host, err)
		}
		return conn, nil
	}

	// Fallback to non-context dial
	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dial to %s via %s failed: %w", addr, u.Host, err)
	}
	return conn, nil
}

// dialHTTPConnect connects through an HTTP proxy using the CONNECT method.
func dialHTTPConnect(ctx context.Context, network, addr string, u *url.URL) (net.Conn, error) {
	// Connect to the proxy server
	proxyAddr := u.Host
	if u.Port() == "" {
		proxyAddr = net.JoinHostPort(u.Hostname(), "8080")
	}

	var d net.Dialer
	proxyConn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HTTP proxy %s: %w", proxyAddr, err)
	}

	// Send CONNECT request
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	// Add proxy authentication if provided
	if u.User != nil {
		connectReq.Header.Set("Proxy-Authorization", "Basic "+basicAuth(u.User))
	}

	if err := connectReq.Write(proxyConn); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to send CONNECT to HTTP proxy: %w", err)
	}

	// Read response
	br := bufio.NewReader(proxyConn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response from HTTP proxy: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		proxyConn.Close()
		return nil, fmt.Errorf("HTTP proxy CONNECT to %s failed with status: %s", addr, resp.Status)
	}

	return proxyConn, nil
}

// basicAuth encodes proxy authentication credentials.
func basicAuth(user *url.Userinfo) string {
	username := user.Username()
	password, _ := user.Password()
	credentials := username + ":" + password
	// Simple base64 encoding without importing encoding/base64
	// to keep dependencies minimal - use net/http internal encoding
	return encodeBase64([]byte(credentials))
}

// encodeBase64 is a minimal base64 encoder.
func encodeBase64(src []byte) string {
	const encode = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	dst := make([]byte, (len(src)+2)/3*4)
	di, si := 0, 0
	n := (len(src) / 3) * 3
	for si < n {
		val := uint(src[si+0])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		dst[di+0] = encode[val>>18&0x3F]
		dst[di+1] = encode[val>>12&0x3F]
		dst[di+2] = encode[val>>6&0x3F]
		dst[di+3] = encode[val&0x3F]
		si += 3
		di += 4
	}
	remain := len(src) - si
	if remain == 0 {
		return string(dst[:di])
	}
	val := uint(src[si+0]) << 16
	if remain == 2 {
		val |= uint(src[si+1]) << 8
	}
	dst[di+0] = encode[val>>18&0x3F]
	dst[di+1] = encode[val>>12&0x3F]
	if remain == 2 {
		dst[di+2] = encode[val>>6&0x3F]
		dst[di+3] = byte('=')
	} else {
		dst[di+2] = byte('=')
		dst[di+3] = byte('=')
	}
	return string(dst[:di+4])
}
