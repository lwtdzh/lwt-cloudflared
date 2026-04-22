package tunnel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/cloudflare/cloudflared/cmd/cloudflared/flags"
	"github.com/cloudflare/cloudflared/connection"
)

const httpTimeout = 15 * time.Second

const disclaimer = "Thank you for trying Cloudflare Tunnel. Doing so, without a Cloudflare account, is a quick way to experiment and try it out. However, be aware that these account-less Tunnels have no uptime guarantee, are subject to the Cloudflare Online Services Terms of Use (https://www.cloudflare.com/website-terms/), and Cloudflare reserves the right to investigate your use of Tunnels for violations of such terms. If you intend to use Tunnels in production you should use a pre-created named tunnel by following: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps"

// RunQuickTunnel requests a tunnel from the specified service.
// We use this to power quick tunnels on trycloudflare.com, but the
// service is open-source and could be used by anyone.
func RunQuickTunnel(sc *subcommandContext) error {
	sc.log.Info().Msg(disclaimer)

	// When --proxy is set, force http2 BEFORE any network operations
	// SOCKS5/HTTP proxies only support TCP, not UDP/QUIC
	proxyVal := sc.c.String("proxy")
	if proxyVal != "" {
		// Validate proxy URL format
		proxyCheck, parseErr := url.Parse(proxyVal)
		if parseErr != nil {
			sc.log.Error().Msgf("Invalid proxy URL %q: %v\nCorrect format: socks5://[user:password@]host:port", proxyVal, parseErr)
			return fmt.Errorf("invalid proxy URL %q: %w\nCorrect format: socks5://[user:password@]host:port", proxyVal, parseErr)
		}
		// Check for common mistake: socks5://host:port:password (extra colon)
		if strings.Count(proxyCheck.Host, ":") > 1 {
			errMsg := fmt.Sprintf("Invalid proxy URL %q: too many colons in host:port\n"+
				"  Correct formats:\n"+
				"    socks5://host:port\n"+
				"    socks5://user:password@host:port\n"+
				"    http://host:port\n"+
				"    http://user:password@host:port", proxyVal)
			sc.log.Error().Msg(errMsg)
			return fmt.Errorf("%s", errMsg)
		}
		sc.log.Info().Msg("Proxy detected, forcing protocol to http2 before network operations")
		_ = sc.c.Set(flags.Protocol, "http2")
	}

	sc.log.Info().Msg("Requesting new quick Tunnel on trycloudflare.com...")

	transport := &http.Transport{
		TLSHandshakeTimeout:   httpTimeout,
		ResponseHeaderTimeout: httpTimeout,
	}

	// Route API requests through proxy if configured
	if proxyVal != "" {
		proxyURL, err := url.Parse(proxyVal)
		if err != nil {
			return fmt.Errorf("invalid proxy URL %q: %w", proxyVal, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		sc.log.Info().Msgf("Quick tunnel API request will use proxy: %s", proxyVal)
	}

	client := http.Client{
		Transport: transport,
		Timeout:   httpTimeout,
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/tunnel", sc.c.String("quick-service")), nil)
	if err != nil {
		return errors.Wrap(err, "failed to build quick tunnel request")
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", buildInfo.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		if proxyVal != "" {
			sc.log.Error().Err(err).Msgf("Failed to request quick Tunnel via proxy %s (check that the proxy is running and accessible)", proxyVal)
			return fmt.Errorf("failed to request quick Tunnel via proxy %s: %w (check that the proxy is running and accessible)", proxyVal, err)
		}
		return errors.Wrap(err, "failed to request quick Tunnel")
	}
	defer resp.Body.Close()

	// This will read the entire response into memory so we can print it in case of error
	rsp_body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read quick-tunnel response")
	}

	var data QuickTunnelResponse
	if err := json.Unmarshal(rsp_body, &data); err != nil {
		rsp_string := string(rsp_body)
		fields := map[string]interface{}{"status_code": resp.Status}
		sc.log.Err(err).Fields(fields).Msgf("Error unmarshaling QuickTunnel response: %s", rsp_string)
		return errors.Wrap(err, "failed to unmarshal quick Tunnel")
	}

	tunnelID, err := uuid.Parse(data.Result.ID)
	if err != nil {
		return errors.Wrap(err, "failed to parse quick Tunnel ID")
	}

	credentials := connection.Credentials{
		AccountTag:   data.Result.AccountTag,
		TunnelSecret: data.Result.Secret,
		TunnelID:     tunnelID,
	}

	url := data.Result.Hostname
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	for _, line := range AsciiBox([]string{
		"Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):",
		url,
	}, 2) {
		sc.log.Info().Msg(line)
	}

	// For non-proxy mode, default to quic if protocol not explicitly set
	if proxyVal == "" && !sc.c.IsSet(flags.Protocol) {
		_ = sc.c.Set(flags.Protocol, "quic")
	}

	// Override the number of connections used. Quick tunnels shouldn't be used for production usage,
	// so, use a single connection instead.
	_ = sc.c.Set(flags.HaConnections, "1")
	return StartServer(
		sc.c,
		buildInfo,
		&connection.TunnelProperties{Credentials: credentials, QuickTunnelUrl: data.Result.Hostname},
		sc.log,
	)
}

type QuickTunnelResponse struct {
	Success bool
	Result  QuickTunnel
	Errors  []QuickTunnelError
}

type QuickTunnelError struct {
	Code    int
	Message string
}

type QuickTunnel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	AccountTag string `json:"account_tag"`
	Secret     []byte `json:"secret"`
}

// Print out the given lines in a nice ASCII box.
func AsciiBox(lines []string, padding int) (box []string) {
	maxLen := maxLen(lines)
	spacer := strings.Repeat(" ", padding)
	border := "+" + strings.Repeat("-", maxLen+(padding*2)) + "+"
	box = append(box, border)
	for _, line := range lines {
		box = append(box, "|"+spacer+line+strings.Repeat(" ", maxLen-len(line))+spacer+"|")
	}
	box = append(box, border)
	return
}

func maxLen(lines []string) int {
	max := 0
	for _, line := range lines {
		if len(line) > max {
			max = len(line)
		}
	}
	return max
}
