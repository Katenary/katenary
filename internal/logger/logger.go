// Package logger provides simple logging functions with icons and colors.
package logger

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

func Info(msg ...any) {
	message("", IconInfo, msg...)
}

// Warn prints a warning message
func Warn(msg ...any) {
	orange := "\033[38;5;214m"
	message(orange, IconWarning, msg...)
}

func Success(msg ...any) {
	green := "\033[38;5;34m"
	message(green, IconSuccess, msg...)
}

func Failure(msg ...any) {
	red := "\033[38;5;196m"
	message(red, IconFailure, msg...)
}

func Log(icon Icon, msg ...any) {
	message("", icon, msg...)
}
