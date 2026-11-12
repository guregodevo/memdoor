# The first things a person does on Windows after the install, run against the
# memdoor.exe the installer put in %LOCALAPPDATA%\Memdoor\bin: the version, a
# gateway that answers, the first account, then the first commands with no key
# and no gateway running. Every command answers in words (no panic, no "not
# supported"), with the exit code it has on the author's machine.
#
# Run by .github/workflows/windows.yml on each pull request's own build, and by
# installers.yml on what memdoor.ai serves after a deploy.

$ErrorActionPreference = 'Stop'
$bin = Join-Path $env:LOCALAPPDATA 'Memdoor\bin\memdoor.exe'
$status = 'http://127.0.0.1:18789/api/status'

function Wait-Gateway($seconds) {
    foreach ($i in 1..$seconds) {
        try { Invoke-WebRequest -UseBasicParsing $status | Out-Null; return $true } catch { Start-Sleep 1 }
    }
    return $false
}

# Version, gateway, status, the first account.
& $bin --version
if ($LASTEXITCODE -ne 0) { throw "memdoor --version exit $LASTEXITCODE" }
$gw = Start-Process -FilePath $bin -ArgumentList 'gateway' -PassThru -RedirectStandardOutput gw.log -RedirectStandardError gw.err
if (-not (Wait-Gateway 40)) { Get-Content gw.log, gw.err; throw "gateway did not answer" }
(Invoke-WebRequest -UseBasicParsing $status).Content
& $bin setup --non-interactive --admin-email ci@example.test --admin-password ci-first-run-2026 --workspace-name ci
if ($LASTEXITCODE -ne 0) { throw "memdoor setup exit $LASTEXITCODE" }
Stop-Process -Id $gw.Id
Start-Sleep 2

# A new shell finds memdoor on the user PATH the installer wrote.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*Memdoor\bin*") { throw "installer did not put Memdoor\bin on the user PATH" }
$env:Path = "$env:Path;" + (Split-Path $bin)
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\proj" | Out-Null
Set-Location "$env:USERPROFILE\proj"

function Check($want, [string[]]$cmd) {
    $out = & memdoor @cmd 2>&1 | Out-String; $rc = $LASTEXITCODE
    Write-Host "--- memdoor $cmd -> exit $rc"; Write-Host ($out.Substring(0, [Math]::Min(400, $out.Length)))
    if ($out -match 'panic|not supported') { throw "crashed: memdoor $cmd" }
    if ($rc -ne $want) { throw "memdoor $cmd exit $rc, want $want" }
}
# No key and no gateway: `memdoor run` starts the gateway itself and says it has no model.
Check 1 @('run', 'reply with ok')
if (-not (Wait-Gateway 5)) { throw "memdoor run did not start a gateway" }
Check 0 @('doctor')
Check 0 @('providers')
Check 0 @('workflow', 'list')
Check 0 @('cron', 'list')
Check 0 @('resume')
Check 1 @('resume', '--last')
Check 1 @('remote', '--last')
exit 0   # the last command's expected 1 would otherwise be the step's exit code
