#!/bin/sh
# Memdoor CLI installer.
# Usage: curl -fsSL https://memdoor.ai/install.sh | bash
#
# Frictionless install path:
#   1. Download the right memdoor binary for OS+arch.
#   2. Print one next-step line — `memdoor setup` — that does
#      everything else (start gateway + register admin + pick an LLM
#      provider).
#
# Net frictionless flow from "I heard about this" to "the coder
# editing my code":
#
#   curl -fsSL https://memdoor.ai/install.sh | bash   # drops the binary
#   memdoor setup                                      # one prompt-driven onboarding
#   memdoor connect                                    # paste a provider key
#   cd <project> && memdoor workspace use <slug> && memdoor tui
#
# Two commands and the window. No second package manager invocation.

set -eu

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
# macOS `uname -m` returns `arm64` on Apple Silicon and `x86_64` on
# Intel Macs — normalize the latter to the binary suffix we publish.
# Linux `uname -m` returns `aarch64` on ARM64 (Debian, Ubuntu, Alpine) —
# normalize it, along with macOS Intel's `x86_64`, to the binary suffix
# we publish.
[ "$ARCH" = "x86_64" ] && ARCH=amd64
[ "$ARCH" = "aarch64" ] && ARCH=arm64

# Supported platforms: every binary deploy publishes (scripts/deploy.sh).
case "${OS}-${ARCH}" in
    darwin-arm64|darwin-amd64|linux-amd64|linux-arm64) ;;
    *)
        echo "Unsupported platform: ${OS}-${ARCH}." >&2
        echo "Supported: macOS (arm64, amd64), Linux (amd64, arm64)." >&2
        exit 1
        ;;
esac

URL="https://memdoor.ai/dl/memdoor-${OS}-${ARCH}"

# Two install modes:
#   1. system-wide (default): /usr/local/bin/memdoor, requires sudo.
#      Opt-in only via --system. Best for shared machines where every
#      user should run the same binary; requires interactive sudo or
#      pre-authenticated sudo (NOPASSWD) — piped curl|bash with
#      --system will fail with "sudo: a terminal is required to read
#      the password."
#   2. user-local: ~/.local/bin/memdoor, NO sudo. **DEFAULT** as of
#      2026-05-29. Works on every host without privilege escalation;
#      survives MDM/EDR alerts on corporate Macs; works in piped
#      curl|bash with no TTY. The 2026-05-29 dogfood test surfaced
#      that the previous "default to system, sudo prompt" path broke
#      the canonical one-liner pitch on fresh machines.
#
# Order of precedence (first match wins):
#   MEMDOOR_PREFIX env       — explicit override (e.g. /opt/memdoor/bin)
#   --system flag            — force /usr/local/bin (requires sudo)
#   --user / -u flag         — explicit ~/.local/bin (redundant with default;
#                              kept for backward compat and as a self-
#                              documenting form in scripts)
#   existing memdoor on PATH — upgrade-in-place at the SAME location
#                              (so a previous system install doesn't get
#                              silently re-homed to ~/.local/bin on
#                              upgrade). Only honored when that dir is
#                              user-writable; root-owned paths fall
#                              through to the default.
#   default                  — ~/.local/bin (no sudo, always succeeds)
USER_BIN="$HOME/.local/bin"
EXISTING=$(command -v memdoor 2>/dev/null || true)

# Record whether a real data dir existed BEFORE this run, so the
# next-step message can distinguish first install from upgrade.
HAD_DATA_DIR=0
[ -d "$HOME/.memdoor" ] && HAD_DATA_DIR=1
if [ -n "${MEMDOOR_PREFIX:-}" ]; then
    INSTALL_MODE="prefix"
elif [ "${1:-}" = "--system" ]; then
    INSTALL_MODE="system"
elif [ "${1:-}" = "--user" ] || [ "${1:-}" = "-u" ]; then
    INSTALL_MODE="user"
elif [ -n "$EXISTING" ] && [ -w "$(dirname "$EXISTING")" ]; then
    INSTALL_MODE="existing"
    EXISTING_DIR=$(dirname "$EXISTING")
    echo "==> Existing memdoor found at $EXISTING — upgrading in place"
else
    INSTALL_MODE="user"
fi

case "$INSTALL_MODE" in
    user)
        DEST_DIR="$USER_BIN"
        ;;
    prefix)
        DEST_DIR="$MEMDOOR_PREFIX"
        ;;
    existing)
        DEST_DIR="$EXISTING_DIR"
        ;;
    system|*)
        DEST_DIR="/usr/local/bin"
        ;;
esac
DEST="$DEST_DIR/memdoor"

