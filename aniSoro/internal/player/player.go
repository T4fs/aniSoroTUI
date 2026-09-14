package player

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var playerPriority = []string{"mpv", "iina", "vlc", "haruna"}

func DetectPlayer() string {
	if envPlayer := os.Getenv("ANISORO_PLAYER"); envPlayer != "" {
		return envPlayer
	}
	for _, player := range playerPriority {
		if path := findPlayer(player); path != "" {
			return path
		}
	}
	return ""
}

func Play(url string) error {
	player := DetectPlayer()
	if player == "" {
		return fmt.Errorf("no supported video player found. Please install mpv, vlc, iina (macOS), or haruna")
	}

	var args []string

	base := strings.ToLower(fileName(player))
	switch {
	case strings.Contains(base, "mpv"):
		args = []string{
			"--really-quiet",
			"--no-terminal",
			"--force-window=yes",
			url,
		}
	case strings.Contains(base, "iina"):
		args = []string{"--no-stdin", "--keep-running", url}
	case strings.Contains(base, "vlc"):
		args = []string{"--quiet", "--play-and-exit", url}
	case strings.Contains(base, "haruna"):
		args = []string{url}
	default:
		args = []string{url}
	}

	cmd := exec.Command(player, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil

	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", player, err)
	}

	go func() {
		cmd.Wait()
	}()

	return nil
}

func findPlayer(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	if runtime.GOOS == "darwin" && name == "iina" {
		if path, err := exec.LookPath("iina-cli"); err == nil {
			return path
		}
	}

	switch runtime.GOOS {
	case "windows":
		return findOnWindows(name)
	case "darwin":
		return findOnDarwin(name)
	}

	return ""
}

func fileName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	if i := strings.LastIndex(path, `\`); i >= 0 {
		return path[i+1:]
	}
	return path
}
