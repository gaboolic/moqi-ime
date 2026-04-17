#Requires -Version 5.1
<#
.SYNOPSIS
  Build the moqi-ime runtime package.

.PARAMETER RepoRoot
  Root of moqi-ime (defaults to the parent directory of this script).

.PARAMETER BuildRoot
  Build output directory (default: scripts\build).

.PARAMETER PackageDir
  Packaged runtime directory (default: scripts\build\moqi-ime).

.PARAMETER CCompiler
  Optional explicit C compiler path for CGO builds (clang.exe, gcc.exe, or cl.exe).
#>
param(
    [string] $RepoRoot = "",
    [string] $BuildRoot = "",
    [string] $PackageDir = "",
    [string] $CCompiler = "",
    [string] $RimeDllPath = ""
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string] $Title)

    Write-Host ""
    Write-Host "============================================"
    Write-Host $Title
    Write-Host "============================================"
    Write-Host ""
}

function Ensure-Directory {
    param([string] $Path)

    New-Item -ItemType Directory -Path $Path -Force | Out-Null
}

function Remove-IfExists {
    param([string] $Path)

    if (Test-Path -LiteralPath $Path) {
        Remove-Item -LiteralPath $Path -Recurse -Force
    }
}

function Invoke-External {
    param(
        [string] $FilePath,
        [string[]] $ArgumentList,
        [switch] $IgnoreExitCode
    )

    Write-Host ">> $FilePath $($ArgumentList -join ' ')"
    & $FilePath @ArgumentList
    $exitCode = $LASTEXITCODE
    if (-not $IgnoreExitCode -and $exitCode -ne 0) {
        throw "Command failed with exit code ${exitCode}: $FilePath"
    }
    return $exitCode
}

function Get-GoToolExecutablePath {
    param([string] $ToolName)

    $command = Get-Command $ToolName -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $goBin = (& go env GOBIN).Trim()
    if (-not $goBin) {
        $goPath = (& go env GOPATH).Trim()
        if (-not $goPath) {
            throw "Unable to resolve GOPATH for Go tool installation."
        }

        $firstGoPath = $goPath.Split([System.IO.Path]::PathSeparator)[0]
        $goBin = Join-Path $firstGoPath "bin"
    }

    return (Join-Path $goBin ($ToolName + ".exe"))
}

function Get-GoTool {
    param(
        [string] $ToolName,
        [string] $ModuleAtVersion
    )

    $toolPath = Get-GoToolExecutablePath -ToolName $ToolName
    if (Test-Path -LiteralPath $toolPath) {
        return $toolPath
    }

    Write-Host "[INFO] Installing Go tool: $ModuleAtVersion"
    $null = Invoke-External -FilePath "go" -ArgumentList @("install", $ModuleAtVersion)

    $toolPath = Get-GoToolExecutablePath -ToolName $ToolName
    if (-not (Test-Path -LiteralPath $toolPath)) {
        throw "Installed Go tool was not found: $toolPath"
    }

    return $toolPath
}

function Copy-DirectoryContents {
    param(
        [string] $Source,
        [string] $Destination
    )

    Ensure-Directory -Path $Destination
    Copy-Item -Path (Join-Path $Source "*") -Destination $Destination -Recurse -Force
}

function Get-AvailableCCompiler {
    param([string] $RequestedCompiler = "")

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($RequestedCompiler)) {
        $candidates += $RequestedCompiler
    }
    foreach ($name in @("MOQI_C_COMPILER", "CC")) {
        $value = [Environment]::GetEnvironmentVariable($name)
        if (-not [string]::IsNullOrWhiteSpace($value)) {
            $candidates += $value
        }
    }
    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        if (Test-Path -LiteralPath $candidate) {
            return [System.IO.Path]::GetFullPath($candidate)
        }
        $command = Get-Command $candidate -ErrorAction SilentlyContinue
        if ($command) {
            return $command.Name
        }
    }
    foreach ($name in @("gcc", "clang", "clang-cl", "cl")) {
        $command = Get-Command $name -ErrorAction SilentlyContinue
        if ($command) {
            return $name
        }
    }
    return ""
}