# Upgrade-in-place: if a gateway is running on :18789, stop it before
# we swap the binary so the new binary takes over cleanly. Independent
# of whether $DEST already has a binary — the user may have deleted
# the on-disk binary while the old process kept running, or be
# reinstalling from a different mode (--user vs sudo) so the old
# binary lives at a different path.
[ -x "$DEST" ] && echo "==> Existing install detected at $DEST"
# The gateway's port: 18789 unless the person runs it elsewhere (MEMDOOR_PORT).
GW_PORT="${MEMDOOR_PORT:-18789}"
# A running gateway is one that answers on the port — never a guess from lsof:
# BusyBox's lsof (Alpine) ignores its arguments, said "running" on an empty
# box and the kill that followed hit the installer itself (2026-10-06). The
# processes are found by name, which pgrep has everywhere; lsof is the fallback.
gw_pids() {
    pgrep -f "memdoor gateway" 2>/dev/null || lsof -ti:"$GW_PORT" -sTCP:LISTEN 2>/dev/null || true
}
WAS_RUNNING=0
if curl -sf --max-time 3 "http://127.0.0.1:$GW_PORT/api/status" >/dev/null 2>&1; then
    echo "==> Gateway running on :$GW_PORT — stopping it so the new binary can take over..."
    pids=$(gw_pids)
    [ -n "$pids" ] && kill -TERM $pids 2>/dev/null || true
    sleep 3
    pids=$(gw_pids)
    [ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
    echo "  ✓ stopped gateway"
    WAS_RUNNING=1
fi

echo "==> Downloading $URL"
# macOS binary distribution has two failure modes we have to defeat
# at install time. Both bit a 64 GB M-series Mac 2026-05-28:
# post-upgrade memdoor was SIGKILLed before it could print --version,
# and recovery required a manual `codesign --force --sign -` of the
# new binary.
#
#   1. Per-inode signature cache. `curl -o $DEST` (when $DEST already
#      exists) truncates in place and preserves the inode. macOS caches
#      the code-signing identity per-inode; when the bytes change but
#      the inode doesn't, the kernel keeps the OLD signature and the
#      new bytes get rejected. Fix: download to a temp file then mv-f
#      over $DEST so the destination path points at a NEW inode (mv on
#      same-volume APFS is atomic + creates a fresh inode).
#
#   2. Unsigned-binary refusal on hardened macOS. On recent macOS
#      versions (Sequoia and forward), Gatekeeper / hardened runtime
#      can refuse to load completely unsigned binaries that link
#      certain frameworks, regardless of inode freshness. The fix is
#      ad-hoc signing: `codesign --force --sign -` assigns an empty
#      signing identity that the kernel accepts as "no Developer ID,
#      but at least there's a sealed signature." No paid Apple Dev
#      account required.
#
# Both happen in the temp-file stage so $DEST is never the broken
# binary even momentarily. xattr -c clears any auto-added quarantine
# bit curl may stamp on.
# Integrity verification (supply-chain defense). The binary is fetched
# over plain HTTPS from a static dir; a compromise of /var/www/memdoor.ai/dl/
# or a TLS downgrade would otherwise be a silent RCE on every installer.
# We pin the download to a SHA256SUMS manifest published alongside the
# binaries (deploy.sh step 4.555). FAIL CLOSED: if the manifest can't be
# fetched, or the expected hash is missing, or the computed hash doesn't
# match, we refuse to install. No silent skip.
#
# macOS ships `shasum` (Perl) in /usr/bin; Linux ships `sha256sum` (coreutils).
SUMS_URL="https://memdoor.ai/dl/SHA256SUMS"
sha256_of() {
    # $1 = file path; prints the lowercase hex digest only.
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'      # Linux
    else
        shasum -a 256 "$1" | awk '{print $1}'  # macOS
    fi
}
verify_download() {
    # $1 = path to the downloaded binary. Aborts the script on any failure.
    _file="$1"
    _name="memdoor-${OS}-${ARCH}"
    echo "==> Verifying integrity against $SUMS_URL"
    _sums=$(curl -fsSL "$SUMS_URL" 2>/dev/null) || {
        echo "ERROR: could not fetch SHA256SUMS from $SUMS_URL." >&2
        echo "       Refusing to install an unverified binary (fail-closed)." >&2
        exit 1
    }
    # Pull the expected hash for our exact filename. The manifest lines
    # are `<hash>  <name>`; match on whitespace + end-of-line so e.g.
    # 'memdoor-darwin-amd64' can't accidentally match a longer name.
    _expected=$(printf '%s\n' "$_sums" \
        | grep -E "[[:space:]]${_name}\$" \
        | awk '{print $1}' | head -n1)
    if [ -z "$_expected" ]; then
        echo "ERROR: SHA256SUMS has no entry for ${_name}." >&2
        echo "       Refusing to install an unverified binary (fail-closed)." >&2
        exit 1
    fi
    _actual=$(sha256_of "$_file") || exit 1
    if [ "$_expected" != "$_actual" ]; then
        echo "ERROR: checksum mismatch for ${_name}!" >&2
        echo "       expected: $_expected" >&2
        echo "       actual:   $_actual" >&2
        echo "       The download may be corrupted or tampered with. Aborting." >&2
        exit 1
    fi
    echo "  ✓ sha256 verified ($_actual)"
}

TMP_DEST=$(mktemp /tmp/memdoor.XXXXXX)
case "$INSTALL_MODE" in
    user|prefix|existing)
        # No sudo — write to a user-writable directory.
        mkdir -p "$DEST_DIR"
        curl -fL --progress-bar "$URL" -o "$TMP_DEST"
        verify_download "$TMP_DEST"
        chmod +x "$TMP_DEST"
        xattr -c "$TMP_DEST" 2>/dev/null || true
        codesign --force --sign - "$TMP_DEST" 2>/dev/null || true
        mv -f "$TMP_DEST" "$DEST"
        ;;
    system|*)
        # Download as the regular user (curl doesn't need root), sudo
        # only the mv into the root-owned dir. Avoids leaving a temp
        # file owned by root in /tmp if the install fails mid-way.
        curl -fL --progress-bar "$URL" -o "$TMP_DEST"
        verify_download "$TMP_DEST"
        chmod +x "$TMP_DEST"
        xattr -c "$TMP_DEST" 2>/dev/null || true
        codesign --force --sign - "$TMP_DEST" 2>/dev/null || true
        sudo mv -f "$TMP_DEST" "$DEST"
        ;;
