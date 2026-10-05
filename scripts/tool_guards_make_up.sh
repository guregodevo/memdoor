#!/bin/bash
# Prints the tool_guards workspace setting that stops an agent restarting or
# stopping the gateway it runs on: the restart kills its own turn mid-flight
# (2026-09-30: the coder ran `make up` twice and lost both turns). Every
# Makefile target that stops the gateway is covered — up, start, stop,
# stop-gateway, gateway, gateway-verbose — plus killing it by hand.
# `make build` stays allowed: it no longer stops the gateway.
#
#   TOKEN=$(memdoor auth token)
#   scripts/tool_guards_make_up.sh | curl -X PUT http://localhost:18789/api/workspace/settings \
#     -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d @-
python3 - <<'PY'
import json
restart = "restarting or stopping the gateway is the user's move: it runs this turn, which would die mid-flight. Verify with `go build ./...`, `go test ./...` or `make build` (binary only), and tell the user to run `make up`"
rules = [
  {"tool": "bash", "pattern": r"(^|[\";&|(]\s*|\\n\s*|\\u0026\s*)make(\s+(-C\s+[^\s|;&\"\\]+|-[^C\s|;&\"\\][^\s|;&\"\\]*|[^-\s|;&\"\\][^\s|;&\"\\]*))*\s+(up|start|stop|stop-gateway|gateway|gateway-verbose)([\s|;&)\"\\]|$)", "message": restart},
  {"tool": "bash", "pattern": r"\b(pkill|killall)\b[^|;&]*\bmemdoor\b", "message": "never kill memdoor processes: that kills the user's TUI session and this turn"},
  {"tool": "bash", "pattern": r"\blsof\b[^|;&]*:18789\b.*\bkill\b", "message": restart},
]
print(json.dumps({"tool_guards": json.dumps(rules)}))
PY