function Resolve-FirstExistingPath {
    param([string[]] $Candidates)

    foreach ($candidate in $Candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        if (Test-Path -LiteralPath $candidate) {
            return [System.IO.Path]::GetFullPath($candidate)
        }
    }
    return ""
}

function Resolve-BertGrammarPluginDll {
    param(
        [string] $PluginRepoRoot,
        [string] $LibrimeRoot,
        [string] $BuildRoot
    )

    $explicitPath = [Environment]::GetEnvironmentVariable("MOQI_BERT_GRAMMAR_DLL")
    return Resolve-FirstExistingPath @(
        $explicitPath,
        (Join-Path $PluginRepoRoot "build\bin\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $PluginRepoRoot "build\lib\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $PluginRepoRoot "build\bin\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $PluginRepoRoot "build\lib\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build-bert-grammar\bin\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build-bert-grammar\lib\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build-bert-grammar\bin\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build-bert-grammar\lib\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build\bin\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build\lib\rime-plugins\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build\bin\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $LibrimeRoot "build\lib\rime-plugins\Release\rime-bert-grammar.dll"),
        (Join-Path $BuildRoot "rime-plugins\rime-bert-grammar.dll")
    )
}

function Resolve-RimeDllPath {
    param(
        [string] $RequestedPath,
        [string] $RepoRoot,
        [string] $BuildRoot
    )

    $explicitPath = $RequestedPath
    if (-not $explicitPath) {
        $explicitPath = [Environment]::GetEnvironmentVariable("MOQI_RIME_DLL")
    }

    $programFilesX86 = [Environment]::GetEnvironmentVariable("ProgramFiles(x86)")
    $installedRimeDll = ""
    if (-not [string]::IsNullOrWhiteSpace($programFilesX86)) {
        $installedRimeDll = Join-Path $programFilesX86 "MoqiIM\moqi-ime\input_methods\rime\rime.dll"
    }

    return Resolve-FirstExistingPath @(
        $explicitPath,
        $installedRimeDll,
        (Join-Path $BuildRoot "rime.dll"),
        (Join-Path $RepoRoot "input_methods\rime\rime.dll")
    )
}

function Test-OnnxRuntimeSdkRoot {
    param([string] $Root)

    if ([string]::IsNullOrWhiteSpace($Root)) {
        return $false
    }

    $resolved = [System.IO.Path]::GetFullPath($Root)
    return (
        (Test-Path -LiteralPath (Join-Path $resolved "include\onnxruntime_cxx_api.h")) -and
        (
            (Test-Path -LiteralPath (Join-Path $resolved "lib\onnxruntime.lib")) -or
            (Test-Path -LiteralPath (Join-Path $resolved "runtimes\win-x64\native\onnxruntime.lib"))
        )
    )
}

function Resolve-OnnxRuntimeRoot {
    param([string] $PluginRepoRoot)

    $explicitRoot = [Environment]::GetEnvironmentVariable("MOQI_ONNXRUNTIME_ROOT")
    if (-not $explicitRoot) {
        $explicitRoot = [Environment]::GetEnvironmentVariable("ONNXRUNTIME_ROOT_DIR")
    }

    $candidates = @(
        $explicitRoot,
        (Join-Path $PluginRepoRoot ".deps\onnxruntime\win-x64"),
        (Join-Path $PluginRepoRoot ".deps\onnxruntime\current")
    )

    $depsRoot = Join-Path $PluginRepoRoot ".deps\onnxruntime"
    if (Test-Path -LiteralPath $depsRoot) {
        $versionDirs = Get-ChildItem -LiteralPath $depsRoot -Directory -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending
        foreach ($dir in $versionDirs) {
            $candidates += $dir.FullName
            $candidates += (Join-Path $dir.FullName "pkg")
        }
    }

    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        if (Test-OnnxRuntimeSdkRoot $candidate) {
            return [System.IO.Path]::GetFullPath($candidate)
        }
    }

    return ""
}

