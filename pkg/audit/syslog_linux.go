//go:build linux

package audit

import (
	"fmt"
	"log/syslog"
	"os"
)

// WithSyslog enables syslog forwarding. Network is "tcp" or "udp";
// addr is e.g. "siem:514".
func WithSyslog(network, addr string) Option {
	return func(al *AuditLogger) {
		w, err := syslog.Dial(network, addr, syslog.LOG_INFO|syslog.LOG_AUTH, "cloudflared-fips")
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit: syslog dial %s/%s failed: %v\n", network, addr, err)
			return
		}
		al.syslogW = w
	}
}
