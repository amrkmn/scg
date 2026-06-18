# --- scg Scoop-compatible helper functions ---
# These functions are injected into every PowerShell hook session to provide
# Scoop-compatible helpers. Manifests that reference Scoop helpers like
# Expand-7zipArchive, Get-HelperPath, etc. will work in scg.

function Get-HelperPath {
    [CmdletBinding()]
    [OutputType([String])]
    param(
        [Parameter(Mandatory=$true, Position=0)]
        [ValidateSet('Git','7zip','Lessmsi','Innounp','Dark','Aria2')]
        [String]$Helper
    )
    process {
        switch ($Helper) {
            '7zip'    { return (Find-HelperApp '7zip' '7z.exe') }
            'Lessmsi' { return (Find-HelperApp 'lessmsi' 'lessmsi.exe') }
            'Innounp' {
                $p = Find-HelperApp 'innounp-unicode' 'innounp.exe'
                if ($p) { return $p }
                return (Find-HelperApp 'innounp' 'innounp.exe')
            }
            'Dark' {
                $p = Find-HelperApp 'wixtoolset' 'wix.exe'
                if ($p) { return $p }
                return (Find-HelperApp 'dark' 'dark.exe')
            }
            'Aria2'   { return (Find-HelperApp 'aria2' 'aria2c.exe') }
            'Git'     {
                $p = Find-HelperApp 'git' 'mingw64\bin\git.exe'
                if ($p) { return $p }
                $p = Find-HelperApp 'git' 'mingw32\bin\git.exe'
                if ($p) { return $p }
                return (Find-CommandPath 'git')
            }
        }
    }
}

function Find-HelperApp {
    param([string]$AppName, [string]$ExeRelPath)
    # Check user and global scoop directories.
    foreach ($base in @($env:scoopdir, "$env:USERPROFILE\scoop", 'C:\ProgramData\scoop')) {
        if (-not $base) { continue }
        $candidate = Join-Path (Join-Path (Join-Path $base 'apps') $AppName) (Join-Path 'current' $ExeRelPath)
        if (Test-Path $candidate) { return $candidate }
    }
    # Fall back to system PATH.
    return (Find-CommandPath ([System.IO.Path]::GetFileName($ExeRelPath)))
}

function Find-CommandPath {
    param([string]$Name)
    $cmd = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue -TotalCount 1
    if ($cmd) { return $cmd.Source }
    return $null
}

function Get-AppFilePath {
    [CmdletBinding()]
    [OutputType([String])]
    param(
        [Parameter(Mandatory=$true, Position=0)][String]$App,
        [Parameter(Mandatory=$true, Position=1)][String]$File
    )
    # Check user scope current/ directory.
    $path = "$(currentdir $App $false)\$File"
    if (Test-Path $path) { return $path }
    # Check global scope.
    $path = "$(currentdir $App $true)\$File"
    if (Test-Path $path) { return $path }
    return $null
}

function Find-BucketDirectory {
    [CmdletBinding()]
    [OutputType([String])]
    param(
        [String]$Name = 'main',
        [Switch]$Root
    )

    if (($null -eq $Name) -or ($Name -eq '')) {
        $Name = 'main'
    }

    $bucket = Join-Path $bucketsdir $Name
    if ((Test-Path (Join-Path $bucket 'bucket')) -and -not $Root) {
        $bucket = Join-Path $bucket 'bucket'
    }

    return $bucket
}

function bucketdir($name) {
    return Find-BucketDirectory -Name $name
}

function appdir($app, $global)  { "$(appsdir $global)\$app" }
function versiondir($app, $ver, $global) { "$(appdir $app $global)\$ver" }
function currentdir($app, $global) {
    if ($global) {
        return "C:\ProgramData\scoop\apps\$app\current"
    }
    if ($env:scoopdir) {
        return "$env:scoopdir\apps\$app\current"
    }
    return "$env:USERPROFILE\scoop\apps\$app\current"
}
function persistdir($app, $global) { "$(basedir $global)\persist\$app" }
function basedir($global) {
    if ($global) { return 'C:\ProgramData\scoop' }
    if ($env:scoopdir) { return $env:scoopdir }
    return "$env:USERPROFILE\scoop"
}
function appsdir($global) { "$(basedir $global)\apps" }

function ensure($dir) {
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    return $dir
}

