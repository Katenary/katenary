//go:build ci
// +build ci

package logger

// CI should be no-op
func message(color string, icon Icon, msg ...any) {
	// no-op
}
