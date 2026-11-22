# Builds Googleplex for 64-bit Windows: build\bin\googleplex.exe and an
# installer, build\bin\Googleplex-amd64-installer.exe.
#
# Needs Go 1.26+, Node 20+, and NSIS for the installer:
#   winget install NSIS.NSIS     (then open a new terminal so makensis is on PATH)
# The installer checks for Microsoft's WebView2 runtime (built into Windows 11
# and current Windows 10) and downloads its small bootstrapper if it's missing.
# The exe isn't code-signed, so SmartScreen may warn on first run: choose
# "More info", then "Run anyway".
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

foreach ($tool in "go", "node") {
  if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "Missing $tool. Install it, then open a new terminal." }
}
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
  $env:Path += ";" + (Join-Path (go env GOPATH) "bin")
}

$nsis = [bool](Get-Command makensis -ErrorAction SilentlyContinue)
if ($nsis) {
  wails build -clean -trimpath -platform windows/amd64 -nsis
} else {
  Write-Warning "NSIS not found: building the exe only. Install NSIS for the installer (winget install NSIS.NSIS)."
  wails build -clean -trimpath -platform windows/amd64
}
if ($LASTEXITCODE -ne 0) { throw "wails build failed" }

Get-ChildItem build\bin | ForEach-Object { Write-Host "Built build\bin\$($_.Name)" }
