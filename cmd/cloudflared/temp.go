package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/cloudflare/cloudflared/i18n"
	"github.com/cloudflare/cloudflared/cmd/cloudflared/tunnel"
)

func buildTempCommand() *cli.Command {
	return &cli.Command{
		Name:  "temp",
		Usage: i18n.T("Quick temporary tunnel: temp <addr> <protocol> <port>", "\u5feb\u901f\u4e34\u65f6\u96a7\u9053: temp <\u5730\u5740> <\u534f\u8bae> <\u7aef\u53e3>"),
		UsageText: i18n.T(
			"cloudflared temp <addr> <protocol> <port>\n\n"+
				"   Examples:\n"+
				"     cloudflared temp local tcp 22      => tcp://localhost:22\n"+
				"     cloudflared temp local http 8080   => http://localhost:8080\n"+
				"     cloudflared temp 1.2.3.4 https 443 => https://1.2.3.4:443\n\n"+
				"   With proxy:\n"+
				"     cloudflared temp local tcp 22 --proxy socks5://127.0.0.1:1080\n"+
				"     cloudflared temp local tcp 22 -x socks5://user:pass@127.0.0.1:1080\n\n"+
				"   'local' is an alias for 'localhost'",
			"cloudflared temp <\u5730\u5740> <\u534f\u8bae> <\u7aef\u53e3>\n\n"+
				"   \u793a\u4f8b:\n"+
				"     cloudflared temp local tcp 22      => tcp://localhost:22\n"+
				"     cloudflared temp local http 8080   => http://localhost:8080\n"+
				"     cloudflared temp 1.2.3.4 https 443 => https://1.2.3.4:443\n\n"+
				"   'local' \u662f 'localhost' \u7684\u522b\u540d"),
		Description: i18n.T(
			"Create a temporary tunnel with a simple syntax.\n"+
				"   The 'addr' can be a hostname or IP. Use 'local' as a shorthand for 'localhost'.\n"+
				"   The 'protocol' can be: http, https, tcp, udp, etc.\n"+
				"   The 'port' is the port number of the local service.",
			"\u4f7f\u7528\u7b80\u5355\u8bed\u6cd5\u521b\u5efa\u4e34\u65f6\u96a7\u9053\u3002\n"+
				"   'addr' \u53ef\u4ee5\u662f\u4e3b\u673a\u540d\u6216 IP \u5730\u5740\u3002\u4f7f\u7528 'local' \u4f5c\u4e3a 'localhost' \u7684\u7b80\u5199\u3002\n"+
				"   'protocol' \u53ef\u4ee5\u662f: http, https, tcp, udp \u7b49\u3002\n"+
				"   'port' \u662f\u672c\u5730\u670d\u52a1\u7684\u7aef\u53e3\u53f7\u3002"),
		Flags: tunnel.Flags(),
		Action: func(c *cli.Context) error {
			// urfave/cli/v2 stops parsing flags after the first positional arg.
			// Manually scan os.Args for --proxy/-x and --protocol/-p that may
			// appear after positional args (e.g., "temp local tcp 2222 -x ...").
			manualFlagScan(c)

			if c.NArg() < 3 {
				fmt.Fprintln(os.Stderr, i18n.T(
					"Usage: cloudflared temp <addr> <protocol> <port>",
					"\u7528\u6cd5: cloudflared temp <\u5730\u5740> <\u534f\u8bae> <\u7aef\u53e3>"))
				fmt.Fprintln(os.Stderr, i18n.T(
					"Example: cloudflared temp local tcp 22",
					"\u793a\u4f8b: cloudflared temp local tcp 22"))
				return errors.New(i18n.T(
					"temp command requires exactly 3 arguments: <addr> <protocol> <port>",
					"temp \u547d\u4ee4\u9700\u8981 3 \u4e2a\u53c2\u6570: <\u5730\u5740> <\u534f\u8bae> <\u7aef\u53e3>"))
			}
			addr := c.Args().Get(0)
			protocol := strings.ToLower(c.Args().Get(1))
			port := c.Args().Get(2)

			if addr == "local" {
				addr = "localhost"
			}

			tunnelURL := fmt.Sprintf("%s://%s:%s", protocol, addr, port)
			fmt.Println(i18n.T(
				fmt.Sprintf("Creating temporary tunnel for: %s", tunnelURL),
				fmt.Sprintf("\u6b63\u5728\u4e3a %s \u521b\u5efa\u4e34\u65f6\u96a7\u9053...", tunnelURL)))

			if err := c.Set("url", tunnelURL); err != nil {
				return fmt.Errorf("failed to set tunnel URL: %w", err)
			}
			fmt.Println("DEBUG temp.go: proxy=" + c.String("proxy"))
			return tunnel.TunnelCommand(c)
		},
	}
}

// manualFlagScan scans os.Args for flags that urfave/cli may have missed
// when they appear after positional arguments.
// e.g., "temp local tcp 2222 -x socks5://..." where -x is after positional args.
func manualFlagScan(c *cli.Context) {
	args := os.Args
	flagMap := map[string]string{
		"--proxy":    "proxy",
		"-x":         "proxy",
		"--protocol": "protocol",
		"-p":         "protocol",
	}
	for i := 0; i < len(args)-1; i++ {
		// Handle "--proxy value" and "-x value" format
		if flagName, ok := flagMap[args[i]]; ok {
			if c.String(flagName) == "" {
				_ = c.Set(flagName, args[i+1])
			}
		}
		// Handle "--proxy=value" format
		for prefix, flagName := range flagMap {
			if strings.HasPrefix(args[i], prefix+"=") {
				val := strings.SplitN(args[i], "=", 2)[1]
				if c.String(flagName) == "" {
					_ = c.Set(flagName, val)
				}
			}
		}
	}
}
