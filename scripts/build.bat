@echo off
setlocal

echo ============================================
echo  Moqi IME Go Backend Build Script
echo ============================================
echo.

set "ROOT_DIR=%~dp0"
if "%ROOT_DIR:~-1%"=="\" set "ROOT_DIR=%ROOT_DIR:~0,-1%"
for %%I in ("%ROOT_DIR%\..") do set "MOQI_IME_ROOT=%%~fI"
set "BUILD_ROOT=%ROOT_DIR%\build"
set "PACKAGE_DIR=%BUILD_ROOT%\moqi-ime"
set "SERVER_EXE=%PACKAGE_DIR%\server.exe"
set "BACKEND_SNIPPET=%BUILD_ROOT%\backends.moqi-ime.json"
set "INPUT_METHODS_DIR=%MOQI_IME_ROOT%\input_methods"
set "RIME_DIR=%INPUT_METHODS_DIR%\rime"
set "RIME_DATA_DIR=%MOQI_IME_ROOT%\rime-frost"
set "PACKAGE_RIME_DIR=%PACKAGE_DIR%\input_methods\rime"
set "PACKAGE_RIME_DATA_DIR=%PACKAGE_RIME_DIR%\data"

REM Check Go environment
where go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Go was not found in PATH.
    echo Install Go from: https://golang.org/dl/
    exit /b 1
)

for /f "tokens=3" %%i in ('go version') do (
    echo [INFO] Go version: %%i
)

echo.
echo ============================================
echo Step 1: Prepare output directory
echo ============================================
echo.

if exist "%PACKAGE_DIR%" (
    echo [INFO] Removing old build output: "%PACKAGE_DIR%"
    rmdir /s /q "%PACKAGE_DIR%"
)

mkdir "%PACKAGE_DIR%"
if errorlevel 1 (
    echo [ERROR] Failed to create output directory: "%PACKAGE_DIR%"
    exit /b 1
)

echo [INFO] Output directory: "%PACKAGE_DIR%"

if not exist "%RIME_DATA_DIR%\default.yaml" (
    echo [ERROR] Missing rime-frost shared data submodule: "%RIME_DATA_DIR%"
    echo [ERROR] Run: git submodule update --init --recursive rime-frost
    exit /b 1
)

echo.
echo ============================================
echo Step 2: Sync Go dependencies
echo ============================================
echo.

pushd "%MOQI_IME_ROOT%"
go mod tidy
if errorlevel 1 (
    echo [WARN] go mod tidy failed, continuing...
)

echo.
echo ============================================
echo Step 3: Build go-backend server
echo ============================================
echo.

set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"

echo [INFO] Building server.exe with dynamic DLL loading ...
go build -ldflags "-s -w" -o "%SERVER_EXE%" .
if errorlevel 1 (
    echo [ERROR] Failed to build server.exe
    popd
    exit /b 1
)

echo [INFO] Built: "%SERVER_EXE%"

echo.
echo ============================================
echo Step 4: Copy input_methods
echo ============================================
echo.

if not exist "%INPUT_METHODS_DIR%" (
    echo [ERROR] Missing input_methods directory: "%INPUT_METHODS_DIR%"
    popd
    exit /b 1
)

xcopy "%INPUT_METHODS_DIR%" "%PACKAGE_DIR%\input_methods\" /E /I /Y >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy input_methods
    popd
    exit /b 1
)

echo [INFO] input_methods copied

echo.
echo ============================================
echo Step 5: Prepare packaged Rime shared data
echo ============================================
echo.

call :prepare_rime_data
if errorlevel 1 (
    echo [ERROR] Failed to prepare packaged Rime shared data
    popd
    exit /b 1
)

if exist "%PACKAGE_DIR%\input_methods\rime\brise" (
    rmdir /s /q "%PACKAGE_DIR%\input_methods\rime\brise"
    if errorlevel 1 (
        echo [ERROR] Failed to remove packaged rime\brise directory
        popd
        exit /b 1
    )
    echo [INFO] Removed rime\brise from package output
)