function movedir($from, $to) {
    $from = $from.TrimEnd('\')
    $to = $to.TrimEnd('\')
    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo.FileName = 'robocopy.exe'
    $proc.StartInfo.Arguments = "`"$from`" `"$to`" /e /move"
    $proc.StartInfo.RedirectStandardOutput = $true
    $proc.StartInfo.RedirectStandardError = $true
    $proc.StartInfo.UseShellExecute = $false
    $proc.StartInfo.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
    [void]$proc.Start()
    $stdoutTask = $proc.StandardOutput.ReadToEndAsync()
    $proc.WaitForExit()
    if ($proc.ExitCode -ge 8) {
        throw "Could not move '$from' to '$to'! (error $($proc.ExitCode))"
    }
    # Wait for robocopy to terminate its threads.
    1..10 | ForEach-Object {
        if (Test-Path $from) { Start-Sleep -Milliseconds 100 }
    }
}

function is_admin {
    $admin = [Security.Principal.WindowsBuiltInRole]::Administrator
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    return ([Security.Principal.WindowsPrincipal]::new($id)).IsInRole($admin)
}

function abort($msg, [int]$exit_code=1) { Write-Host $msg -ForegroundColor Red; exit $exit_code }
function error($msg) { Write-Host "ERROR $msg" -ForegroundColor DarkRed }
function warn($msg) { Write-Host "WARN  $msg" -ForegroundColor DarkYellow }
function info($msg) { Write-Host "INFO  $msg" -ForegroundColor DarkGray }
function success($msg) { Write-Host $msg -ForegroundColor DarkGreen }

function Invoke-ExternalCommand {
    [CmdletBinding(DefaultParameterSetName='Default')]
    [OutputType([Boolean])]
    param(
        [Parameter(Mandatory=$true, Position=0)][Alias('Path')][String]$FilePath,
        [Parameter(Position=1)][Alias('Args')][String[]]$ArgumentList,
        [Parameter(ParameterSetName='UseShellExecute')][Switch]$RunAs,
        [Parameter(ParameterSetName='UseShellExecute')][Switch]$Quiet,
        [Alias('Msg')][String]$Activity,
        [Alias('cec')][Hashtable]$ContinueExitCodes,
        [Parameter(ParameterSetName='Default')][Alias('Log')][String]$LogPath
    )

    if ($Activity) {
        Write-Host "$Activity " -NoNewline
    }

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo.FileName = $FilePath
    $proc.StartInfo.UseShellExecute = $false

    if ($LogPath) {
        $leaf = [System.IO.Path]::GetFileName($FilePath)
        if ($leaf -match '^(?i:msiexec(?:\.exe)?)$') {
            $ArgumentList += "/lwe `"$LogPath`""
        } else {
            $redirectToLogFile = $true
            $proc.StartInfo.RedirectStandardOutput = $true
            $proc.StartInfo.RedirectStandardError = $true
        }
    }

    if ($RunAs) {
        $proc.StartInfo.UseShellExecute = $true
        $proc.StartInfo.Verb = 'RunAs'
    }
    if ($Quiet) {
        $proc.StartInfo.UseShellExecute = $true
        $proc.StartInfo.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
    }

    if ($ArgumentList -and $ArgumentList.Length -gt 0) {
        $legacyCommand = $FilePath -match '^((cmd|cscript|find|sqlcmd|wscript|msiexec)(\.exe)?|.*\.(bat|cmd|js|vbs|wsf))$' -or
            ($ArgumentList -match '^/S$|^/D=[A-Z]:[\\/].*$').Length -eq 2
        $supportsArgumentList = $proc.StartInfo.PSObject.Properties.Name -contains 'ArgumentList'
        if ((-not $legacyCommand) -and $supportsArgumentList) {
            $ArgumentList.ForEach({ $proc.StartInfo.ArgumentList.Add($_) })
        } else {
            $escapedArgs = switch -regex ($ArgumentList) {
                '(?<!/D=)[A-Z]:[\\/].*' { $_ -replace '([A-Z]:[\\/].*)', '"$1"'; continue }
                '/D=[A-Z]:[\\/].*' { $_; continue }
                ' ' { "`"$_`""; continue }
                default { $_; continue }
            }
            $proc.StartInfo.Arguments = $escapedArgs -join ' '
        }
    }

    try {
        [void]$proc.Start()
    } catch {
        if ($Activity) {
            Write-Host 'error.' -ForegroundColor DarkRed
        }
        error $_.Exception.Message
        return $false
    }

    if ($redirectToLogFile) {
        $stdoutTask = $proc.StandardOutput.ReadToEndAsync()
        $stderrTask = $proc.StandardError.ReadToEndAsync()
    }

    $proc.WaitForExit()

    if ($redirectToLogFile) {
        $logDir = Split-Path -Parent $LogPath
        if ($logDir) {
            ensure $logDir | Out-Null
        }
        [System.IO.File]::AppendAllText($LogPath, $stdoutTask.Result)
        [System.IO.File]::AppendAllText($LogPath, $stderrTask.Result)
    }

    if ($proc.ExitCode -ne 0) {
        $continueMessage = $null
        if ($ContinueExitCodes) {
            if ($ContinueExitCodes.ContainsKey($proc.ExitCode)) {
                $continueMessage = $ContinueExitCodes[$proc.ExitCode]
            } elseif ($ContinueExitCodes.ContainsKey([string]$proc.ExitCode)) {
                $continueMessage = $ContinueExitCodes[[string]$proc.ExitCode]
            }
        }
        if ($null -ne $continueMessage) {
            if ($Activity) {
                Write-Host 'done.' -ForegroundColor DarkYellow
            }
            warn $continueMessage
            return $true
        }
        if ($Activity) {
            Write-Host 'error.' -ForegroundColor DarkRed
        }
        error "Exit code was $($proc.ExitCode)!"
        return $false
    }

    if ($Activity) {
        Write-Host 'done.' -ForegroundColor Green
    }

    return $true
}

function Expand-7zipArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true, Position=0, ValueFromPipeline=$true)]
        [String]$Path,
        [Parameter(Position=1)]
        [String]$DestinationPath = (Split-Path $Path),
        [String]$ExtractDir,
        [Parameter(ValueFromRemainingArguments=$true)]
        [String]$Switches,
        [ValidateSet('All','Skip','Rename')]
        [String]$Overwrite,
        [Switch]$Removal
    )
    # Resolve 7z path using Scoop-compatible helper.
    $7zPath = Get-HelperPath -Helper 7zip
    if (-not $7zPath) {
        throw '7-Zip is required. Run: scg install 7zip'
    }

    $DestinationPath = $DestinationPath.TrimEnd('\')
    $ArgList = @('x', $Path, "-o$DestinationPath", '-xr!*.nsis', '-y')
    $IsTar = (([System.IO.Path]::GetFileNameWithoutExtension($Path) -match '\.tar$') -or ($Path -match '\.t[abgpx]z2?$'))
    if (-not $IsTar -and $ExtractDir) {
        $ArgList += "-ir!$ExtractDir\*"
    }
    if ($Switches) { $ArgList += (-split $Switches) }
    switch ($Overwrite) {
        'All'    { $ArgList += '-aoa' }
        'Skip'   { $ArgList += '-aos' }
        'Rename' { $ArgList += '-aou' }
    }
    $Status = Invoke-ExternalCommand $7zPath $ArgList
    if (-not $Status) {
        throw "Failed to extract files from $Path."
    }

    if ($IsTar) {
        # List to find inner tar file.
        $output = & $7zPath l "$Path" 2>&1
        $tarFile = ($output | Where-Object { $_ -match '\.tar\s*$' } | Select-Object -First 1).Trim()
        if ($tarFile) {
            $tarFileName = ($tarFile -split '\s+')[-1]
            if ($tarFileName) {
                Expand-7zipArchive -Path "$DestinationPath\$tarFileName" -DestinationPath $DestinationPath -ExtractDir $ExtractDir -Removal
            }
        }
    }

    if (-not $IsTar -and $ExtractDir) {
        movedir "$DestinationPath\$ExtractDir" $DestinationPath | Out-Null
        $topPath = "$DestinationPath\$(($ExtractDir -split '[\\/]')[0])"
        if ((Test-Path $topPath) -and (Get-ChildItem $topPath -Force -ErrorAction Ignore).Count -eq 0) {
            Remove-Item $topPath -Recurse -Force -ErrorAction Ignore
        }
    }

    if ($Removal) {
        Remove-Item $Path -Force -ErrorAction Ignore
    }
}

