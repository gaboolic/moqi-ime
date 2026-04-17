param(
    [string] $RimeDllPath = ""
)

$ErrorActionPreference = "Stop"

function Set-DefaultClang64 {
    $requestedCompiler = [Environment]::GetEnvironmentVariable("MOQI_C_COMPILER")
    if ([string]::IsNullOrWhiteSpace($requestedCompiler)) {
        $requestedCompiler = [Environment]::GetEnvironmentVariable("CC")
    }
    if (-not [string]::IsNullOrWhiteSpace($requestedCompiler)) {
        Write-Host ("[INFO] Using existing C compiler setting: {0}" -f $requestedCompiler)
        return
    }

    $clangCommand = Get-Command "clang" -ErrorAction SilentlyContinue
    if ($clangCommand) {
        Write-Host ("[INFO] clang already available on PATH: {0}" -f $clangCommand.Source)
        return
    }

    $defaultClang64Bin = "C:\msys64\clang64\bin"
    $defaultClang = Join-Path $defaultClang64Bin "clang.exe"
    if (-not (Test-Path -LiteralPath $defaultClang)) {
        Write-Host ("[WARN] Default clang64 compiler not found: {0}" -f $defaultClang)
        return
    }

    $pathEntries = @()
    if (-not [string]::IsNullOrWhiteSpace($env:PATH)) {
        $pathEntries = $env:PATH.Split([System.IO.Path]::PathSeparator)
    }
    $alreadyPresent = $false
    foreach ($entry in $pathEntries) {
        if ([string]::Equals($entry.TrimEnd('\'), $defaultClang64Bin.TrimEnd('\'), [System.StringComparison]::OrdinalIgnoreCase)) {
            $alreadyPresent = $true
            break
        }
    }
    if (-not $alreadyPresent) {
        $env:PATH = $defaultClang64Bin + [System.IO.Path]::PathSeparator + $env:PATH
    }
    $env:CC = $defaultClang
    $env:MOQI_C_COMPILER = $defaultClang
    Write-Host ("[INFO] Defaulted to clang64 compiler: {0}" -f $defaultClang)
}

Write-Host "moqi-ime go"
Set-DefaultClang64
if ($RimeDllPath) {
    & (Join-Path $PSScriptRoot "build.ps1") -RimeDllPath $RimeDllPath
}
else {
    & (Join-Path $PSScriptRoot "build.ps1")
}
& (Join-Path $PSScriptRoot "deploy-server.ps1")