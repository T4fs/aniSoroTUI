# anitui — agents instructions

## Entrypoint

- `cmd/anitui/main.go` — builds to `./build/anitui` via `make build` or `go build ./cmd/anitui`
- `make run` / `go run ./cmd/anitui`

## Build quirks

- **Version injection**: `-X github.com/anitui/anitui/internal/tui.Version=$(VERSION)` via ldflags. Without it, `tui.Version` == `"0.2.0"` and update checks are skipped.
- **Cross-compile**: `make build-{linux,windows}-{amd64,arm64}`, or `CGO_ENABLED=0 GOOS=... GOARCH=... go build ./cmd/anitui`
- **Release**: GoReleaser, triggered on `v*` tags. CGO_ENABLED=0 always. Publishes to AUR, winget, updates flake.nix.

## Test commands

| What | Command |
|------|---------|
| All unit tests | `go test ./...` |
| Integration tests (network) | `go test -tags=integration ./...` |
| Single package | `go test ./internal/scraper/` |
| Lint | `golangci-lint run ./...` |

- The scraper integration test (`TestFullPipeline`) makes live HTTP calls to allanime.day, anilist.co, and jikan.moe. It's NOT tagged `integration` — it runs with `go test ./...` and may fail without network.

## Build tags

- `player_windows.go` → `//go:build windows` — Windows App Paths registry lookup
- `player_darwin.go` → `//go:build darwin` — macOS mdfind + bundle ID
- `player_stubs.go` → `//go:build !windows && !darwin` — stubs for Linux

## Env vars

| Var | Effect |
|-----|--------|
| `ANITUI_DEBUG` | Writes Bubbletea debug log to `/tmp/anitui-debug.log` |
| `ANITUI_PLAYER` | Override detected media player (full path or binary name) |
| `ANITUI_NO_UPDATE` | Skips auto-update check (treats as package-manager install) |

## Architecture

```
cmd/anitui/main.go  ──►  internal/tui/   (Bubbletea model + view)
                            │
                    internal/scraper/   (Scraper interface + AllanimeScraper)
                            │
                    internal/player/    (findPlayer → platform-specific discovery)
                    internal/update/    (self-update via GitHub releases)
                    internal/models/    (Anime, Episode, VideoSource structs)
```

- `Scraper` interface: `Search(query, dub) → []Anime`, `GetEpisodes(url, dub) → []Episode`, `GetVideoURL(url, dub) → []VideoSource`
- `UnifiedScraper` fans out to multiple Scrapers concurrently, merges results. Currently only `AllanimeScraper` is registered.
- `AllanimeScraper` fetches from `allanime.day`, decodes hex-obfuscated paths, decrypts AES video URLs.
- AniList GraphQL (`graphql.anilist.co`) + Jikan REST (`api.jikan.moe/v4`) enrich episode titles and English names. Both use 10s client timeouts.
- `SaveDubPref()`/`LoadDubPref()` in `allanime.go` persist the sub/dub toggle to the `prefs` bbolt bucket.
- `truncate()` in `view.go` collapses all whitespace sequences (newlines, multiple spaces) to single spaces using strings.Fields, and uses rune-aware slicing for proper Unicode truncation.
- `ControlStyle` in `styles.go` provides bold accent-colour styling for interactive controls in the watching screen.
- `formatStatus()` in `view.go` maps AniList status enums (`FINISHED`, `RELEASING`, etc.) to readable labels.
- Episode number sanitization strips `"EP "` and `"Episode "` prefixes from raw API numbers in both `allanime.go` and `view.go` (watching screen).
- `fetchAnimeDetails()` in `anilist.go` enriches search results with `EpisodeCount`, `Score`, `Genres`, `Year`, `Synopsis`, `Studio`, and `Status` via AniList GraphQL.
- `viewEpisodes()` renders a rich header with metadata line (type, year, score, studio, genres, status) and collapsible synopsis (space to toggle, auto-collapses on episode navigation).
- `viewResults()` shows full-row background highlight on selected item (width-extended for box effect).
- `showHelp` / `renderHelpPopup()` / `overlayHelp()` provide a `?` key popup with screen-specific keybinding reference, overlaid on current content.
- Cache TTL: 30-minute expiry on `anime_details` entries; stale entries are refetched.
- `anime_details` bbolt bucket caches enriched Anime structs (includes Studio, Status fields).

## Cache (bbolt)

- Cache DB auto-opened in `init()` at `~/.cache/anitui/cache.db` (Linux/macOS) or `%LOCALAPPDATA%/anitui/cache.db` (Windows).
- Three buckets: `show_names` (enriched English titles per show ID), `episode_titles` (AniList episode titles per show URL), `anime_details` (enriched Anime structs with Studio/Status, 30-min TTL), and `prefs` (user preferences).
- `prefs` bucket stores key-value preferences. Currently only `dub` (1/0) — persisted sub/dub toggle via `SaveDubPref()`/`LoadDubPref()`.
- `LoadDubPref()` is called in `NewModel()` at startup; `SaveDubPref()` is called whenever the user toggles dub mode (`d` key in watching screen).
- If bbolt open fails, `db` is `nil` — caches degrade to in-memory-only (no persistence, no crash).
- For testing/reset: `rm ~/.cache/anitui/cache.db`

## Media player detection

1. `exec.LookPath` on all platforms (fast path)
2. macOS: mdfind by bundle ID (`io.mpv`, `com.colliderli.iina`, `org.videolan.vlc`) with 5s timeout
3. Windows: App Paths registry (`HKLM` + `HKCU`)

## Nix

- Dev shell: `nix develop` gives `go_1_25`, `gopls`, `gotools`
- Build: `nix build .` (vendorHash = null, so it's local builds only)