function Expand-MsiArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true, Position=0, ValueFromPipeline=$true)]
        [String]$Path,
        [Parameter(Position=1)]
        [String]$DestinationPath = (Split-Path $Path),
        [String]$ExtractDir,
        [Parameter(ValueFromRemainingArguments=$true)]
        [String]$Switches,
        [Switch]$Removal
    )
    $DestinationPath = $DestinationPath.TrimEnd('\')
    if ($ExtractDir) {
        $OriDestinationPath = $DestinationPath
        $DestinationPath = "$DestinationPath\_tmp"
    }
    $MsiPath = Get-HelperPath -Helper Lessmsi
    if ($MsiPath) {
        $ArgList = @('x', $Path, "$DestinationPath\")
    } else {
        $MsiPath = 'msiexec.exe'
        $ArgList = @('/a', $Path, '/qn', "TARGETDIR=$DestinationPath\SourceDir")
    }
    if ($Switches) { $ArgList += (-split $Switches) }
    $Status = Invoke-ExternalCommand $MsiPath $ArgList
    if (-not $Status) { throw "Failed to extract files from $Path." }

    if ($ExtractDir -and (Test-Path "$DestinationPath\SourceDir")) {
        movedir "$DestinationPath\SourceDir\$ExtractDir" $OriDestinationPath | Out-Null
        Remove-Item $DestinationPath -Recurse -Force
    } elseif ($ExtractDir) {
        movedir "$DestinationPath\$ExtractDir" $OriDestinationPath | Out-Null
        Remove-Item $DestinationPath -Recurse -Force
    } elseif (Test-Path "$DestinationPath\SourceDir") {
        movedir "$DestinationPath\SourceDir" $DestinationPath | Out-Null
    }
    if ($Removal) { Remove-Item $Path -Force }
}

function Expand-InnoArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true, Position=0, ValueFromPipeline=$true)]
        [String]$Path,
        [Parameter(Position=1)]
        [String]$DestinationPath = (Split-Path $Path),
        [String]$ExtractDir,
        [Parameter(ValueFromRemainingArguments=$true)]
        [String]$Switches,
        [Switch]$Removal
    )
    $InnoPath = Get-HelperPath -Helper Innounp
    if (-not $InnoPath) {
        throw 'Inno Setup Unpacker is required. Run: scg install innounp'
    }
    $ArgList = @('-x', "-d$DestinationPath", $Path, '-y')
    switch -Regex ($ExtractDir) {
        '^[^{].*' { $ArgList += "-c{app}\$ExtractDir" }
        '^{.*'    { $ArgList += "-c$ExtractDir" }
        Default   { $ArgList += '-c{app}' }
    }
    if ($Switches) { $ArgList += (-split $Switches) }
    $Status = Invoke-ExternalCommand $InnoPath $ArgList
    if (-not $Status) { throw "Failed to extract files from $Path." }
    if ($Removal) { Remove-Item $Path -Force }
}

