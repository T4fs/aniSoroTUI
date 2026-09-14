//go:build windows

package player

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

func findOnWindows(name string) string {
	paths := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	keyPath := `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + name + `.exe`

	for _, root := range paths {
		k, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		path, _, err := k.GetStringValue("")
		k.Close()
		if err != nil || path == "" {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func findOnDarwin(name string) string { return "" }
