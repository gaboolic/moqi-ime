param(
    [string]$LibrimeRoot = (Resolve-Path "$PSScriptRoot\..\..\librime").Path,
    [string]$Abi = "arm64-v8a",
    [string]$AndroidPlatform = "android-23",
    [string]$AndroidNdk = (Join-Path $env:ANDROID_HOME "ndk\27.1.12297006"),
    [string]$BoostRoot = (Join-Path (Resolve-Path "$PSScriptRoot\..\..\librime").Path ".deps\boost_1_84_0")
)

$ErrorActionPreference = "Stop"

function Invoke-Checked {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory
    )

    Write-Host ">> $FilePath $($Arguments -join ' ')"
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE"
    }
}

if (-not (Test-Path $LibrimeRoot)) {
    throw "librime root not found: $LibrimeRoot"
}
if (-not (Test-Path $AndroidNdk)) {
    throw "Android NDK not found: $AndroidNdk"
}
if (-not (Test-Path (Join-Path $BoostRoot "boost\regex.hpp"))) {
    $boostParent = Split-Path $BoostRoot
    $boostZip = Join-Path $boostParent "boost_1_84_0.zip"
    New-Item -ItemType Directory -Force -Path $boostParent | Out-Null
    if (-not (Test-Path $boostZip)) {
        Invoke-WebRequest -Uri "https://archives.boost.io/release/1.84.0/source/boost_1_84_0.zip" -OutFile $boostZip
    }
    & tar -xf $boostZip -C $boostParent
    if (-not (Test-Path (Join-Path $BoostRoot "boost\regex.hpp"))) {
        throw "Boost headers not found: $BoostRoot"
    }
}

$cmake = "cmake"
$toolchain = Join-Path $AndroidNdk "build\cmake\android.toolchain.cmake"
$make = Join-Path $AndroidNdk "prebuilt\windows-x86_64\bin\make.exe"
$llvmBin = Join-Path $AndroidNdk "toolchains\llvm\prebuilt\windows-x86_64\bin"
$buildRoot = Join-Path $LibrimeRoot "build-android\$Abi"
$prefix = Join-Path $LibrimeRoot "dist-android\$Abi"
$depsPrefix = Join-Path $buildRoot "deps-prefix"
$outDir = Join-Path (Resolve-Path "$PSScriptRoot\..").Path "input_methods\rime\android\$Abi"

New-Item -ItemType Directory -Force -Path $buildRoot, $prefix, $depsPrefix, $outDir | Out-Null

$commonCMakeArgs = @(
    "-G", "Unix Makefiles",
    "-DCMAKE_MAKE_PROGRAM=$make",
    "-DCMAKE_TOOLCHAIN_FILE=$toolchain",
    "-DANDROID_ABI=$Abi",
    "-DANDROID_PLATFORM=$AndroidPlatform",
    "-DANDROID_STL=c++_shared",
    "-DCMAKE_BUILD_TYPE=Release",
    "-DCMAKE_POSITION_INDEPENDENT_CODE=ON",
    "-DCMAKE_INSTALL_PREFIX=$depsPrefix",
    "-DBUILD_SHARED_LIBS=OFF"
)

function Invoke-CMakeProjectBuild {
    param(
        [string]$Name,
        [string]$SourceDir,
        [string[]]$ExtraArgs,
        [string]$Target = "install"
    )

    $buildDir = Join-Path $buildRoot $Name
    New-Item -ItemType Directory -Force -Path $buildDir | Out-Null

    Invoke-Checked $cmake (@("-S", $SourceDir, "-B", $buildDir) + $commonCMakeArgs + $ExtraArgs) $LibrimeRoot
    Invoke-Checked $cmake @("--build", $buildDir, "--target", $Target, "--", "-j$env:NUMBER_OF_PROCESSORS") $LibrimeRoot
}

