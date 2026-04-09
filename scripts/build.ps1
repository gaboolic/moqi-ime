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
#>
param(
    [string] $RepoRoot = "",
    [string] $BuildRoot = "",
    [string] $PackageDir = ""
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
    $goversioninfo = Get-GoTool -ToolName "goversioninfo" -ModuleAtVersion "github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"

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

    $rimeDll = Join-Path $RimeDir "rime.dll"
    if (Test-Path -LiteralPath $rimeDll) {
        Copy-Item -LiteralPath $rimeDll -Destination (Join-Path $PackageDir "input_methods\rime\rime.dll") -Force
        Write-Host "[INFO] Copied rime.dll into package output"
    }

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
Write-Host "6. Start or restart MoqiLauncher.exe after install."
