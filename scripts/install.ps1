# Memdoor installer for Windows.
#
#   irm https://memdoor.ai/install.ps1 | iex
#
# Mirrors scripts/install.sh: download from /dl, VERIFY against the published
# SHA256SUMS before anything executes, install user-local without admin
# rights, and put it on PATH. Read it before you run it — that is the whole
# reason it is a plain script and not a packed installer.
#
# What you get on Windows is the same as everywhere: the TUI and the agent,
# with every model on your own provider key.

$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1, the one most people run `irm | iex` in, redraws the
# progress bar in the console for every chunk Invoke-WebRequest receives, a
# known slowdown of large downloads: the 40 MB exe took minutes to arrive, and
# some never finished (November 2026). The bar is off; the lines this script
# prints say what it is doing. (A CI runner draws no bar, so it cannot show the
# difference: 0.7 s with it and without.)
$ProgressPreference = 'SilentlyContinue'

# `irm | iex` runs this script inside the person's own session: `exit` there
# closes their window, and the error with it. A failure throws instead, and
# the message stays on the screen (a file run, powershell -File, still exits 1).
function Fail($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; throw 'memdoor was not installed.' }

$Base      = 'https://memdoor.ai/dl'
# A GitHub release instead of memdoor.ai: MEMDOOR_INSTALL_SOURCE=github, the
# latest of MEMDOOR_GITHUB_REPOSITORY (default guregodevo/memdoor) or the tag
# in MEMDOOR_GITHUB_RELEASE. Every release carries the binaries and SHA256SUMS.
switch ("$env:MEMDOOR_INSTALL_SOURCE") {
    { $_ -in '', 'memdoor.ai' } { break }
    'github' {
        $repo = if ($env:MEMDOOR_GITHUB_REPOSITORY) { $env:MEMDOOR_GITHUB_REPOSITORY } else { 'guregodevo/memdoor' }
        if ($repo -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') { Fail "MEMDOOR_GITHUB_REPOSITORY=$repo is not owner/repository." }
        if ($env:MEMDOOR_GITHUB_RELEASE) {
            if ($env:MEMDOOR_GITHUB_RELEASE -notmatch '^[A-Za-z0-9_.+-]+$') { Fail "MEMDOOR_GITHUB_RELEASE=$($env:MEMDOOR_GITHUB_RELEASE) is not a release tag." }
            $Base = "https://github.com/$repo/releases/download/$($env:MEMDOOR_GITHUB_RELEASE)"
        } else {
            $Base = "https://github.com/$repo/releases/latest/download"
        }
        break
    }
    default { Fail "MEMDOOR_INSTALL_SOURCE=$($env:MEMDOOR_INSTALL_SOURCE): set it to github, or unset it for memdoor.ai." }
}
$Asset     = 'memdoor-windows-amd64.exe'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Memdoor\bin'
$Target    = Join-Path $InstallDir 'memdoor.exe'

if ([Environment]::Is64BitOperatingSystem -ne $true) {
    Fail "Memdoor needs 64-bit Windows."
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "memdoor-$([guid]::NewGuid()).exe"

# Downloads $Asset to $tmp and returns its expected and actual hashes. Verify
# BEFORE the file is ever placed or run: a compromise of the download host or
# a downgraded connection would otherwise be silent code execution on every
# machine that ran this line.
function Get-VerifiedDownload {
    Write-Host "Downloading $Asset ..."
    # Our own test runs mark their download so adoption counts people only.
    $ciMark = if ($env:MEMDOOR_CI -and $Base -eq 'https://memdoor.ai/dl') { "?ci=1" } else { "" }
    # Three tries: a connection that drops halfway is not the end of the install.
    $err = $null
    foreach ($try in 1..3) {
        try {
            Invoke-WebRequest -Uri "$Base/$Asset$ciMark" -OutFile $tmp -UseBasicParsing
            $err = $null
            break
        } catch {
            $err = $_.Exception.Message
            Remove-Item $tmp -Force -ErrorAction SilentlyContinue
            if ($try -lt 3) {
                Write-Host "  the download stopped ($err); trying again ($($try + 1) of 3) ..."
                Start-Sleep -Seconds 2
            }
        }
    }
    if ($err) { Fail "could not download $Base/$Asset (3 tries) — $err" }

    Write-Host "Verifying checksum ..."
    try {
        # /dl/ is served as application/octet-stream (so browsers download the
        # binaries), and for that type PowerShell hands back bytes, not text: the
        # manifest then matched nothing and every Windows install was refused
        # (found 2026-10-06). Decode it as text whatever the server says.
        $resp = Invoke-WebRequest -Uri "$Base/SHA256SUMS" -UseBasicParsing
        if ($resp.Content -is [byte[]]) { $sums = [System.Text.Encoding]::UTF8.GetString($resp.Content) }
        else { $sums = [string]$resp.Content }
    } catch {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "could not fetch $Base/SHA256SUMS — refusing to install an unverified binary."
    }

    $expected = $null
    foreach ($line in $sums -split "`n") {
        # sha256sum format: "<hash>  <filename>"
        $parts = ($line.Trim() -split '\s+')
        if ($parts.Count -ge 2 -and $parts[-1] -eq $Asset) { $expected = $parts[0].ToLower() }
    }
    if (-not $expected) {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "$Asset is not listed in SHA256SUMS — refusing to install."
    }
    @{ Expected = $expected; Actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLower() }
}

# The binary is fetched before the manifest, so an install that straddles a
# deploy's switch (under a second, deploy.sh step 8) can hold one release's
# binary and the next one's hashes. Both are fetched again once, a few
# seconds later, when they are the same release; a second mismatch is not a
# deploy and the install is refused.
#
# MEMDOOR_BINARY is an exe already built (the Windows CI installs each pull
# request's build this way, as `make install` hands install.sh its own): it is
# placed like a download, with no download and no checksum.
if ($env:MEMDOOR_BINARY) {
    if (-not (Test-Path -PathType Leaf $env:MEMDOOR_BINARY)) { Fail "MEMDOOR_BINARY=$($env:MEMDOOR_BINARY) is not a file." }
    Write-Host "Installing $($env:MEMDOOR_BINARY)"
    Copy-Item -Path $env:MEMDOOR_BINARY -Destination $tmp -Force
} else {
    $hashes = Get-VerifiedDownload
    if ($hashes.Actual -ne $hashes.Expected) {
        Write-Host "checksum mismatch for $Asset; a release may have been switching: fetching both again in 5s"
        Start-Sleep -Seconds 5
        $hashes = Get-VerifiedDownload
    }
    if ($hashes.Actual -ne $hashes.Expected) {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "checksum mismatch for $Asset.`n  expected $($hashes.Expected)`n  got      $($hashes.Actual)`nRefusing to install."
    }
    Write-Host "  ok ($($hashes.Expected.Substring(0,16))...)"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
# Replacing a RUNNING exe fails on Windows, unlike Unix, and `memdoor upgrade`
# is that exe running this script. A running exe can be RENAMED: the one in
# place moves aside, the new one takes its name, and the next install sweeps
# the copies set aside (one may still be running; it is left for the one after).
Get-ChildItem -Path $InstallDir -Filter 'memdoor.old-*.exe' -ErrorAction SilentlyContinue |
    Remove-Item -Force -ErrorAction SilentlyContinue
if (Test-Path $Target) {
    $aside = Join-Path $InstallDir "memdoor.old-$([guid]::NewGuid().ToString('N').Substring(0, 8)).exe"
    try {
        Move-Item -Path $Target -Destination $aside -Force
    } catch {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "could not move $Target aside — close any running memdoor and try again.`n  $($_.Exception.Message)"
    }
}
try {
    Move-Item -Path $tmp -Destination $Target -Force
} catch {
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    Fail "could not write $Target — close any running memdoor and try again.`n  $($_.Exception.Message)"
}

# PATH for future shells, user scope only: no admin rights, and nothing
# outside this account is touched.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
    Write-Host "Added $InstallDir to your PATH (new terminals will see it)."
}
# ...and for THIS shell, so the next line of this session works.
if ($env:Path -notlike "*$InstallDir*") { $env:Path = "$env:Path;$InstallDir" }

Write-Host ""
& $Target version
Write-Host ""
Write-Host "Installed to $Target" -ForegroundColor Green
Write-Host ""
Write-Host "Next:"
Write-Host "  memdoor                        put it to work: it finds a key on this machine, or signs you in with OpenRouter (free)"
Write-Host "  memdoor connect                a provider's key (or set OPENROUTER_API_KEY)"