function Ensure-LibrimeLuaPlugin {
    param(
        [string]$LibrimeRoot
    )

    $pluginDir = Join-Path $LibrimeRoot "plugins\lua"
    if (-not (Test-Path (Join-Path $pluginDir "CMakeLists.txt"))) {
        Write-Host "Installing librime-lua plugin..."
        Remove-Item -Recurse -Force $pluginDir -ErrorAction SilentlyContinue
        Invoke-Checked "git" @(
            "clone",
            "--depth=1",
            "https://github.com/hchunhui/librime-lua.git",
            $pluginDir
        ) $LibrimeRoot
    }

    $luaHeader = Join-Path $pluginDir "thirdparty\lua5.4\lua.h"
    if (-not (Test-Path $luaHeader)) {
        Write-Host "Installing librime-lua third-party Lua runtime..."
        Remove-Item -Recurse -Force (Join-Path $pluginDir "thirdparty") -ErrorAction SilentlyContinue
        Invoke-Checked "git" @(
            "clone",
            "--depth=1",
            "-b",
            "thirdparty",
            "https://github.com/hchunhui/librime-lua.git",
            "thirdparty"
        ) $pluginDir
    }
}

Invoke-CMakeProjectBuild "leveldb" (Join-Path $LibrimeRoot "deps\leveldb") @(
    "-DLEVELDB_BUILD_TESTS=OFF",
    "-DLEVELDB_BUILD_BENCHMARKS=OFF",
    "-DLEVELDB_INSTALL=ON"
)

Invoke-CMakeProjectBuild "yaml-cpp" (Join-Path $LibrimeRoot "deps\yaml-cpp") @(
    "-DYAML_CPP_BUILD_CONTRIB=OFF",
    "-DYAML_CPP_BUILD_TESTS=OFF",
    "-DYAML_CPP_BUILD_TOOLS=OFF",
    "-DYAML_CPP_INSTALL=ON",
    "-DYAML_BUILD_SHARED_LIBS=OFF"
)

Invoke-CMakeProjectBuild "marisa-trie" (Join-Path $LibrimeRoot "deps\marisa-trie") @(
    "-DBUILD_TESTING=OFF",
    "-DENABLE_TOOLS=OFF",
    "-DENABLE_NATIVE_CODE=OFF"
)

# OpenCC's install target generates dictionaries by running opencc_dict. When
# cross-compiling, that executable is for Android and cannot run on Windows, so
# build only the static library and install the pieces librime needs to link.
$openccBuild = Join-Path $buildRoot "opencc"
Invoke-Checked $cmake (@(
    "-S", (Join-Path $LibrimeRoot "deps\opencc"),
    "-B", $openccBuild
) + $commonCMakeArgs + @(
    "-DENABLE_GTEST=OFF",
    "-DENABLE_BENCHMARK=OFF",
    "-DBUILD_DOCUMENTATION=OFF",
    "-DBUILD_PYTHON=OFF",
    "-DUSE_SYSTEM_MARISA=OFF"
)) $LibrimeRoot
Invoke-Checked $cmake @("--build", $openccBuild, "--target", "libopencc", "--", "-j$env:NUMBER_OF_PROCESSORS") $LibrimeRoot
New-Item -ItemType Directory -Force -Path (Join-Path $depsPrefix "include\opencc"), (Join-Path $depsPrefix "lib") | Out-Null
Copy-Item -Force (Join-Path $LibrimeRoot "deps\opencc\src\*.h") (Join-Path $depsPrefix "include\opencc")
Copy-Item -Force (Join-Path $LibrimeRoot "deps\opencc\src\*.hpp") (Join-Path $depsPrefix "include\opencc")
Copy-Item -Force (Join-Path $openccBuild "src\opencc_config.h") (Join-Path $depsPrefix "include\opencc\opencc_config.h")
Copy-Item -Force (Join-Path $openccBuild "src\libopencc.a") (Join-Path $depsPrefix "lib\libopencc.a")

