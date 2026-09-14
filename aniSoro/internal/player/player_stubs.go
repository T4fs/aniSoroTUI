//go:build !windows && !darwin

package player

func findOnWindows(name string) string { return "" }

func findOnDarwin(name string) string { return "" }
