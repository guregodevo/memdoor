#!/usr/bin/env bash
# Tell the engines that share IndexNow (Bing, DuckDuckGo, Yandex, Seznam, Naver,
# Yep) which pages changed, so they recrawl now rather than on their own clock.
# The URL list is the sitemap's; the key is the public one served at the site
# root (web/public/<key>.txt, also .memdoor/indexnow-key). One POST, every URL.
# Google ignores IndexNow: its only lever is Search Console (sitemap resubmit,
# URL inspection → request indexing).
set -euo pipefail
cd "$(dirname "$0")/.."
DOMAIN="${DOMAIN:-memdoor.ai}"
KEY="$(cat .memdoor/indexnow-key)"
URLS=$(grep -o '<loc>[^<]*' web/public/sitemap.xml | sed 's/<loc>//' | sed 's/^/        "/; s/$/",/' | sed '$ s/,$//')
BODY=$(cat <<JSON
{
    "host": "$DOMAIN",
    "key": "$KEY",
    "keyLocation": "https://$DOMAIN/$KEY.txt",
    "urlList": [
$URLS
    ]
}
JSON
)
COUNT=$(grep -c '<loc>' web/public/sitemap.xml)
CODE=$(curl -s -o /tmp/indexnow.out -w '%{http_code}' -X POST "https://api.indexnow.org/IndexNow" \
    -H 'Content-Type: application/json; charset=utf-8' --data "$BODY")
case "$CODE" in
    200|202) echo "==> IndexNow accepted $COUNT URLs for $DOMAIN (HTTP $CODE)" ;;
    *) echo "==> IndexNow refused (HTTP $CODE): $(cat /tmp/indexnow.out)"; exit 1 ;;
esac