function Expand-DarkArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true, Position=0, ValueFromPipeline=$true)]
        [String]$Path,
        [Parameter(Position=1)]
        [String]$DestinationPath = (Split-Path $Path),
        [Parameter(ValueFromRemainingArguments=$true)]
        [String]$Switches,
        [Switch]$Removal
    )
    $DarkPath = Get-HelperPath -Helper Dark
    if (-not $DarkPath) {
        throw 'WiX Toolset (dark) is required. Run: scg install wixtoolset'
    }
    $ArgList = @('-nologo', '-x', $DestinationPath, $Path)
    if ($Switches) { $ArgList += (-split $Switches) }
    $Status = Invoke-ExternalCommand $DarkPath $ArgList
    if (-not $Status) { throw "Failed to extract files from $Path." }
    if ($Removal) { Remove-Item $Path -Force }
}

function Expand-ZipArchive {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$true, Position=0, ValueFromPipeline=$true)]
        [String]$Path,
        [Parameter(Position=1)]
        [String]$DestinationPath = (Split-Path $Path),
        [String]$ExtractDir,
        [Switch]$Removal
    )
    if ($ExtractDir) {
        $OriDestinationPath = $DestinationPath
        $DestinationPath = "$DestinationPath\_tmp"
    }
    $oldProgressPreference = $ProgressPreference
    $global:ProgressPreference = 'SilentlyContinue'
    Microsoft.PowerShell.Archive\Expand-Archive -Path $Path -DestinationPath $DestinationPath -Force
    $global:ProgressPreference = $oldProgressPreference
    if ($ExtractDir) {
        movedir "$DestinationPath\$ExtractDir" $OriDestinationPath | Out-Null
        Remove-Item $DestinationPath -Recurse -Force
    }
    if ($Removal) { Remove-Item $Path -Force }
}

# --- End scg Scoop-compatible helper functions ---
