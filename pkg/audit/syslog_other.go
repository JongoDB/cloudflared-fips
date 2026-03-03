//go:build !linux

package audit

import (
	"fmt"
	"os"
)

// WithSyslog is a no-op on non-Linux platforms where log/syslog is unavailable.
func WithSyslog(network, addr string) Option {
	return func(_ *AuditLogger) {
		fmt.Fprintf(os.Stderr, "audit: syslog not supported on this platform\n")
	}
}
