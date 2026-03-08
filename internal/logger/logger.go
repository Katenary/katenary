// Package logger provides simple logging functions with icons and colors.
package logger

import (
	"fmt"
	"os"
)

// Icon is a unicode icon
type Icon string

// Icons used in katenary.
const (
	IconSuccess    Icon = "✅"
	IconFailure    Icon = "❌"
	IconWarning    Icon = "❕"
	IconNote       Icon = "📝"
	IconWorld      Icon = "🌐"
	IconPlug       Icon = "🔌"
	IconPackage    Icon = "📦"
	IconCabinet    Icon = "🗄️"
	IconInfo       Icon = "🔵"
	IconSecret     Icon = "🔒"
	IconConfig     Icon = "🔧"
	IconDependency Icon = "🔗"
)

const reset = "\033[0m"

// Print prints a message without icon.
func Print(msg ...any) {
	fmt.Print(msg...)
}

// Printf prints a formatted message without icon.
func Printf(format string, msg ...any) {
	fmt.Printf(format, msg...)
}

// Info prints an informational message.
func Info(msg ...any) {
	message("", IconInfo, msg...)
}

// Infof prints a formatted informational message.
func Infof(format string, msg ...any) {
	message("", IconInfo, fmt.Sprintf(format, msg...))
}

// Warn prints a warning message.
func Warn(msg ...any) {
	orange := "\033[38;5;214m"
	message(orange, IconWarning, msg...)
}

// Warnf prints a formatted warning message.
func Warnf(format string, msg ...any) {
	orange := "\033[38;5;214m"
	message(orange, IconWarning, fmt.Sprintf(format, msg...))
}

// Success prints a success message.
func Success(msg ...any) {
	green := "\033[38;5;34m"
	message(green, IconSuccess, msg...)
}

// Successf prints a formatted success message.
func Successf(format string, msg ...any) {
	green := "\033[38;5;34m"
	message(green, IconSuccess, fmt.Sprintf(format, msg...))
}

// Failure prints a failure message.
func Failure(msg ...any) {
	red := "\033[38;5;196m"
	message(red, IconFailure, msg...)
}

// Failuref prints a formatted failure message.
func Failuref(format string, msg ...any) {
	red := "\033[38;5;196m"
	message(red, IconFailure, fmt.Sprintf(format, msg...))
}

// Log prints a message with a custom icon.
func Log(icon Icon, msg ...any) {
	message("", icon, msg...)
}

// Logf prints a formatted message with a custom icon.
func Logf(icon Icon, format string, msg ...any) {
	message("", icon, fmt.Sprintf(format, msg...))
}

func fatal(red string, icon Icon, msg ...any) {
	fmt.Print(icon, " ", red)
	fmt.Print(msg...)
	fmt.Println(reset)
	os.Exit(1)
}

func fatalf(red string, icon Icon, format string, msg ...any) {
	fatal(red, icon, fmt.Sprintf(format, msg...))
}

// Fatal prints a fatal error message and exits with code 1.
func Fatal(msg ...any) {
	red := "\033[38;5;196m"
	fatal(red, IconFailure, msg...)
}

// Fatalf prints a fatal error message with formatting and exits with code 1.
func Fatalf(format string, msg ...any) {
	red := "\033[38;5;196m"
	fatalf(red, IconFailure, format, msg...)
}