esac

if [ "${WAS_RUNNING:-0}" = "1" ]; then
    echo "✓ Upgraded memdoor → $DEST"
else
    echo "✓ Installed memdoor → $DEST"
fi

# PATH hint for user-mode installs. Skip if $DEST_DIR is already on
# PATH (most macOS users picked up ~/.local/bin via pipx, pyenv, asdf,
# or a manual rc entry).
case "$INSTALL_MODE" in
    user|prefix|existing)
        case ":$PATH:" in
            *":$DEST_DIR:"*)
                ;;
            *)
                echo
                echo "⚠ $DEST_DIR is not on your PATH. Add this to your shell rc:"
                echo "    export PATH=\"$DEST_DIR:\$PATH\""
                ;;
        esac
        ;;
esac

echo
echo "✓ Memdoor is installed."
echo

# WHAT COMES NEXT IS THE COMMAND LINE (onboarding battle test, 2026-09-27).
# The path is "export your provider key and go"; this block used to point at
# surfaces and downloads that belonged to an earlier product.
#
# The gateway this script stopped is started again on the new binary (an
# upgrade that left it down cost a working session on 2026-10-06): in the
# background, its log beside the data, and reported either way. It starts
# with this shell's environment — keys kept by `memdoor connect` are on disk
# and come back; a key that only lived in the old gateway's shell does not.
if [ "${WAS_RUNNING:-0}" = "1" ]; then
    mkdir -p "$HOME/.memdoor"
    nohup "$DEST" gateway --port "$GW_PORT" >> "$HOME/.memdoor/gateway.log" 2>&1 &
    i=0
    while [ $i -lt 20 ]; do
        if curl -sf "http://127.0.0.1:$GW_PORT/api/status" >/dev/null 2>&1; then break; fi
        sleep 1; i=$((i+1))
    done
    if curl -sf "http://127.0.0.1:$GW_PORT/api/status" >/dev/null 2>&1; then
        echo "✓ Gateway restarted on :$GW_PORT with the new binary (log: ~/.memdoor/gateway.log)."
        WAS_RUNNING=0
    else
        echo "⚠ The gateway did not come back on :$GW_PORT — see ~/.memdoor/gateway.log."
    fi
    echo
fi
echo "Next:"
if [ "${WAS_RUNNING:-0}" = "1" ]; then
    echo "  memdoor gateway &                      # restart it (this upgrade stopped it and could not restart it)"
fi
if [ "$HAD_DATA_DIR" != "1" ]; then
    echo "  memdoor setup                          # once on this machine"
fi
echo "  memdoor connect                        # your provider's key (or export OPEN_ROUTER_API_KEY=sk-or-...)"
echo "  cd <your-project> && memdoor tui       # start working"
echo
echo "Pro (remote control)? memdoor login you@example.com"
