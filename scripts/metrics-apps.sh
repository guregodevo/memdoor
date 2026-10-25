#!/bin/sh
# scripts/metrics-apps.sh — other people's use of Memdoor, from the providers'
# own app statistics (pkg/attribution: the headers that name us to each one).
#
# OpenRouter: every request Memdoor sends carries HTTP-Referer https://memdoor.ai,
# and OpenRouter sums them on a public page: ALL-TIME totals per model (the page
# says "this month"; the daily chart embedded in it sums to the same figure, so it
# is everything since the first request, 2026-09-27), and that daily chart, per
# model per UTC day. This machine's own share is in its ledger (memdoor meter,
# read in UTC to match). What is left on a day is other people on their own key.
#
# Each run rewrites .memdoor/metrics/openrouter.tsv (date, model, openrouter,
# this_machine, others; one line per model per UTC day) from the chart, so the
# series is complete whenever it runs; the day in progress is refreshed next run.
# The launch (Show HN) was 2026-10-21: days before it include Memdoor's own
# machines of the time, days after it are strangers.
#
# The other gateways that list apps publicly are checked for a Memdoor row:
# LLM Gateway (llmgateway.io/apps, by X-Source memdoor.ai), EUrouter
# (eurouter.ai/rankings, by HTTP-Referer) and Vercel AI Gateway
# (vercel.com/ai-gateway/leaderboards, opted-in apps only, past week). Requesty
# publishes no app page. Vendor keys (Anthropic, OpenAI, DeepSeek, …) see only a
# User-Agent and publish nothing.
#
# Usage: make metrics (calls this), or ./scripts/metrics-apps.sh [days]
set -e
cd "$(dirname "$0")/.."
mkdir -p .memdoor/metrics
meter=./memdoor; [ -x "$meter" ] || meter=memdoor
echo "==> OpenRouter app page (https://openrouter.ai/apps?url=https%3A%2F%2Fmemdoor.ai), all time since 2026-09-27, UTC days"
curl -s -A "Mozilla/5.0" "https://openrouter.ai/apps?url=https%3A%2F%2Fmemdoor.ai" \
| METER_CMD="$meter" DAYS="${1:-14}" TZ=UTC python3 -c '
import collections, os, re, subprocess, sys
LAUNCH = "2026-10-21"
page = sys.stdin.read().replace("\\\"", "\"")
total = re.search(r"\"totalTokens\":(\d+)", page)
used = re.search(r"\"modelsUsed\":(\d+)", page)
def base(slug):
    return re.sub(r"(-\d{8}|:free)$", "", slug.split("/")[-1].lower())
days = collections.OrderedDict()
for m in re.finditer(r"\{\"x\":\"(2026-\d\d-\d\d) 00:00:00\",\"ys\":\{([^}]*)\}\}", page):
    if m.group(1) in days: continue
    ys = {}
    for k, v in re.findall(r"\"([^\"]+)\":(\d+)", m.group(2)):
        ys[base(k)] = ys.get(base(k), 0) + int(v)
    days[m.group(1)] = ys
if not total or not days:
    print("page unreadable (layout changed, or no attribution yet)"); sys.exit(0)
mine = collections.defaultdict(lambda: collections.defaultdict(int))
try:
    out = subprocess.run([os.environ["METER_CMD"], "meter", "--tail", "1000000"], capture_output=True, text=True, timeout=60).stdout
    for line in out.splitlines():
        f = line.split()
        if len(f) >= 8 and "openrouter" in f[2] and f[4].isdigit():
            mine["2026-" + f[0]][base(f[3])] += int(f[4]) + int(f[6])
except Exception as e:
    print("this machine\x27s ledger unavailable:", e)
print("tokens %sM over %s models, all time" % (int(total.group(1)) // 1_000_000, used.group(1) if used else "?"))
rows, pre, post = [], 0, 0
perday = collections.OrderedDict()
for day, ys in days.items():
    oth_models = {}
    for model, t in ys.items():
        me = mine[day].get(model, 0)
        gap = 0 if model.startswith("jev") else max(t - me, 0)
        rows.append("%s\t%s\t%d\t%d\t%d" % (day, model, t, me, gap))
        if gap > 20000: oth_models[model] = gap
    o = sum(ys.values()); me = sum(mine[day].values()); oth = sum(oth_models.values())
    perday[day] = (o, me, oth, oth_models)
    if day >= LAUNCH: post += oth
    else: pre += oth
with open(".memdoor/metrics/openrouter.tsv", "w") as f:
    f.write("date\tmodel\topenrouter\tthis_machine\tothers\n" + "\n".join(rows) + "\n")
print("others: %dK tokens since the launch (%s), %dK before it (Memdoor\x27s own machines of the time included)" % (post // 1000, LAUNCH, pre // 1000))
print()
print("%-10s %10s %12s %10s  %s" % ("UTC DAY", "OPENROUTER", "THIS MACHINE", "OTHERS", "others\x27 models (decision model rows left out)"))
for day in list(perday)[-int(os.environ["DAYS"]):]:
    o, me, oth, models = perday[day]
    print("%-10s %9dK %11dK %9dK  %s" % (day, o // 1000, me // 1000, oth // 1000, ", ".join("%s %dK" % (k, v // 1000) for k, v in sorted(models.items(), key=lambda kv: -kv[1]))))
print("a day missing from the chart had no request; the day in progress grows until midnight UTC")
'

echo
echo "==> Other gateways with a public app list (a row appears once someone sends through them with our headers)"
for u in https://llmgateway.io/apps https://www.eurouter.ai/rankings https://vercel.com/ai-gateway/leaderboards; do
    page=$(curl -s -L -A "Mozilla/5.0" --max-time 30 "$u" || true)
    if [ -z "$page" ]; then
        echo "  $u: unreachable"
    elif printf '%s' "$page" | grep -qi memdoor; then
        echo "  $u: LISTED"
        printf '%s' "$page" | tr '<' '\n' | grep -i -A6 memdoor | grep -ioE 'memdoor|[0-9.,]+ ?[KMBT]? ?(tokens|requests)' | sort -u | head -6 | sed 's/^/      /'
    else
        echo "  $u: not listed yet"
    fi
done
