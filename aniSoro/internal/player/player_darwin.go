//go:build darwin

package player

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var playerBundleIDs = map[string]string{
	"io.mpv":              "mpv",
	"com.colliderli.iina": "iina",
	"org.videolan.vlc":    "vlc",
}

func findOnDarwin(name string) string {
	for bundleID, pname := range playerBundleIDs {
		if pname == name {
			return findBundleByID(bundleID, name)
		}
	}
	return findBundleByName(name)
}

func findBundleByID(bundleID, playerName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "mdfind",
		"kMDItemContentType==com.apple.application-bundle && kMDItemCFBundleIdentifier=="+bundleID)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if p := binaryFromBundle(line, playerName); p != "" {
			return p
		}
	}
	return ""
}

func findBundleByName(name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "mdfind", "kMDItemFSName == \""+name+".app\"")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if p := binaryFromBundle(line, name); p != "" {
			return p
		}
	}
	return ""
}

func binaryFromBundle(bundlePath, playerName string) string {
	binaryName := playerName
	if playerName == "iina" {
		binaryName = "iina-cli"
	}
	binaryPath := filepath.Join(bundlePath, "Contents", "MacOS", binaryName)
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath
	}
	return ""
}

func findOnWindows(name string) string { return "" }
