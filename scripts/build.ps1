param(
    [string]$Version,
    [ValidateSet('amd64', '386', 'arm64')]
    [string]$Arch,
    [switch]$All
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

if (-not $Version) {
    if ($env:VERSION) {
        $Version = $env:VERSION
    } else {
        try {
            $sha = git -C $root rev-parse --short HEAD 2>$null
        } catch {
            $sha = 'unknown'
        }
        $date = (Get-Date -Format 'yyyy.MM.dd')
        $Version = "v0.0.0-dev.$date.$sha"
    }
}

function Get-NativeArch {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        'AMD64'  { return 'amd64' }
        'x86'    { return '386' }
        'ARM64'  { return 'arm64' }
        default  { return 'amd64' }
    }
}

function Build-One {
    param(
        [string]$TargetArch,
        [string]$OutputName
    )

    if ($TargetArch -eq 'amd64') {
        $zigTarget = 'x86_64-windows'
        $zigPrefix = 'zig-out/x64'
        $goos = 'windows'
        $goarch = 'amd64'
    } elseif ($TargetArch -eq '386') {
        $zigTarget = 'x86-windows'
        $zigPrefix = 'zig-out/x86'
        $goos = 'windows'
        $goarch = '386'
    } else {
        $zigTarget = 'aarch64-windows'
        $zigPrefix = 'zig-out/arm64'
        $goos = 'windows'
        $goarch = 'arm64'
    }

    # Build shim for target arch
    Push-Location "$root\shim"
    try {
        & zig build "-Dtarget=$zigTarget" --prefix $zigPrefix
    } finally {
        Pop-Location
    }

    $assetsDir = "$root\internal\install\assets"
    if (-not (Test-Path $assetsDir)) {
        New-Item -ItemType Directory -Path $assetsDir -Force | Out-Null
    }
    Copy-Item "$root\shim\$zigPrefix\bin\shim.exe" $assetsDir -Force

    # Build Go binary
    $distDir = "$root\dist"
    if (-not (Test-Path $distDir)) {
        New-Item -ItemType Directory -Path $distDir -Force | Out-Null
    }

    Write-Host "Building scg $Version [$TargetArch] ..."
    $env:CGO_ENABLED = '0'
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    go build -ldflags "-X main.Version=$Version -s -w" -o "$distDir\$OutputName" ./cmd
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed for $TargetArch"
    }
    Write-Host "Done: $distDir\$OutputName"
}

if ($All) {
    Build-One 'amd64' 'scg-amd64.exe'
    Build-One '386'   'scg-386.exe'
    Build-One 'arm64' 'scg-arm64.exe'
    return
}

$targetArch = if ($Arch) { $Arch } else { Get-NativeArch }
$outputName = if ($Arch) { "scg-$Arch.exe" } else { 'scg.exe' }
Build-One $targetArch $outputName
