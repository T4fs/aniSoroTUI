package update

import (
	"os"
	"strings"
)

const pkgMarker = "/usr/share/anitui/.package-manager"

func IsPackageManagerInstall() bool {
	if os.Getenv("ANISORO_NO_UPDATE") != "" {
		return true
	}

	if _, err := os.Stat(pkgMarker); err == nil {
		return true
	}

	exe, err := os.Executable()
	if err == nil && strings.HasPrefix(exe, "/nix/store/") {
		return true
	}

	return false
}
