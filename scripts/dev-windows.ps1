# Windows dev mode. Needs Go 1.23+, Node 20+ and the WebView2 runtime (built into Windows 11).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
  $env:Path += ";" + (Join-Path (go env GOPATH) "bin")
}
wails dev
