#!/bin/bash
# Point the subscription checkout at a different Stripe price.
#
# Greg, 2026-09-27: "update the stripe price to $10". The plan of record is Pro
# at $10/month for the decision model (docs/roadmap/MUST.md), while Stripe still
# holds the $149 seat, so nothing in the binary or on the site may quote $10
# until this has run.
#
# WHY THIS IS A SCRIPT AND NOT A COMMAND: it edits a production secrets file and
# restarts the money service, which this repo does only through something a
# person can read first (AGENTS.md).
#
# WHAT IT CANNOT DO. A Stripe price is immutable — an amount is never edited,
# a new price is created and pointed at. The production key is an organization
# key restricted to what checkout needs: products read is allowed, prices read
# and write are NOT (probed 2026-09-27: more_permissions_required on both). So
# the price itself is created by a human, in the dashboard or with a key that
# has plan_write, on the existing product:
#
#     product  prod_VHW7BU6q64bOwo   "Memdoor seat"   (live)
#     price    recurring, monthly, USD 10.00
#
# Then run this with the new price id:
#
#     ./scripts/set-seat-price.sh price_XXXXXXXXXXXX
#
# It backs up /etc/memdoor/billing.env with a dated copy, sets
# STRIPE_PRO_MONTHLY_PRICE, restarts the three billing units one at a time,
# refuses to carry on if billing does not answer, and then VERIFIES by
# creating a real checkout session and reading the amount Stripe quotes back.
#
# EXISTING SUBSCRIBERS KEEP THE PRICE THEY SIGNED AT. This changes what a NEW
# checkout charges. Moving somebody from $149 to $10 is a subscription update in
# Stripe, per customer, and is deliberately not automated here.
#
# Rollback: the script prints the previous price id and the backup file; run it
# again with that id.
set -euo pipefail
cd "$(dirname "$0")/.."

NEW_PRICE="${1:-}"
if [ -z "$NEW_PRICE" ]; then
    echo "Usage: $0 price_XXXXXXXXXXXX" >&2
    echo "  Create it first: a recurring monthly USD price on prod_VHW7BU6q64bOwo (\"Memdoor seat\")." >&2
    exit 2
fi
case "$NEW_PRICE" in
    price_*) ;;
    *) echo "That is not a Stripe price id (they start with price_): $NEW_PRICE" >&2; exit 2 ;;
esac

# eval, not source <(...): macOS's bash 3.2 reads nothing from a process substitution
eval "$(grep -E '^(export )?VPS_HOST=' .envrc)"
: "${VPS_HOST:?VPS_HOST missing from .envrc}"

echo "==> 1. current value, and a dated backup"
ssh "$VPS_HOST" "set -e
F=/etc/memdoor/billing.env
OLD=\$(grep '^STRIPE_PRO_MONTHLY_PRICE=' \$F | cut -d= -f2-)
echo \"    was: \${OLD:-<unset>}\"
if [ \"\$OLD\" = '$NEW_PRICE' ]; then echo '    already this price — nothing to change'; exit 3; fi
B=\$F.bak-\$(date +%Y%m%d%H%M%S)
cp -p \$F \$B
echo \"    backup: \$B\"
echo \"    rollback: ./scripts/set-seat-price.sh \${OLD:-<none>}\""

echo "==> 2. set STRIPE_PRO_MONTHLY_PRICE"
ssh "$VPS_HOST" "set -e
F=/etc/memdoor/billing.env
if grep -q '^STRIPE_PRO_MONTHLY_PRICE=' \$F; then
  # A temp file in the same directory, then mv: never a half-written secrets file.
  awk -v p='$NEW_PRICE' '/^STRIPE_PRO_MONTHLY_PRICE=/ {print \"STRIPE_PRO_MONTHLY_PRICE=\" p; next} {print}' \$F > \$F.new
else
  cp \$F \$F.new && echo 'STRIPE_PRO_MONTHLY_PRICE=$NEW_PRICE' >> \$F.new
fi
chmod --reference=\$F \$F.new 2>/dev/null || chmod 600 \$F.new
mv \$F.new \$F
echo \"    now: \$(grep '^STRIPE_PRO_MONTHLY_PRICE=' \$F | cut -d= -f2-)\""

echo "==> 3. restart billing"
ssh "$VPS_HOST" "set -e
systemctl restart memdoor-billing
for n in \$(seq 1 15); do
  if curl -sf -m 2 http://127.0.0.1:18790/v1/health >/dev/null; then echo '    billing restarted'; exit 0; fi
  sleep 2
done
echo '    billing did not answer — restore the backup from step 1 and restart.'; exit 1"

echo "==> 4. verify: what does a real checkout quote?"
# This creates an abandoned checkout session. It is the only end-to-end proof
# that the new price is the one a customer would be charged.
ssh "$VPS_HOST" 'set -e
source /etc/memdoor/billing.env
S=$(curl -s -m 30 -X POST https://api.stripe.com/v1/checkout/sessions \
  -H "Authorization: Bearer $STRIPE_SECRET_KEY" \
  ${STRIPE_CONTEXT:+-H "Stripe-Context: $STRIPE_CONTEXT"} \
  -H "Stripe-Version: 2025-03-31.basil" \
  -d mode=subscription \
  -d "line_items[0][price]=$STRIPE_PRO_MONTHLY_PRICE" \
  -d "line_items[0][quantity]=1" \
  -d success_url=https://memdoor.ai/pro/success \
  -d cancel_url=https://memdoor.ai/pro/cancelled)
echo "$S" | python3 -c "
import json,sys
d=json.load(sys.stdin)
e=d.get(\"error\")
if e:
    print(\"    checkout REFUSED:\", e.get(\"message\"))
    sys.exit(1)
cents=d.get(\"amount_total\")
print(\"    a new checkout quotes:\", (\"$%.2f\" % (cents/100)) if cents is not None else \"(amount not returned)\", d.get(\"currency\",\"\"))
print(\"    session:\", d[\"id\"], \"(abandoned; no payment)\")
"'

echo "==> done. Existing subscribers keep the price they signed at."
