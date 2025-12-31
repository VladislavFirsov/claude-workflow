// Package audit provides structured logging for execution audit.
package audit

import "log"

// Log writes an audit event with [AUDIT] prefix.
// Format should use key=value pairs for structured logging.
func Log(format string, args ...interface{}) {
	log.Printf("[AUDIT] "+format, args...)
}
