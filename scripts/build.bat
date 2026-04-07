@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
if "%SCRIPT_DIR:~-1%"=="\" set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"
set "POWERSHELL_EXE=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"
set "BUILD_PS1=%SCRIPT_DIR%\build.ps1"

if not exist "%POWERSHELL_EXE%" (
    echo [ERROR] PowerShell was not found: "%POWERSHELL_EXE%"
    exit /b 1
)

if not exist "%BUILD_PS1%" (
    echo [ERROR] build.ps1 was not found: "%BUILD_PS1%"
    exit /b 1
)

"%POWERSHELL_EXE%" -NoProfile -ExecutionPolicy Bypass -File "%BUILD_PS1%" %*
exit /b %ERRORLEVEL%
