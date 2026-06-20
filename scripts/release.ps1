param(
    [Parameter(Position = 0)]
    [ValidateSet('major', 'minor', 'patch')]
    [string]$Bump = 'patch',

    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

function git {
    $gitPath = (Get-Command git.exe -ErrorAction Stop).Source
    Push-Location $root
    try { & $gitPath @args } finally { Pop-Location }
}

function Get-LatestTag {
    try {
        $tag = git tag -l 'v*' --sort=-v:refname 2>$null |
            Where-Object { $_ -match '^v\d+\.\d+\.\d+$' } |
            Select-Object -First 1
        return $tag
    } catch { return $null }
}

function New-Version($version, $type) {
    $parts = ($version -replace '^v', '') -split '\.'
    $major, $minor, $patch = [int]$parts[0], [int]$parts[1], [int]$parts[2]
    switch ($type) {
        'major' { return "v$($major + 1).0.0" }
        'minor' { return "v$major.$($minor + 1).0" }
        'patch' { return "v$major.$minor.$($patch + 1)" }
    }
}

function Assert-GitClean {
    $status = git status --porcelain 2>$null
    if ($status) {
        throw "Uncommitted changes found. Commit or stash them first."
    }
    $branch = git rev-parse --abbrev-ref HEAD 2>$null
    if ($branch -ne 'main') {
        throw "Must be on main branch to release (current: $branch)."
    }
    $local = git rev-parse HEAD 2>$null
    $remote = git rev-parse '@{u}' 2>$null
    if ($remote -and $local -ne $remote) {
        throw "Branch out of sync with remote. Push or pull first."
    }
}

# ---- main ----

$current = Get-LatestTag
if (-not $current) { $current = 'v0.0.0' }
$next = New-Version $current $Bump

Write-Host ''
Write-Host "Releasing scg"
Write-Host "   $current -> $next"
Write-Host ''

if ($DryRun) {
    Write-Host 'Dry run -- no changes will be made'
    Write-Host ''
    Write-Host 'Would perform:'
    Write-Host '  1. Check git status and branch'
    Write-Host '  2. Run tests'
    Write-Host '  3. Build all architectures'
    Write-Host "  4. Commit: chore: release $next"
    Write-Host "  5. Create tag $next"
    Write-Host '  6. Push to origin/main and tags'
    Write-Host "  7. Create GitHub release $next"
    exit 0
}

Assert-GitClean

if (git tag -l $next) {
    throw "Tag $next already exists locally. Delete it first: git tag -d $next"
}
try {
    $remoteTags = git ls-remote --tags origin "refs/tags/$next" 2>$null
} catch { $remoteTags = $null }
if ($remoteTags) {
    throw "Tag $next already exists on remote. Delete it first: git push origin :refs/tags/$next"
}

Write-Host 'Git checks passed'
Write-Host ''

Write-Host 'Running tests...'
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
Write-Host ''

Write-Host "Committing: chore: release $next"
git add -A
git commit -m "chore: release $next"

Write-Host "Creating tag $next..."
git tag -a $next -m "chore: release $next"

try {
    Write-Host 'Pushing to origin/main...'
    git push origin main

    Write-Host "Pushing tag $next..."
    git push origin $next
} catch {
    Write-Host ''
    Write-Host 'Push failed! Rolling back...'
    git tag -d $next
    git reset --hard HEAD~1
    throw "Push failed -- rolled back. $($_.Exception.Message)"
}

Write-Host ''
Write-Host "Successfully pushed $next!"
Write-Host 'CI will build and create the release.'
Write-Host ''
