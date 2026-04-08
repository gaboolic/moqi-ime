param(
    [string]$SourceRoot = (Join-Path $PSScriptRoot "build\moqi-ime"),
    [string]$InstallRoot = "C:\Program Files (x86)\MoqiIM\moqi-ime",
    [string]$LauncherPath = "C:\Program Files (x86)\MoqiIM\MoqLauncher.exe"
)

$ErrorActionPreference = "Stop"

function Test-Admin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Restart-Elevated {
    $argumentList = @(
        "-NoProfile"
        "-ExecutionPolicy"
        "Bypass"
        "-File"
        ('"{0}"' -f $PSCommandPath)
        "-SourceRoot"
        ('"{0}"' -f $SourceRoot)
        "-InstallRoot"
        ('"{0}"' -f $InstallRoot)
        "-LauncherPath"
        ('"{0}"' -f $LauncherPath)
    )

    Write-Host "[INFO] Requesting administrator privileges ..."
    try {
        Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList $argumentList | Out-Null
        Write-Host "[INFO] Elevated PowerShell launched. Continuing in the administrator window."
        exit 0
    }
    catch {
        throw "Administrator elevation was cancelled or failed."
    }
}

function Stop-MoqLauncher {
    param([string]$Path)

    $running = @(Get-Process -Name "MoqLauncher" -ErrorAction SilentlyContinue)
    if (-not $running) {
        Write-Host "[INFO] MoqLauncher.exe is not running."
        return
    }

    Write-Host "[INFO] Stopping MoqLauncher.exe ..."
    if (Test-Path -LiteralPath $Path) {
        try {
            Start-Process -FilePath $Path -ArgumentList "/quit" -WindowStyle Hidden | Out-Null
        }
        catch {
            Write-Host "[WARN] Graceful quit failed, forcing stop."
        }
    }

    $deadline = (Get-Date).AddSeconds(5)
    do {
        Start-Sleep -Milliseconds 250
        $running = @(Get-Process -Name "MoqLauncher" -ErrorAction SilentlyContinue)
    } while ($running.Count -gt 0 -and (Get-Date) -lt $deadline)

    if ($running.Count -gt 0) {
        Write-Host "[WARN] Timed out waiting for graceful shutdown, forcing stop."
        $running | Stop-Process -Force
    }

    Start-Sleep -Seconds 1
    Write-Host "[INFO] MoqLauncher.exe stopped."
}

function Start-MoqLauncher {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "MoqLauncher.exe not found: $Path"
    }

    Write-Host "[INFO] Starting MoqLauncher.exe ..."
    Start-Process -FilePath $Path | Out-Null
    Write-Host "[INFO] MoqLauncher.exe started."
}

function Write-FileDetails {
    param(
        [string]$Label,
        [string]$Path
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Host ("[INFO] {0}: not found ({1})" -f $Label, $Path)
        return
    }

    $item = Get-Item -LiteralPath $Path
    $version = $item.VersionInfo.FileVersion
    if (-not $version) {
        $version = "<none>"
    }

    Write-Host ("[INFO] {0}: LastWriteTime={1:yyyy-MM-dd HH:mm:ss}, FileVersion={2}" -f $Label, $item.LastWriteTime, $version)
}

function Sync-GoBackendRuntime {
    param(
        [string]$Source,
        [string]$Destination
    )

    Write-Host "[INFO] Syncing moqi-ime runtime directory ..."

    if (-not (Test-Path -LiteralPath $Destination)) {
        New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    }

    $sourceServer = Join-Path $Source "server.exe"
    $destinationServer = Join-Path $Destination "server.exe"
    Copy-Item -LiteralPath $sourceServer -Destination $destinationServer -Force

    $sourceInputMethods = Join-Path $Source "input_methods"
    $destinationInputMethods = Join-Path $Destination "input_methods"
    if (Test-Path -LiteralPath $destinationInputMethods) {
        Remove-Item -LiteralPath $destinationInputMethods -Recurse -Force
    }
    Copy-Item -LiteralPath $sourceInputMethods -Destination $destinationInputMethods -Recurse -Force

    $sourceIcons = Join-Path $Source "icons"
    $destinationIcons = Join-Path $Destination "icons"
    if (Test-Path -LiteralPath $destinationIcons) {
        Remove-Item -LiteralPath $destinationIcons -Recurse -Force
    }
    if (Test-Path -LiteralPath $sourceIcons) {
        Copy-Item -LiteralPath $sourceIcons -Destination $destinationIcons -Recurse -Force
    }
    else {
        Write-Host "[WARN] Source icons directory not found, skipping icons sync."
    }

    Write-Host "[INFO] moqi-ime runtime directory synced."
}

function Sync-RimeUserAIConfig {
    param(
        [string]$SourceConfig,
        [string]$AppDataPath
    )

    if (-not $AppDataPath) {
        Write-Host "[WARN] APPDATA is empty, skipping user ai_config.json sync."
        return
    }

    $destinationDir = Join-Path $AppDataPath "Moqi\Rime"
    $destinationConfig = Join-Path $destinationDir "ai_config.json"

    Write-Host "[INFO] Syncing user ai_config.json ..."

    if (-not (Test-Path -LiteralPath $destinationDir)) {
        New-Item -ItemType Directory -Path $destinationDir -Force | Out-Null
    }

    Copy-Item -LiteralPath $SourceConfig -Destination $destinationConfig -Force
    Write-Host ("[INFO] User ai_config.json synced to {0}" -f $destinationConfig)
}

if (-not (Test-Admin)) {
    Restart-Elevated
}

if (-not (Test-Path -LiteralPath $SourceRoot)) {
    throw "Source moqi-ime directory not found: $SourceRoot"
}

if (-not (Test-Path -LiteralPath $InstallRoot)) {
    throw "Install directory not found: $InstallRoot"
}

$sourceServer = Join-Path $SourceRoot "server.exe"
$destinationServer = Join-Path $InstallRoot "server.exe"
$sourceRimeDLL = Join-Path $SourceRoot "input_methods\rime\rime.dll"
$destinationRimeDLL = Join-Path $InstallRoot "input_methods\rime\rime.dll"

$sourceDataDir = Join-Path $SourceRoot "input_methods\rime\data"
$sourceIconsDir = Join-Path $SourceRoot "input_methods\rime\icons"
$sourceIMEJSON = Join-Path $SourceRoot "input_methods\rime\ime.json"
$sourceAIConfig = Join-Path $SourceRoot "input_methods\rime\ai_config.json"

$sourcePaths = @($sourceServer, $sourceRimeDLL, $sourceDataDir, $sourceIconsDir, $sourceIMEJSON, $sourceAIConfig)
foreach ($path in $sourcePaths) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Required source path not found: $path"
    }
}

Write-FileDetails -Label "Source server.exe" -Path $sourceServer
Write-FileDetails -Label "Destination server.exe (before)" -Path $destinationServer
Write-FileDetails -Label "Source rime.dll" -Path $sourceRimeDLL
Write-FileDetails -Label "Destination rime.dll (before)" -Path $destinationRimeDLL

try {
    Stop-MoqLauncher -Path $LauncherPath

    Sync-GoBackendRuntime -Source $SourceRoot -Destination $InstallRoot
    Sync-RimeUserAIConfig -SourceConfig $sourceAIConfig -AppDataPath $env:APPDATA

    Write-FileDetails -Label "Destination server.exe (after)" -Path $destinationServer
    Write-FileDetails -Label "Destination rime.dll (after)" -Path $destinationRimeDLL
}
finally {
    Start-MoqLauncher -Path $LauncherPath
}
 