//go:build !ci
// +build !ci

package logger

import "fmt"

func message(color string, icon Icon, msg ...any) {
	fmt.Print(icon, " ", color)
	fmt.Print(msg...)
	fmt.Println(reset)
}
