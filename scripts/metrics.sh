#!/bin/sh
# scripts/metrics.sh — privacy-clean adoption metrics, server-side only.
#
# No client telemetry, no phone-home, nothing added to the binary. These
# are counts of PUBLIC file downloads already recorded in the prod nginx
# access log, so they cost the "nothing leaves your machine" promise
# exactly nothing — the only thing that "phones home" is a browser/curl
# fetching a public download, which is unavoidable for any download.
#
# What it reports:
#   - install.sh one-liner runs      = people who ran `curl … | bash`
#   - distinct IPs on install.sh     = rough distinct-install count
#   - /dl/VERSION hits               = `memdoor upgrade` checks ≈ engaged installs
#   - /dl/memdoor-* downloads        = binaries pulled, broken out by platform
#   - runs by day                    = the adoption trend line
#
# For richer (opt-in) signal, the gateway already ships pkg/telemetry
# (MEMDOOR_TELEMETRY_ENABLED, default OFF). This script is the zero-cost,
# zero-privacy-tradeoff baseline that needs no client cooperation.
#
# Usage: ./scripts/metrics.sh [VPS_HOST]   (VPS_HOST falls back to .envrc)
set -eu

[ -f .envrc ] && . ./.envrc
VPS_HOST="${1:-${VPS_HOST:-}}"
if [ -z "${VPS_HOST:-}" ]; then
    echo "Set VPS_HOST in .envrc or pass it: ./scripts/metrics.sh root@host" >&2
    exit 2
fi

ssh "$VPS_HOST" 'sh -s' <<"REMOTE"
set -eu
catlog() {
    for f in /var/log/nginx/access.log /var/log/nginx/access.log.1; do
        [ -f "$f" ] && cat "$f"
    done
    for g in /var/log/nginx/access.log.*.gz; do
        [ -f "$g" ] && zcat "$g" 2>/dev/null
    done
}

echo "==> Memdoor adoption — from nginx access logs (server-side, no client telemetry)"
echo
echo "install.sh one-liner runs:    $(catlog | grep -c 'GET /install\.sh' || true)"
echo "distinct IPs (install.sh):    $(catlog | grep 'GET /install\.sh' | awk '{print $1}' | sort -u | wc -l | tr -d ' ')"
echo "version checks (/dl/VERSION):  $(catlog | grep 'GET /dl/VERSION' | awk '{print $1}' | sort -u | wc -l | tr -d ' ') distinct IPs (≈ engaged installs)"
echo
echo "binary downloads by platform:"
catlog | grep -oE 'GET /dl/memdoor-[a-z0-9_-]+' | sed 's#GET /dl/##' | sort | uniq -c | sort -rn || true
echo
echo "install.sh runs by day (last 14 with traffic):"
catlog | grep 'GET /install\.sh' | grep -oE '\[[0-9]{2}/[A-Za-z]+/[0-9]{4}' | tr -d '[' | sort | uniq -c | tail -14 || true
REMOTE
