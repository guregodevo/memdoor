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

$Base      = 'https://memdoor.ai/dl'
$Asset     = 'memdoor-windows-amd64.exe'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Memdoor\bin'
$Target    = Join-Path $InstallDir 'memdoor.exe'

function Fail($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; exit 1 }

if ([Environment]::Is64BitOperatingSystem -ne $true) {
    Fail "Memdoor needs 64-bit Windows."
}

Write-Host "Downloading $Asset ..."
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "memdoor-$([guid]::NewGuid()).exe"
try {
    Invoke-WebRequest -Uri "$Base/$Asset" -OutFile $tmp -UseBasicParsing
} catch {
    Fail "could not download $Base/$Asset — $($_.Exception.Message)"
}

# Verify BEFORE the file is ever placed or run. A compromise of the download
# host or a downgraded connection would otherwise be silent code execution on
# every machine that ran this line.
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

$actual = (Get-FileHash -Path $tmp -Algorithm SHA256).Hash.ToLower()
if ($actual -ne $expected) {
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    Fail "checksum mismatch for $Asset.`n  expected $expected`n  got      $actual`nRefusing to install."
}
Write-Host "  ok ($($expected.Substring(0,16))...)"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
# Replacing a RUNNING exe fails on Windows, unlike Unix. Say which process to
# close rather than leaving a locked-file error to interpret.
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
Write-Host "  memdoor connect                your provider's key (or set OPEN_ROUTER_API_KEY)"
Write-Host "  memdoor tui                    put it to work"
Write-Host ""
Write-Host "Models run on your own provider key."
