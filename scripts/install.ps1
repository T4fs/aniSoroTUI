# aniSoroTUI self-contained installer for a brand-new Windows machine.
# - No Go, no git, no software required (built-in curl.exe + tar/Expand-Archive only).
# - Downloads a standalone Go toolchain, fetches the source as a zip,
#   builds anitui.exe, installs it, and creates an `anikoto` command on PATH.
#
# Usage (copy-paste into PowerShell):
#   irm https://raw.githubusercontent.com/T4fs/aniSoroTUI/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'

function Say($m)  { Write-Host "==> $m" }
function Warn($m) { Write-Host "warn: $m" -ForegroundColor Yellow }
function Die($m)  { Write-Host "error: $m" -ForegroundColor Red; exit 1 }

$AnituiHome = Join-Path $HOME '.anitui'

Say "aniSoroTUI installer for Windows"

# --- detect architecture ---
$GOARCH = 'amd64'
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { $GOARCH = 'arm64' }
Say "Architecture: $GOARCH"

# --- need curl.exe (built into Windows 10 1803+; no install required) ---
if (-not (Get-Command curl.exe -ErrorAction SilentlyContinue)) {
  Die "curl.exe not found. Update to Windows 10 1803 or later."
}

# --- 1. install Go if missing ---
$gobin = Get-Command go -ErrorAction SilentlyContinue
if (-not $gobin) {
  Say "Go not found - downloading a standalone toolchain..."
  New-Item -ItemType Directory -Force -Path $AnituiHome | Out-Null
  $ver = (curl.exe -fsSL https://go.dev/VERSION?m=text) -split "`n" | Select-Object -First 1
  if ([string]::IsNullOrWhiteSpace($ver)) { Die "could not determine the latest Go version." }

  $goParent = Join-Path $HOME 'go-tools'
  $goRoot   = Join-Path $goParent 'go'
  $zipPath  = Join-Path $AnituiHome 'go.zip'

  Say "Downloading Go $ver (windows/$GOARCH)..."
  curl.exe -fsSL "https://go.dev/dl/$ver.windows-$GOARCH.zip" -o $zipPath
  if ($LASTEXITCODE -ne 0) { Die "failed to download Go." }

  Say "Extracting Go to $goRoot..."
  New-Item -ItemType Directory -Force -Path $goParent | Out-Null
  Remove-Item -Recurse -Force $goRoot -ErrorAction SilentlyContinue
  Expand-Archive -Path $zipPath -DestinationPath $goParent -Force
  Remove-Item -Force $zipPath -ErrorAction SilentlyContinue

  $env:Path = "$goRoot\bin;$env:Path"
  Say "Go $ver ready"
} else {
  Say "using existing Go: $(& go version)"
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Die "Go is unavailable after setup." }

# --- 2. fetch the aniSoroTUI source (zip - no git needed) ---
New-Item -ItemType Directory -Force -Path $AnituiHome | Out-Null
Say "Downloading the aniSoroTUI source..."
$srcZip = Join-Path $AnituiHome 'src.zip'
curl.exe -fsSL "https://codeload.github.com/T4fs/aniSoroTUI/zip/refs/heads/main" -o $srcZip
if ($LASTEXITCODE -ne 0) { Die "failed to download the source." }

$srcRoot = Join-Path $AnituiHome 'src'
Remove-Item -Recurse -Force $srcRoot -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $srcRoot | Out-Null
Expand-Archive -Path $srcZip -DestinationPath $srcRoot -Force
Remove-Item -Force $srcZip -ErrorAction SilentlyContinue

$proj = Get-ChildItem -Path $srcRoot -Directory | Where-Object { $_.Name -like '*aniSoroTUI*' } | Select-Object -First 1
if (-not $proj) { Die "could not locate the source after extraction." }
$module = Join-Path $proj.FullName 'anikoto'

# --- 3. build ---
$binDir = Join-Path $AnituiHome 'bin'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Say "Building aniSoroTUI (this can take a minute)..."
$env:CGO_ENABLED = '0'
Push-Location $module
try {
  & go build -trimpath -ldflags='-s -w' -o (Join-Path $binDir 'anitui.exe') ./cmd/anitui
  if ($LASTEXITCODE -ne 0) { Die "build failed." }
} finally {
  Pop-Location
}

# --- 4. install binary + create anikoto command on PATH ---
$exePath = Join-Path $AnituiHome 'anitui.exe'
Copy-Item (Join-Path $binDir 'anitui.exe') $exePath -Force

$shimDir = Join-Path $HOME 'scoop\shims'
New-Item -ItemType Directory -Force -Path $shimDir | Out-Null
$shim = Join-Path $shimDir 'anikoto.cmd'
Set-Content -Path $shim -Value "@`"$exePath`" %*" -Encoding Ascii

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$shimDir*") {
  [Environment]::SetEnvironmentVariable('Path', "$userPath;$shimDir", 'User')
  Warn "added $shimDir to your user PATH - open a NEW terminal to use 'anikoto'"
}

Write-Host ""
Write-Host "aniSoroTUI installed successfully." -ForegroundColor Green
Write-Host "Run 'anikoto' in a new terminal." -ForegroundColor Cyan