$rimeBuild = Join-Path $buildRoot "librime"
Remove-Item -Recurse -Force $rimeBuild -ErrorAction SilentlyContinue
Ensure-LibrimeLuaPlugin -LibrimeRoot $LibrimeRoot
$env:RIME_PLUGINS = "lua"
$env:RIME_PLUGINS_STRICT = "1"
Invoke-Checked $cmake @(
    "-S", $LibrimeRoot,
    "-B", $rimeBuild,
    "-G", "Unix Makefiles",
    "-DCMAKE_MAKE_PROGRAM=$make",
    "-DCMAKE_TOOLCHAIN_FILE=$toolchain",
    "-DANDROID_ABI=$Abi",
    "-DANDROID_PLATFORM=$AndroidPlatform",
    "-DANDROID_STL=c++_shared",
    "-DCMAKE_BUILD_TYPE=Release",
    "-DCMAKE_INSTALL_PREFIX=$prefix",
    "-DCMAKE_PREFIX_PATH=$depsPrefix",
    "-DBOOST_ROOT=$BoostRoot",
    "-DBoost_INCLUDE_DIR=$BoostRoot",
    "-DBoost_NO_SYSTEM_PATHS=ON",
    "-DLevelDb_INCLUDE_PATH=$(Join-Path $depsPrefix 'include')",
    "-DLevelDb_LIBRARY=$(Join-Path $depsPrefix 'lib\libleveldb.a')",
    "-DMarisa_INCLUDE_PATH=$(Join-Path $depsPrefix 'include')",
    "-DMarisa_LIBRARY=$(Join-Path $depsPrefix 'lib\libmarisa.a')",
    "-DOpencc_INCLUDE_PATH=$(Join-Path $depsPrefix 'include')",
    "-DOpencc_LIBRARY=$(Join-Path $depsPrefix 'lib\libopencc.a')",
    "-DYamlCpp_INCLUDE_PATH=$(Join-Path $depsPrefix 'include')",
    "-DYamlCpp_NEW_API=$(Join-Path $depsPrefix 'include')",
    "-DYamlCpp_LIBRARY=$(Join-Path $depsPrefix 'lib\libyaml-cpp.a')",
    "-DBUILD_SHARED_LIBS=ON",
    "-DBUILD_STATIC=ON",
    "-DBUILD_TEST=OFF",
    "-DENABLE_LOGGING=OFF",
    "-DGlog_LIBRARY=",
    "-DGlog_FOUND=FALSE",
    "-DGflags_FOUND=FALSE",
    "-DCMAKE_CXX_FLAGS=-DBOOST_REGEX_HEADER_ONLY"
) $LibrimeRoot
Invoke-Checked $cmake @("--build", $rimeBuild, "--target", "rime", "--", "-j$env:NUMBER_OF_PROCESSORS") $LibrimeRoot

$librimeSo = Join-Path $rimeBuild "lib\librime.so"
if (-not (Test-Path $librimeSo)) {
    throw "librime.so not produced at $librimeSo"
}

Copy-Item -Force $librimeSo (Join-Path $outDir "librime.so")
$strip = Join-Path $llvmBin "llvm-strip.exe"
if (Test-Path $strip) {
    Invoke-Checked $strip @("--strip-unneeded", (Join-Path $outDir "librime.so")) $LibrimeRoot
}

$runtimeTriples = @{
    "arm64-v8a" = "aarch64-linux-android"
    "armeabi-v7a" = "arm-linux-androideabi"
    "x86" = "i686-linux-android"
    "x86_64" = "x86_64-linux-android"
}
if ($runtimeTriples.ContainsKey($Abi)) {
    $cxxShared = Join-Path $AndroidNdk "toolchains\llvm\prebuilt\windows-x86_64\sysroot\usr\lib\$($runtimeTriples[$Abi])\libc++_shared.so"
    if (Test-Path $cxxShared) {
        Copy-Item -Force $cxxShared (Join-Path $outDir "libc++_shared.so")
    }
}
Write-Host "Built $(Join-Path $outDir 'librime.so')"
