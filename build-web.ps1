$ErrorActionPreference = 'Stop'

$go = 'C:\Users\admin\tools\go\bin\go.exe'
if (!(Test-Path $go)) {
  $go = 'go'
}

$goroot = & $go env GOROOT
if (-not $goroot) {
  throw 'GOROOT not found'
}

New-Item -ItemType Directory -Force -Path .\web | Out-Null
Copy-Item -Force (Join-Path $goroot 'lib\wasm\wasm_exec.js') .\web\wasm_exec.js

$env:GOOS = 'js'
$env:GOARCH = 'wasm'
& $go build -o .\web\game.wasm .
if ($LASTEXITCODE -ne 0) {
  throw "WASM build failed: $LASTEXITCODE"
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Write-Host 'Web build done:'
Write-Host '  web\index.html'
Write-Host '  web\wasm_exec.js'
Write-Host '  web\game.wasm'
Write-Host ''
Write-Host 'Run local server (example):'
Write-Host '  python -m http.server 8080 -d web'
