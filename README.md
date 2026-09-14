# aniSoroTUI

**watch anime in your terminal**

**website:** [aniSoroTUI](https://t4fs.github.io/aniSoroTUI/)



## features

- 🔎 search anidb.app and browse results without a browser
- 🎙️ sub / dub toggle per episode
- 📶 multiple quality sources (1080p / 720p / 360p) with automatic selection
- 🖱️ mouse support — click the **made by t4fs** button under the search bar to open the author's github
- 🎬 plays in mpv, vlc, iina, or haruna
- 🖥️ cross-platform: linux, macos, windows

## wiki

full documentation, the update log, and bug fixes live in the [**aniSoroTUI wiki**](https://t4fs.github.io/aniSoroTUI/wiki.html) � every update is documented there.

## getting started

the code lives in the [`aniSoro/`](aniSoro) directory.

### from source (any platform)

```bash
cd aniSoro
make build
./build/anitui
```

or directly with Go:

```bash
cd aniSoro
go run ./cmd/anitui
```

### windows

```powershell
cd aniSoro
go build -trimpath -o "$HOME\.anitui\anitui.exe" ./cmd/anitui
```

then run `%USERPROFILE%\.anitui\anitui.exe` — or add an `aniSoro` command to
any terminal (cmd, powershell, git bash) with a tiny shim on your `PATH`:

```powershell
New-Item -ItemType Directory -Force -Path "$HOME\scoop\shims" | Out-Null
Set-Content "$HOME\scoop\shims\aniSoro.cmd" '@"%USERPROFILE%\.anitui\anitui.exe" %*'
```

## controls

### navigation

| key | action |
|-----|--------|
| enter | confirm / select |
| esc | return |
| j / ↓ | down |
| k / ↑ | up |
| g g | jump to top |
| G | jump to bottom |
| ctrl+u / ⌘u | page up |
| ctrl+d / ⌘d | page down |
| / | search |
| ? | toggle help popup |
| ctrl+c / ⌘c | exit |

### episode screen

| key | action |
|-----|--------|
| j / ↓ | down |
| k / ↑ | up |
| enter | play episode |
| space | toggle synopsis expand |
| d | toggle sub / dub |
| esc | back to results |

### watching screen

| key | action |
|-----|--------|
| h / ← | previous episode |
| l / → | next episode |
| r | replay current episode |
| space | replay current episode |
| s | cycle video source |
| d | toggle sub / dub |
| esc | back to episode list |

## player support

aniSoro auto-detects and uses:

1. mpv
2. iina (macOS)
3. vlc
4. haruna

override via the `ANITUI_PLAYER` environment variable.

## platform support

| os | architectures |
|----|---------------|
| linux | amd64, arm64 |
| macos | amd64, arm64 |
| windows | amd64, arm64 |

## project layout

```
aniSoro/
├── cmd/anitui        # entrypoint
├── internal/scraper  # anidb.app source
├── internal/tui      # bubbletea ui, home screen, credit button
├── internal/player   # mpv / vlc / iina / haruna detection
└── internal/update   # self-update
```

## credits

- made by [t4fs](https://github.com/T4fs) — the in-app **made by t4fs**
  button is clickable, try it
- a inspired fork of [anitui](https://github.com/typechecks/anitui)
  by [typechecks](https://github.com/typechecks), with a fresh source
  (**anidb.app**) and home-screen polish

## license

GPL-3.0 — see [`aniSoro/LICENSE`](aniSoro/LICENSE).