if exist "%PACKAGE_DIR%\input_methods\rime\*.go" (
    del /q "%PACKAGE_DIR%\input_methods\rime\*.go" >nul
    if errorlevel 1 (
        echo [ERROR] Failed to remove packaged Go source files
        popd
        exit /b 1
    )
    echo [INFO] Removed packaged Go source files
)

if exist "%PACKAGE_DIR%\input_methods\rime\rime.dll.bak-32bit" (
    del /q "%PACKAGE_DIR%\input_methods\rime\rime.dll.bak-32bit" >nul
    if errorlevel 1 (
        echo [ERROR] Failed to remove packaged backup DLL
        popd
        exit /b 1
    )
    echo [INFO] Removed packaged backup DLL
)

if exist "%PACKAGE_DIR%\input_methods\rime\icons\icons" (
    rmdir /s /q "%PACKAGE_DIR%\input_methods\rime\icons\icons"
    if errorlevel 1 (
        echo [ERROR] Failed to remove nested icons directory
        popd
        exit /b 1
    )
    echo [INFO] Removed nested icons directory
)

if exist "%RIME_DIR%\rime.dll" (
    copy /Y "%RIME_DIR%\rime.dll" "%PACKAGE_DIR%\input_methods\rime\rime.dll" >nul
    echo [INFO] Copied rime.dll into package output
)

echo.
echo ============================================
echo Step 6: Generate backends.json snippet
echo ============================================
echo.

> "%BACKEND_SNIPPET%" echo [
>> "%BACKEND_SNIPPET%" echo   {
>> "%BACKEND_SNIPPET%" echo     "name": "moqi-ime",
>> "%BACKEND_SNIPPET%" echo     "command": "moqi-ime\\server.exe",
>> "%BACKEND_SNIPPET%" echo     "workingDir": "moqi-ime",
>> "%BACKEND_SNIPPET%" echo     "params": ""
>> "%BACKEND_SNIPPET%" echo   }
>> "%BACKEND_SNIPPET%" echo ]

echo [INFO] Generated: "%BACKEND_SNIPPET%"
popd

echo.
echo ============================================
echo Build completed
echo ============================================
echo.
echo Output directory:
echo   "%PACKAGE_DIR%"
echo.
echo Install target:
echo   C:\Program Files (x86)\MoqiIM\moqi-ime
echo.
echo Notes:
echo 1. backends.json in this repo uses a top-level array.
echo 2. Ensure C:\Program Files (x86)\MoqiIM\backends.json includes moqi-ime.
echo 3. Ensure C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\*\ime.json exists.
echo 4. Re-register both MoqiTextService.dll files after copying.
echo 5. Ensure C:\Program Files (x86)\MoqiIM\moqi-ime\input_methods\rime contains rime.dll.
echo 6. Start or restart MoqLauncher.exe after install.
echo.
exit /b 0

:prepare_rime_data
if exist "%PACKAGE_RIME_DATA_DIR%" (
    rmdir /s /q "%PACKAGE_RIME_DATA_DIR%"
)
mkdir "%PACKAGE_RIME_DATA_DIR%"
if errorlevel 1 (
    echo [ERROR] Failed to create packaged Rime data directory: "%PACKAGE_RIME_DATA_DIR%"
    exit /b 1
)

echo [INFO] Copying shared data from rime-frost submodule ...
xcopy "%RIME_DATA_DIR%\*" "%PACKAGE_RIME_DATA_DIR%\" /E /I /Y >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy bundled Rime data from "%RIME_DATA_DIR%"
    exit /b 1
)

if exist "%PACKAGE_RIME_DATA_DIR%\.github" (
    rmdir /s /q "%PACKAGE_RIME_DATA_DIR%\.github"
)
if exist "%PACKAGE_RIME_DATA_DIR%\README.md" (
    del /q "%PACKAGE_RIME_DATA_DIR%\README.md" >nul
)
if exist "%PACKAGE_RIME_DATA_DIR%\LICENSE" (
    del /q "%PACKAGE_RIME_DATA_DIR%\LICENSE" >nul
)

echo [INFO] Packaged Rime shared data prepared at "%PACKAGE_RIME_DATA_DIR%"
exit /b 0
