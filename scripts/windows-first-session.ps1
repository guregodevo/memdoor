# A newcomer's first session on Windows, on the memdoor.exe the installer put
# in %LOCALAPPDATA%\Memdoor\bin: memdoor connect deepseek with the key, a
# `memdoor run` turn, then the gateway stopped and the next `memdoor run`
# starting it again with the key read from providers.json. The model is the
# stand-in provider (internal/standin): one fixed reply, only to its own key.
# The window and the pasted key are scripts/first-session.sh's, on macOS and
# Linux: Windows has no tmux to drive them.
#
# Env: STANDIN_BIN (required), MEMDOOR_BIN, PORT (default 18792, clear of a
# gateway already on 18789), STANDIN_PORT (default 18961).
#
# Run by .github/workflows/installers.yml after each deploy.

$ErrorActionPreference = 'Stop'
$bin = if ($env:MEMDOOR_BIN) { $env:MEMDOOR_BIN } else { Join-Path $env:LOCALAPPDATA 'Memdoor\bin\memdoor.exe' }
if (-not $env:STANDIN_BIN) { throw 'STANDIN_BIN: go build -o standin.exe ./internal/standin' }
$port = if ($env:PORT) { $env:PORT } else { '18792' }
$standinPort = if ($env:STANDIN_PORT) { $env:STANDIN_PORT } else { '18961' }
$gateway = "http://127.0.0.1:$port"
$standinUrl = "http://127.0.0.1:$standinPort/v1"
$reply = 'Hello from the stand-in provider.'
$key = 'sk-' + -join (1..16 | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })

$work = Join-Path ([IO.Path]::GetTempPath()) ("memdoor-first-session-" + [guid]::NewGuid().ToString('N').Substring(0, 8))
$homeDir = Join-Path $work 'home'
$proj = Join-Path $work 'proj'
New-Item -ItemType Directory -Force -Path $homeDir, $proj | Out-Null

function Gateway-Pid {
    try { return (Invoke-RestMethod -TimeoutSec 2 "$gateway/api/status").pid } catch { return $null }
}

function Stop-Gateway {
    $id = Gateway-Pid
    if (-not $id) { return }
    Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
    foreach ($i in 1..30) { if (-not (Gateway-Pid)) { return }; Start-Sleep 1 }
    throw "the gateway on :$port did not stop"
}

# memdoor in the scenario's own home, with this run's variables only.
function Memdoor([string[]]$cmd) {
    $out = & $bin --gateway $gateway @cmd 2>&1 | Out-String
    $rc = $LASTEXITCODE
    Write-Host "--- memdoor $($cmd -join ' ') -> exit $rc"
    Write-Host (($out -replace [regex]::Escape($key), '<key>').Trim())
    if ($out -match 'panic') { throw "crashed: memdoor $cmd" }
    if ($rc -ne 0) { throw "memdoor $cmd exit $rc" }
    return $out
}

function Expect-Reply {
    $out = Memdoor @('run', 'say hello')
    if (-not $out.Contains($reply)) { throw 'memdoor run: no reply from the stand-in' }
}

$env:STANDIN_KEY = $key
$standin = Start-Process -FilePath $env:STANDIN_BIN -ArgumentList '-port', $standinPort -PassThru -NoNewWindow `
    -RedirectStandardError (Join-Path $work 'standin.log') -RedirectStandardOutput (Join-Path $work 'standin.out')
Remove-Item Env:STANDIN_KEY

$saved = @{ HOME = $env:HOME; USERPROFILE = $env:USERPROFILE }
try {
    foreach ($i in 1..20) {
        try { Invoke-RestMethod -Headers @{ Authorization = "Bearer $key" } "$standinUrl/models" | Out-Null; break } catch { Start-Sleep -Milliseconds 500 }
    }
    if (Gateway-Pid) { throw "a gateway already answers on :$port; pick another PORT" }
    $env:HOME = $homeDir; $env:USERPROFILE = $homeDir
    $env:MEMDOOR_OWNER = '1'; $env:MEMDOOR_NO_BROWSER = '1'
    Set-Location $proj
    & $bin --version

    Write-Host '== memdoor connect deepseek, the key given'
    $out = Memdoor @('connect', 'deepseek', '--base', $standinUrl, '--key', $key)
    if ($out -notmatch 'deepseek connected') { throw 'memdoor connect did not connect' }
    Expect-Reply

    Write-Host '== the gateway stopped; memdoor run starts it again, the key read from providers.json'
    Stop-Gateway
    Expect-Reply
    $out = Memdoor @('providers')
    if ($out -notmatch '(?m)^\s+deepseek .* connected \(') { throw 'deepseek is not connected in memdoor providers after the restart' }

    Write-Host "== the stand-in's requests"
    $log = Get-Content (Join-Path $work 'standin.log') | Where-Object { $_ -match ' key=' }
    $log | ForEach-Object { $_ -replace '^\S+ \S+ ', '' } | Group-Object | ForEach-Object { "{0,4} {1}" -f $_.Count, $_.Name }
    if ($log -match 'key=refused') { throw 'a request reached the stand-in with a wrong key' }
    Write-Host 'first session: connected, answered, and the key kept over a restart'
}
catch {
    Write-Host "FAIL: $_"
    Write-Host '--- the stand-in''s log'; Get-Content (Join-Path $work 'standin.log') -Tail 20 -ErrorAction SilentlyContinue
    Write-Host '--- the gateway''s log'; Get-Content (Join-Path $homeDir '.memdoor\gateway.log') -Tail 20 -ErrorAction SilentlyContinue
    throw
}
finally {
    Stop-Gateway
    Stop-Process -Id $standin.Id -Force -ErrorAction SilentlyContinue
    $env:HOME = $saved.HOME; $env:USERPROFILE = $saved.USERPROFILE
}
