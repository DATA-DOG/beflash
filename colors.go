// +build !windows

package main

import "fmt"

func green(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func red(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func cyan(s string) string {
	return fmt.Sprintf("\033[36m%s\033[0m", s)
}

func yellow(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}
