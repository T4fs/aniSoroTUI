param(
	[string]$InstallDir = "$HOME\.anitui"
)

$ErrorActionPreference = "Stop"
$repo = "typechecks/anitui"

# Kill any running instances before touching files
Get-Process anitui -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500

# Resolve to an absolute literal path (avoids short-path / space issues)
if (-not (Test-Path $InstallDir)) {
	New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}
$InstallDir = (Resolve-Path $InstallDir).Path

Write-Host "Detecting latest version..." -ForegroundColor Cyan
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
$tag = $release.tag_name

if (-not $tag) {
	Write-Error "Could not find latest release tag"
	exit 1
}

$arch = "amd64"
$binaryName = "anitui_windows_$arch.zip"
$asset = $release.assets | Where-Object { $_.name -eq $binaryName }

if (-not $asset) {
	Write-Error "Could not find asset $binaryName for version $tag"
	exit 1
}

$url = $asset.browser_download_url

# Download to a uniquely-named temp folder (avoids short-path / space issues)
$tempDir = Join-Path $env:TEMP "anitui_update_$(Get-Random)"
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
$zipPath = Join-Path $tempDir "anitui.zip"
$extractDir = Join-Path $tempDir "anitui_next"

Write-Host "Downloading anitui $tag..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $url -OutFile $zipPath

if (-not (Test-Path $zipPath)) {
	Write-Error "Download failed: $zipPath not found"
	exit 1
}

Write-Host "Extracting..." -ForegroundColor Cyan
Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

$newExe = Join-Path $extractDir "anitui.exe"
if (-not (Test-Path $newExe)) {
	Write-Error "Extraction failed: anitui.exe not found"
	exit 1
}

# If anitui is still running (update scenario), use a launcher to swap after exit.
# Otherwise (fresh install), copy directly.
$running = Get-Process anitui -ErrorAction SilentlyContinue
if ($running) {
	Write-Host "Detected running instance — deferring file swap..." -ForegroundColor Yellow
	$launcherPath = Join-Path $tempDir "anitui_update.ps1"
	@"
`$ErrorActionPreference = "Stop"
Start-Sleep -Seconds 3
Get-Process anitui -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1
Copy-Item "$extractDir\anitui.exe" "$InstallDir\anitui.exe" -Force
Remove-Item -Recurse -Force "$tempDir" -ErrorAction SilentlyContinue
Start-Process "$InstallDir\anitui.exe"
"@ | Set-Content -Path $launcherPath

	Start-Process powershell -ArgumentList "-ExecutionPolicy Bypass -WindowStyle Hidden -File `"$launcherPath`""
	Write-Host "Launcher started. The app will restart automatically after update." -ForegroundColor Green
} else {
	Write-Host "Installing to $InstallDir..." -ForegroundColor Cyan
	Copy-Item "$newExe" "$InstallDir\anitui.exe" -Force

	if ($InstallDir -eq "$HOME\.anitui") {
		$path = [Environment]::GetEnvironmentVariable("Path", "User")
		if ($path -notlike "*$InstallDir*") {
			Write-Host "Adding $InstallDir to PATH..." -ForegroundColor Cyan
			[Environment]::SetEnvironmentVariable("Path", $path + ";$InstallDir", "User")
			$env:Path += ";$InstallDir"
		}
	}

	Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
	Write-Host "Done! Run 'anitui' to start." -ForegroundColor Green
}
