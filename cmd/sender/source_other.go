//go:build !windows && !darwin

package main

import "fmt"

var captureMethods = []string{"pipe"}

func openSource(_ string, method string, display, fps int, encoder string) (*source, error) {
	if method != "" && method != "pipe" {
		return nil, fmt.Errorf("unknown capture method %q (use pipe)", method)
	}
	return pipeSource(display, fps, encoder)
}

func listDisplays(string) error {
	listScreenshotDisplays()
	return nil
}