function Resolve-OnnxRuntimeRuntimeDlls {
    param([string] $OnnxRuntimeRoot)

    if ([string]::IsNullOrWhiteSpace($OnnxRuntimeRoot)) {
        return @()
    }

    $resolvedRoot = [System.IO.Path]::GetFullPath($OnnxRuntimeRoot)
    $candidates = @(
        (Join-Path $resolvedRoot "bin\onnxruntime.dll"),
        (Join-Path $resolvedRoot "lib\onnxruntime.dll"),
        (Join-Path $resolvedRoot "runtimes\win-x64\native\onnxruntime.dll"),
        (Join-Path $resolvedRoot "bin\onnxruntime_providers_shared.dll"),
        (Join-Path $resolvedRoot "lib\onnxruntime_providers_shared.dll"),
        (Join-Path $resolvedRoot "runtimes\win-x64\native\onnxruntime_providers_shared.dll")
    )

    $resolved = New-Object System.Collections.Generic.List[string]
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) {
            $fullPath = [System.IO.Path]::GetFullPath($candidate)
            if (-not $resolved.Contains($fullPath)) {
                $resolved.Add($fullPath)
            }
        }
    }
    return $resolved.ToArray()
}

function Prepare-RimeData {
    param(
        [string] $RimeDataDir,
        [string] $PackageRimeDataDir
    )

    Remove-IfExists -Path $PackageRimeDataDir
    Ensure-Directory -Path $PackageRimeDataDir

    Write-Host "[INFO] Copying shared data from rime-frost submodule ..."
    Copy-DirectoryContents -Source $RimeDataDir -Destination $PackageRimeDataDir

    Remove-IfExists -Path (Join-Path $PackageRimeDataDir ".github")

    foreach ($name in @("README.md", "LICENSE")) {
        $path = Join-Path $PackageRimeDataDir $name
        if (Test-Path -LiteralPath $path) {
            Remove-Item -LiteralPath $path -Force
        }
    }

    Write-Host "[INFO] Packaged Rime shared data prepared at `"$PackageRimeDataDir`""
}

function Write-ServerVersionInfo {
    param(
        [string] $VersionInfoPath,
        [string] $IconPath
    )

    $versionInfo = [ordered]@{
        FixedFileInfo  = [ordered]@{
            FileVersion    = [ordered]@{
                Major = 1
                Minor = 0
                Patch = 0
                Build = 0
            }
            ProductVersion = [ordered]@{
                Major = 1
                Minor = 0
                Patch = 0
                Build = 0
            }
            FileFlagsMask  = "3f"
            FileFlags      = "00"
            FileOS         = "040004"
            FileType       = "01"
            FileSubType    = "00"
        }
        StringFileInfo = [ordered]@{
            Comments         = ""
            CompanyName      = ""
            FileDescription  = "墨奇输入法引擎服务"
            FileVersion      = "1.0.0.0"
            InternalName     = "server.exe"
            LegalCopyright   = ""
            LegalTrademarks  = ""
            OriginalFilename = "server.exe"
            PrivateBuild     = ""
            ProductName      = "墨奇输入法"
            ProductVersion   = "1.0.0.0"
            SpecialBuild     = ""
        }
        VarFileInfo    = [ordered]@{
            Translation = [ordered]@{
                LangID    = "0804"
                CharsetID = "04B0"
            }
        }
        IconPath       = $IconPath
        ManifestPath   = ""
    }

    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText(
        $VersionInfoPath,
        ($versionInfo | ConvertTo-Json -Depth 6),
        $utf8NoBom
    )
}

$scriptRepoRoot = Join-Path $PSScriptRoot ".."
if (-not $RepoRoot) { $RepoRoot = $scriptRepoRoot }
$RepoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
$WorkspaceRoot = Split-Path $RepoRoot -Parent
$LibrimeRoot = Join-Path $WorkspaceRoot "librime"
$BertGrammarRepoRoot = Join-Path $WorkspaceRoot "librime-bert-gram"

if (-not $BuildRoot) { $BuildRoot = Join-Path $PSScriptRoot "build" }
if (-not $PackageDir) { $PackageDir = Join-Path $BuildRoot "moqi-ime" }

$BuildRoot = [System.IO.Path]::GetFullPath($BuildRoot)
$PackageDir = [System.IO.Path]::GetFullPath($PackageDir)
$ServerExe = Join-Path $PackageDir "server.exe"
$BackendSnippet = Join-Path $BuildRoot "backends.moqi-ime.json"
$InputMethodsDir = Join-Path $RepoRoot "input_methods"
$IconsDir = Join-Path $RepoRoot "icons"
$RimeDir = Join-Path $InputMethodsDir "rime"
$RimeDataDir = Join-Path $RepoRoot "rime-frost"
$PackageRimeDir = Join-Path $PackageDir "input_methods\rime"
$PackageRimeDataDir = Join-Path $PackageRimeDir "data"
$PackageRimePluginsDir = Join-Path $PackageRimeDir "rime-plugins"
$BertSourceDir = Join-Path $RimeDir "bert"
$BertRuntimeDLL = Join-Path $BertSourceDir "onnxruntime.dll"
$BertGrammarSourceDir = Join-Path $BertGrammarRepoRoot "bert_grammar"
$ResolvedRimeDll = Resolve-RimeDllPath -RequestedPath $RimeDllPath -RepoRoot $RepoRoot -BuildRoot $BuildRoot
$BertGrammarPluginDll = Resolve-BertGrammarPluginDll -PluginRepoRoot $BertGrammarRepoRoot -LibrimeRoot $LibrimeRoot -BuildRoot $BuildRoot
$OnnxRuntimeRoot = Resolve-OnnxRuntimeRoot -PluginRepoRoot $BertGrammarRepoRoot
$OnnxRuntimeRuntimeDlls = Resolve-OnnxRuntimeRuntimeDlls -OnnxRuntimeRoot $OnnxRuntimeRoot
$ServerIcon = Join-Path $IconsDir "mo.ico"
$ServerVersionInfo = Join-Path $BuildRoot "server.versioninfo.json"
$ServerResource = Join-Path $RepoRoot "resource_windows_amd64.syso"

Write-Host "============================================"
Write-Host " Moqi IME Go Backend Build Script"
Write-Host "============================================"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go was not found in PATH. Install Go from https://golang.org/dl/"
}

$goVersion = & go version
if ($LASTEXITCODE -ne 0) {
    throw "Failed to query Go version."
}
Write-Host "[INFO] Go version: $goVersion"

Write-Step -Title "Step 1: Prepare output directory"
if (Test-Path -LiteralPath $PackageDir) {
    Write-Host "[INFO] Removing old build output: `"$PackageDir`""
    Remove-Item -LiteralPath $PackageDir -Recurse -Force
}
Ensure-Directory -Path $PackageDir
Write-Host "[INFO] Output directory: `"$PackageDir`""

if (-not (Test-Path -LiteralPath (Join-Path $RimeDataDir "default.yaml"))) {
    throw "Missing rime-frost shared data submodule: `"$RimeDataDir`"`nRun: git submodule update --init --recursive rime-frost"
}
if (-not (Test-Path -LiteralPath $BertGrammarRepoRoot)) {
    throw "Missing librime-bert-gram repository: `"$BertGrammarRepoRoot`""
}
if (-not $BertGrammarPluginDll) {
    throw "Missing rime-bert-grammar.dll. Build the standalone plugin first, or set MOQI_BERT_GRAMMAR_DLL to the built DLL path."
}
if (-not $ResolvedRimeDll) {
    throw "Missing rime.dll. Pass -RimeDllPath, set MOQI_RIME_DLL, or install/copy the official release rime.dll first."
}
if (-not $OnnxRuntimeRoot) {
    throw "Missing ONNX Runtime SDK. Set MOQI_ONNXRUNTIME_ROOT / ONNXRUNTIME_ROOT_DIR, or prepare librime-bert-gram\.deps\onnxruntime."
}
if (-not $OnnxRuntimeRuntimeDlls -or $OnnxRuntimeRuntimeDlls.Count -eq 0) {
    throw "Missing ONNX Runtime runtime DLLs under `"$OnnxRuntimeRoot`""
}

Push-Location $RepoRoot
try {
    Write-Step -Title "Step 2: Sync Go dependencies"
    $tidyExitCode = Invoke-External -FilePath "go" -ArgumentList @("mod", "tidy") -IgnoreExitCode
    if ($tidyExitCode -ne 0) {
        Write-Warning "go mod tidy failed, continuing..."
    }

    Write-Step -Title "Step 3: Build go-backend server"
    Write-Host "[INFO] Building server.exe with dynamic DLL loading ..."

    $oldGoos = $env:GOOS
    $oldGoarch = $env:GOARCH
    $oldCgoEnabled = $env:CGO_ENABLED
    $oldCC = $env:CC
    $goversioninfo = Get-GoTool -ToolName "goversioninfo" -ModuleAtVersion "github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $availableCompiler = Get-AvailableCCompiler -RequestedCompiler $CCompiler
    $useCgo = $false
    $shouldBuildBertRuntime = (Test-Path -LiteralPath $BertRuntimeDLL)
    if ($shouldBuildBertRuntime) {
        if ($availableCompiler) {
            Write-Host ("[INFO] BERT runtime detected; using C compiler: {0}" -f $availableCompiler)
            $useCgo = $true
            $env:CC = $availableCompiler
        }
        else {
            Write-Warning "BERT runtime assets are present but no C compiler was found. Pass -CCompiler or set CC/MOQI_C_COMPILER to build the real reranker; otherwise the stub reranker will be used."
        }
    }
    $env:CGO_ENABLED = if ($useCgo) { "1" } else { "0" }

    try {
        if (-not (Test-Path -LiteralPath $ServerIcon)) {
            throw "Missing server icon: `"$ServerIcon`""
        }

        Write-ServerVersionInfo -VersionInfoPath $ServerVersionInfo -IconPath $ServerIcon
        Remove-IfExists -Path $ServerResource
        $null = Invoke-External -FilePath $goversioninfo -ArgumentList @("-64", "-o", $ServerResource, $ServerVersionInfo)
        $null = Invoke-External -FilePath "go" -ArgumentList @("build", "-ldflags", "-s -w", "-o", $ServerExe, ".")
    }
    finally {
        Remove-IfExists -Path $ServerResource
        Remove-IfExists -Path $ServerVersionInfo
        $env:GOOS = $oldGoos
        $env:GOARCH = $oldGoarch
        $env:CGO_ENABLED = $oldCgoEnabled
        $env:CC = $oldCC
    }

    Write-Host "[INFO] Built: `"$ServerExe`""

    Write-Step -Title "Step 4: Copy packaged input_methods"
    if (-not (Test-Path -LiteralPath $RimeDir)) {
        throw "Missing Rime input method directory: `"$RimeDir`""
    }

    $packageInputMethodsDir = Join-Path $PackageDir "input_methods"
    Ensure-Directory -Path $packageInputMethodsDir
    Copy-DirectoryContents -Source $RimeDir -Destination (Join-Path $packageInputMethodsDir "rime")
    Write-Host "[INFO] Packaged only input_methods\rime"

    Write-Step -Title "Step 5: Copy shared icons"
    if (Test-Path -LiteralPath $IconsDir) {
        Copy-DirectoryContents -Source $IconsDir -Destination (Join-Path $PackageDir "icons")
        Write-Host "[INFO] icons copied"
    }
    else {
        Write-Warning "Missing icons directory: `"$IconsDir`""
    }

    Write-Step -Title "Step 6: Prepare packaged Rime shared data"
    Prepare-RimeData -RimeDataDir $RimeDataDir -PackageRimeDataDir $PackageRimeDataDir

    if (-not (Test-Path -LiteralPath $BertGrammarSourceDir)) {
        throw "BERT grammar asset directory is missing: `"$BertGrammarSourceDir`""
    }
    $packageBertDir = Join-Path $PackageRimeDir "bert"
    Write-Host ("[INFO] BERT runtime assets staged in `"{0}`"; model path and enablement come from user Rime config" -f $packageBertDir)
    foreach ($assetName in @("model.onnx", "model.onnx.data", "vocab.txt", "tokenizer.json", "tokenizer_config.json", "special_tokens_map.json")) {
        $packagedAsset = Join-Path $PackageRimeDir ("bert\" + $assetName)
        if (Test-Path -LiteralPath $packagedAsset) {
            Remove-Item -LiteralPath $packagedAsset -Force
            Write-Host ("[INFO] Removed packaged optional BERT asset `"{0}`"" -f $packagedAsset)
        }
    }

    $pathsToRemove = @(
        @{ Path = Join-Path $PackageDir "input_methods\rime\data\others"; Label = "rime shared data others directory" },
        @{ Path = Join-Path $PackageDir "input_methods\rime\icons\icons"; Label = "nested icons directory" }
    )
    foreach ($entry in $pathsToRemove) {
        if (Test-Path -LiteralPath $entry.Path) {
            Remove-Item -LiteralPath $entry.Path -Recurse -Force
            Write-Host "[INFO] Removed packaged $($entry.Label)"
        }
    }

    $packagedGoFiles = Get-ChildItem -LiteralPath (Join-Path $PackageDir "input_methods\rime") -Filter "*.go" -File -ErrorAction SilentlyContinue
    if ($packagedGoFiles) {
        $packagedGoFiles | Remove-Item -Force
        Write-Host "[INFO] Removed packaged Go source files"
    }

    Copy-Item -LiteralPath $ResolvedRimeDll -Destination (Join-Path $PackageDir "input_methods\rime\rime.dll") -Force
    Write-Host "[INFO] Copied rime.dll into package output from `"$ResolvedRimeDll`""
    Ensure-Directory -Path $PackageRimePluginsDir
    Copy-Item -LiteralPath $BertGrammarPluginDll -Destination (Join-Path $PackageRimePluginsDir "rime-bert-grammar.dll") -Force
    foreach ($runtimeDll in $OnnxRuntimeRuntimeDlls) {
        Copy-Item -LiteralPath $runtimeDll -Destination (Join-Path $PackageRimePluginsDir ([System.IO.Path]::GetFileName($runtimeDll))) -Force
    }
    Write-Host "[INFO] Copied BERT grammar plugin DLL and ONNX Runtime runtime DLLs into rime-plugins"

    $packageBertGrammarDataDir = Join-Path $PackageRimeDataDir "bert_grammar"
    Copy-DirectoryContents -Source $BertGrammarSourceDir -Destination $packageBertGrammarDataDir
    Write-Host "[INFO] Copied BERT grammar shared data assets (users must reference them from their own Rime config if desired)"

    Write-Step -Title "Step 7: Generate backends.json snippet"
    @(
        [ordered]@{
            name       = "moqi-ime"
            command    = "moqi-ime\server.exe"
            workingDir = "moqi-ime"
            params     = ""
        }
    ) | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $BackendSnippet -Encoding UTF8

    Write-Host "[INFO] Generated: `"$BackendSnippet`""
}
finally {
    Pop-Location
}

Write-Step -Title "Build completed"
Write-Host "Output directory:"
Write-Host "  `"$PackageDir`""
Write-Host ""
Write-Host "Install target:"
Write-Host "  C:\Program Files (x86)\MoqiIM\moqi-ime"
Write-Host ""
Write-Host "Notes:"
Write-Host "1. backends.json in this repo uses a top-level array."
Write-Host "2. Ensure C:\Program Files (x86)\MoqiIM\backends.json includes moqi-ime."
Write-Host "3. Ensure C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\*\ime.json exists."
Write-Host "4. Re-register both MoqiTextService.dll files after copying."
Write-Host "5. Ensure C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\rime contains rime.dll."
Write-Host "6. Ensure C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\rime\rime-plugins contains rime-bert-grammar.dll, onnxruntime.dll, and any companion ONNX Runtime DLLs."
Write-Host "7. If you want to use BERT grammar, configure model and vocab paths in your own Rime config; packaged shared assets are under C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\rime\data\bert_grammar."
Write-Host "8. Start or restart MoqiLauncher.exe after install."
