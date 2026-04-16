param(
    [string]$Sequence = "gegeguoujijay"
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
        throw "Default clang64 compiler not found: $defaultClang"
    }

    $env:PATH = $defaultClang64Bin + [System.IO.Path]::PathSeparator + $env:PATH
    $env:CC = $defaultClang
    $env:MOQI_C_COMPILER = $defaultClang
    Write-Host ("[INFO] Defaulted to clang64 compiler: {0}" -f $defaultClang)
}

Set-DefaultClang64

$env:CGO_ENABLED = "1"
$env:MOQI_REAL_BERT_SEQUENCE = "1"
$env:MOQI_REAL_BERT_SEQUENCE_INPUT = $Sequence
$env:MOQI_REAL_BERT_USER_DIR = "C:\Users\gbl\AppData\Roaming\Moqi\Rime"

Push-Location (Join-Path $PSScriptRoot "..")
try {
    go test ./input_methods/rime -run '^TestRealBertSequence_gegeguoujijay$' -count=1 -v
}
finally {
    Pop-Location
}
