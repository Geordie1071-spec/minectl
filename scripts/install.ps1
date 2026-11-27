# minectl installer for Windows.
#
#   irm https://raw.githubusercontent.com/Geordie1071-spec/minectl/main/scripts/install.ps1 | iex
#
# Installs Docker Desktop (if missing, via winget) and minectl, puts minectl on
# your PATH and, when run as administrator, opens the Minecraft ports in
# Windows Firewall. Re-run it to update.
#
# Env vars:
#   MINECTL_VERSION=v2.0.0       install this release instead of the latest
#   MINECTL_INSTALL_DIR=path     (default: %LOCALAPPDATA%\Programs\minectl)
#   MINECTL_SKIP_DOCKER=1        don't install Docker Desktop

# Everything runs inside a function: under "irm | iex", exit would close the
# user's window, so failures return instead.
function Install-Minectl {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is very slow with the progress bar
    # Windows PowerShell 5.1 doesn't offer TLS 1.2 by default; GitHub requires it.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'Geordie1071-spec/minectl'
    $installDir = $env:MINECTL_INSTALL_DIR
    if (-not $installDir) { $installDir = Join-Path $env:LOCALAPPDATA 'Programs\minectl' }

    Write-Host 'minectl installer'
    Write-Host '================='

    # Windows on ARM runs the amd64 build through emulation.
    $arch = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) { $arch = $env:PROCESSOR_ARCHITEW6432 }
    if ($arch -ne 'AMD64' -and $arch -ne 'ARM64') {
        Write-Host "minectl needs 64-bit Windows (this is $arch)." -ForegroundColor Red
        return
    }

    # 1. Docker Desktop.
    $restartNeeded = $false
    if ($env:MINECTL_SKIP_DOCKER) {
        Write-Host 'Skipping Docker (MINECTL_SKIP_DOCKER is set).'
    } elseif (Get-Command docker -ErrorAction SilentlyContinue) {
        Write-Host "Docker: $((Get-Command docker).Source)"
    } elseif (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Host 'Docker not found. Installing Docker Desktop (Windows will ask for permission)...'
        winget install -e --id Docker.DockerDesktop --accept-source-agreements --accept-package-agreements
        if ($LASTEXITCODE -ne 0) {
            Write-Host 'Docker Desktop did not install. Install it from https://docs.docker.com/desktop/setup/install/windows-install/ and run this again.' -ForegroundColor Red
            return
        }
        $restartNeeded = $true
    } else {
        Write-Host 'Docker not found, and winget is not available to install it.' -ForegroundColor Red
        Write-Host 'Install Docker Desktop from https://docs.docker.com/desktop/setup/install/windows-install/ and run this again.'
        return
    }

    # 2. The release. Never guess a version: an old release can't read a newer database.
    $version = $env:MINECTL_VERSION
    if (-not $version) {
        try {
            $version = (Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$repo/releases/latest").tag_name
        } catch {
            Write-Host "Couldn't find the latest minectl release (GitHub may be rate-limiting this machine)." -ForegroundColor Red
            Write-Host 'Try again in a while, or pick a version: $env:MINECTL_VERSION = "v2.0.0"'
            return
        }
    }
    $archive = "minectl_$($version.TrimStart('v'))_windows_amd64.zip"
    $base = "https://github.com/$repo/releases/download/$version"
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("minectl-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading minectl $version..."
        $zip = Join-Path $tmp $archive
        try {
            Invoke-WebRequest -UseBasicParsing "$base/$archive" -OutFile $zip
            Invoke-WebRequest -UseBasicParsing "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
        } catch {
            Write-Host "Download failed: $($_.Exception.Message)" -ForegroundColor Red
            Write-Host "Download it yourself from https://github.com/$repo/releases"
            return
        }
        $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" } | Select-Object -First 1
        if (-not $line) {
            Write-Host "checksums.txt has no entry for $archive. Not installing it." -ForegroundColor Red
            return
        }
        $want = ($line -split '\s+')[0]
        $got = (Get-FileHash -Algorithm SHA256 $zip).Hash
        if ($got -ne $want) { # -ne ignores case: Get-FileHash is upper case, checksums.txt lower
            Write-Host "The download doesn't match the release checksum. Not installing it." -ForegroundColor Red
            return
        }
        Write-Host 'Checksum OK.'

        Expand-Archive -Path $zip -DestinationPath $tmp -Force
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        $exe = Join-Path $installDir 'minectl.exe'
        try {
            Copy-Item (Join-Path $tmp 'minectl.exe') $exe -Force
        } catch {
            Write-Host "Couldn't replace $exe. If minectl (or its panel) is running, close it and run this again." -ForegroundColor Red
            return
        }
        Write-Host "Installed minectl $version to $exe"
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }

    # 3. PATH, for new terminals and this one.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    if (($userPath -split ';') -notcontains $installDir) {
        [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';') + ";$installDir").TrimStart(';')), 'User')
        Write-Host "Added $installDir to your PATH."
    }
    if (($env:Path -split ';') -notcontains $installDir) { $env:Path += ";$installDir" }

    # 4. Windows Firewall, which needs administrator rights.
    $admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    $ruleName = 'minectl (Minecraft)'
    $ruleCmd = "New-NetFirewallRule -DisplayName '$ruleName' -Direction Inbound -Protocol TCP -LocalPort 25565-25575 -Action Allow"
    $firewallDone = $false
    if ($admin) {
        if (-not (Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue)) {
            Invoke-Expression $ruleCmd | Out-Null
            Write-Host 'Opened TCP 25565-25575 in Windows Firewall.'
        }
        $firewallDone = $true
    }

    Write-Host ''
    Write-Host 'Next steps:' -ForegroundColor Green
    $n = 1
    if ($restartNeeded) {
        Write-Host "  $n. Restart Windows if Docker Desktop asks to, then open Docker Desktop and wait until it says it's running."
        Write-Host '     (It may ask to install or update WSL first: say yes.)'
        $n++
    }
    if (-not $firewallDone) {
        Write-Host "  $n. So friends on your network can join, open the ports in an administrator PowerShell:"
        Write-Host "       $ruleCmd"
        $n++
    }
    Write-Host "  $n. In a new terminal:"
    Write-Host '       minectl create -n myserver -t paper -m 4G'
    Write-Host '       minectl list'
    $n++
    Write-Host "  $n. To let friends join from the internet: minectl public myserver"
    Write-Host ''
    Write-Host 'To update later, run this installer again.'
}

Install-Minectl
