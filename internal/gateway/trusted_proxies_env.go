package gateway

import (
	"os"
	"strings"
)

// TrustedProxyCIDRsFromEnv reads TRUSTED_PROXY_CIDRS as a comma-separated list of CIDRs.
// Example: "10.0.0.0/8, 192.168.0.0/16".
func TrustedProxyCIDRsFromEnv() []string {
	v := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

