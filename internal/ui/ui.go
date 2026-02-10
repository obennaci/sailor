package ui

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	blue   = color.New(color.FgBlue)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed)
	cyan   = color.New(color.FgCyan, color.Bold)
	bold   = color.New(color.Bold)
	dim    = color.New(color.Faint)
)

func Info(format string, args ...interface{}) {
	blue.Print("ℹ  ")
	fmt.Printf(format+"\n", args...)
}

func Success(format string, args ...interface{}) {
	green.Print("✔  ")
	fmt.Printf(format+"\n", args...)
}

func Warn(format string, args ...interface{}) {
	yellow.Print("⚠  ")
	fmt.Printf(format+"\n", args...)
}

func Error(format string, args ...interface{}) {
	red.Print("✖  ")
	fmt.Fprintf(color.Error, format+"\n", args...)
}

func Header(format string, args ...interface{}) {
	fmt.Println()
	cyan.Printf(format+"\n", args...)
}

func Bold(s string) string {
	return bold.Sprint(s)
}

func Dim(s string) string {
	return dim.Sprint(s)
}

func Green(s string) string {
	return green.Sprint(s)
}

func Red(s string) string {
	return red.Sprint(s)
}

func Yellow(s string) string {
	return yellow.Sprint(s)
}

func Cyan(s string) string {
	return cyan.Sprint(s)
}
