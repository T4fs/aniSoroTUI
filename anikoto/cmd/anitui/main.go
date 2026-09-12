package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anitui/anitui/internal/scraper"
	"github.com/anitui/anitui/internal/tui"
	"github.com/anitui/anitui/internal/update"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if os.Getenv("ANIKOTO_DEBUG") != "" {
		f, err := tea.LogToFile("/tmp/anikoto-debug.log", "anikoto")
		if err != nil {
			fmt.Fprintf(os.Stderr, "debug log setup: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	update.Cleanup()

	if isReleaseBuild() && !update.IsPackageManagerInstall() && checkForUpdates() {
		return
	}

	anidb := scraper.NewAnidbScraper()
	unified := scraper.NewUnifiedScraper(anidb)

	model := tui.NewModel(unified)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "AniSoro error: %v\n", err)
		os.Exit(1)
	}
}

func checkForUpdates() bool {
	fmt.Print("Checking for updates...")
	newVersion, err := update.Check(tui.Version)

	if err != nil {
		fmt.Printf("\r\033[K! Cannot check for updates (%v)\n", err)
		fmt.Println("  Continuing with current version...")
		time.Sleep(time.Second)
		return false
	}

	if newVersion == "" {
		fmt.Printf("\r\033[K~ v%s is up to date\n", tui.Version)
		time.Sleep(400 * time.Millisecond)
		return false
	}

	fmt.Printf("\r\033[Kv%s available (current v%s)\n", newVersion, tui.Version)
	if !update.IsWritable() {
		fmt.Println("  Auto-update requires sudo. To update manually:")
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/typechecks/anitui/main/scripts/install.sh | sudo sh")
		fmt.Println("  Continuing with current version...")
		time.Sleep(time.Second)
		return false
	}

	if err := update.Apply(newVersion); err != nil {
		fmt.Printf("\r\033[K! Update failed: %v\n", err)
		fmt.Println("  Continuing with current version...")
		time.Sleep(time.Second)
		return false
	}

	fmt.Printf("\r\033[KUpdated to v%s. Relaunching...\n", newVersion)
	time.Sleep(300 * time.Millisecond)
	update.Relaunch()
	return true
}

func isReleaseBuild() bool {
	return tui.Version != "dev" && !strings.Contains(tui.Version, "-")
}
