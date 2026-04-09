echo "moqi-ime go"
& (Join-Path $PSScriptRoot "build.ps1")
& (Join-Path $PSScriptRoot "deploy-server.ps1")