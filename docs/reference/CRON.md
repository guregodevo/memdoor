# Cron Jobs Reference

**Version**: 1.0
**Date**: 2026-04-18

**Automated task scheduling for Memdoor agents**

Memdoor's cron system enables scheduled execution of AI agent tasks, from daily health checks to weekly reports. Jobs run in dedicated agent contexts with full conversation history, tool access, and execution tracking.

---

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Cron Expression Syntax](#cron-expression-syntax)
- [CLI Commands](#cli-commands)
- [Configuration](#configuration)
- [Execution Model](#execution-model)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

---

## Overview

### Features

- **Flexible scheduling** - standard 5-field crontab, or 6-field with an optional leading seconds field for sub-minute precision
- **Per-agent execution** - Jobs run in specific agent contexts with full tool access
- **Persistent storage** - Jobs stored in JSON, survive gateway restarts
- **Execution history** - Full tracking of runs, durations, success/failure
- **Session isolation** - Each job gets dedicated session (`agent:main:cron:<job-id>`)
- **Conversation continuity** - Jobs maintain context across executions

### Architecture

```
┌─────────────────────────────────────────┐
│         Cron Scheduler                  │
│  • robfig/cron/v3 (Go library)          │
│  • Second-level precision               │
│  • Concurrent job execution             │
└──────────────┬──────────────────────────┘
               │
      ┌────────┴────────┐
      │                 │
┌─────▼──────┐   ┌─────▼──────┐
│   Store    │   │  History   │
│ jobs.json  │   │history.json│
│ Persistent │   │  Tracking  │
└────────────┘   └────────────┘
      │                 │
      └────────┬────────┘
               │
    ┌──────────▼──────────┐
    │   Agent Executor    │
    │ • Session: cron:ID  │
    │ • Full tool access  │
    │ • 10min timeout     │
    └─────────────────────┘
```

### Storage Paths

```bash
~/.memdoor/cron/
├── jobs.json        # Job definitions (persistent)
├── history.json     # Execution history (last 1000 runs)
└── sessions/        # Per-job conversation histories
    └── cron:<job-id>.jsonl
```

---

## Quick Start

### 1. Create a Job

```bash
# Daily health check at 9 AM
memdoor cron add \
  --id "daily-health-check" \
  --schedule "0 0 9 * * *" \
  --message "Run health diagnostics: check gateway logs for errors, verify agent status, report any issues" \
  --enabled
```

### 2. Restart Gateway

```bash
# Jobs load on gateway startup
memdoor gateway restart
```

### 3. Monitor Execution

```bash
# What the schedule has done (the CLI has no live tail any more)
memdoor logs query --regex 'cron' --since 30m

# Check history
memdoor cron history daily-health-check --limit 10

# View statistics
memdoor cron stats daily-health-check
```

---

## Cron Expression Syntax

Memdoor accepts **standard 5-field crontab** and the **6-field** form with a
leading seconds field (seconds are optional). A 5-field expression means exactly
what it does in any other cron; add a seconds field only when you need
sub-minute precision. Named shorthands (`@hourly`, `@daily`, `@every 1h30m`)
work too. The 6-field form below shows every field:

```
┌───────────── second (0-59)
│ ┌───────────── minute (0-59)
│ │ ┌───────────── hour (0-23)
│ │ │ ┌───────────── day of month (1-31)
│ │ │ │ ┌───────────── month (1-12)
│ │ │ │ │ ┌───────────── day of week (0-6) (Sunday=0)
│ │ │ │ │ │
* * * * * *
```

### Special Characters

| Symbol | Meaning | Example |
|--------|---------|---------|
| `*` | Any value | `* * * * * *` = every second |
| `,` | Value list | `0 0,30 * * * *` = 0 and 30 minutes past |
| `-` | Range | `0 0 9-17 * * *` = 9 AM to 5 PM |
| `/` | Step values | `0 */15 * * * *` = every 15 minutes |

### Common Patterns

```bash
# Every minute
0 * * * * *

# Every 15 minutes
0 */15 * * * *

# Every hour at :00
0 0 * * * *

# Daily at 9 AM
0 0 9 * * *

# Weekdays at 9 AM (Monday-Friday)
0 0 9 * * 1-5

# Every Monday at midnight
0 0 0 * * 1

# First of month at noon
0 0 12 1 * *

# Every 30 seconds
*/30 * * * * *

# Twice daily (9 AM and 5 PM)
0 0 9,17 * * *
```

---

## CLI Commands

### `cron list`

List all configured cron jobs.

```bash
memdoor cron list
```

**Output:**
```
JOB ID              SCHEDULE      AGENT      ENABLED  MESSAGE
------              --------      -----      -------  -------
daily-report        0 0 9 * * *   main       yes      Generate daily summary report
health-check        0 */30 * * * * main       yes      Check system health
backup-memories     0 0 2 * * *   main       yes      Backup agent memories to Drive
```

---

### `cron add`

Add a new cron job.

```bash
memdoor cron add \
  --id <job-id> \
  --schedule <cron-expr> \
  --message <message> \
  [--agent <agent-id>] \
  [--enabled]
```

**Parameters:**

| Flag | Required | Description |
|------|----------|-------------|
| `--id` |  | Unique job identifier (alphanumeric, hyphens, underscores) |
| `--schedule` |  | Cron expression: 5-field (min hour day month weekday) or 6-field with leading seconds |
| `--message` |  | Message sent to agent on execution |
| `--agent` |  | Agent ID (default: main agent) |
| `--enabled` |  | Enable immediately (default: true) |

**Example:**
```bash
memdoor cron add \
  --id "weekly-summary" \
  --schedule "0 0 9 * * 1" \
  --message "Generate weekly summary: analyze last 7 days of logs, create report, email to team" \
  --enabled
```

---

### `cron remove`

Remove a cron job.

```bash
memdoor cron remove <job-id>
```

**Example:**
```bash
memdoor cron remove weekly-summary
```

---

### `cron history`

Show execution history for a job (or all jobs).

```bash
# Specific job
memdoor cron history <job-id> [--limit N]

# All jobs
memdoor cron history [--limit N]
```

**Output:**
```
TIME                 DURATION  STATUS   ERROR
----                 --------  ------   -----
2026-02-27 09:00:06  4521ms    SUCCESS
2026-02-26 09:00:08  4832ms    SUCCESS
2026-02-25 09:00:05  5102ms    SUCCESS
```

---

### `cron stats`

Show execution statistics for a job (or all jobs).

```bash
# Specific job
memdoor cron stats <job-id>

# All jobs
memdoor cron stats
```

**Output:**
```
JOB ID         TOTAL  SUCCESS  FAILED  SUCCESS RATE  AVG DURATION  LAST RUN
------         -----  -------  ------  ------------  ------------  --------
daily-report   30     30       0       100.0%        4835ms        3 hours ago
health-check   720    718      2       99.7%         1254ms        5 minutes ago
```

---

## Configuration

### YAML Configuration (Legacy)

You can define cron jobs in `~/.memdoor/config.yaml`:

```yaml
cron:
  enabled: true
  store: ~/.memdoor/cron/jobs.json  # Persistent store path
  jobs:
    - id: morning-briefing
      schedule: "0 0 9 * * 1-5"  # Weekdays at 9 AM
      agent_id: main
      message: "Generate morning briefing: check calendar, unread emails, pending tasks"
      enabled: true

    - id: nightly-backup
      schedule: "0 0 2 * * *"  # Daily at 2 AM
      agent_id: main
      message: "Backup agent memories and session histories to Google Drive"
      enabled: true
```

**Note:** CLI-added jobs (via `cron add`) take precedence over config-defined jobs.

### Runtime Configuration (Recommended)

Use CLI commands for dynamic job management:
-  **Persistent** - Jobs survive gateway restarts
-  **No manual editing** - YAML parsing errors avoided
-  **Immediate validation** - Cron expression validated on add

```bash
# Add jobs at runtime
memdoor cron add --id "job-1" --schedule "0 0 9 * * *" --message "..."
memdoor cron add --id "job-2" --schedule "0 */30 * * * *" --message "..."
```

---

## Execution Model

### Session Isolation

Each cron job runs in a dedicated session:

```
Session Key: agent:<agent-id>:cron:<job-id>
Example: agent:main:cron:daily-report
```

**Implications:**
-  Full conversation history preserved across executions
-  Job can reference previous runs ("Compare today's metrics to yesterday")
-  Agent memory persists (tools, preferences learned)
-  Context grows over time (auto-compaction at 60%/90%)

### Execution Flow

```
1. Scheduler triggers at scheduled time
   └─> Cron: "0 0 9 * * *" fires at 09:00:00

2. Job wrapper created
   └─> Session: agent:main:cron:daily-report
   └─> Message: "Generate daily summary report"

3. Agent executor invoked
   └─> Load conversation history (JSONL)
   └─> Append user message
   └─> Call the configured engine (the provider on the person's key) with full tool access
   └─> Execute tool calls (read_file, web_search, etc.)
   └─> Save response to history

4. History recorded
   └─> Start time, end time, duration
   └─> Success/failure status
   └─> Error message (if failed)
   └─> Saved to history.json

5. Next execution waits for schedule
```

### Timeout

**Default**: 10 minutes per execution

If a job exceeds 10 minutes, it's terminated with error:
```
Error: context deadline exceeded
```

**Tip:** For long-running tasks, break into smaller jobs or use subagents.

---

## Best Practices

### 1. Design Idempotent Jobs

Jobs should be safe to re-run if they fail:

```bash
#  BAD: Creates duplicate records
--message "Create new daily report in Google Docs"

#  GOOD: Checks if report exists first
--message "Create daily report in Google Docs if not exists (check for today's date in Drive)"
```

### 2. Use Descriptive Job IDs

```bash
#  BAD: Generic, hard to identify
--id "job-1"

#  GOOD: Clear purpose
--id "daily-standup-reminder"
--id "weekly-metrics-report"
--id "hourly-api-health-check"
```

### 3. Include Context in Messages

```bash
#  BAD: Vague, agent doesn't know what to do
--message "Check logs"

#  GOOD: Specific, actionable
--message "Check gateway logs for errors in last 24h, summarize by severity, create Jira ticket for criticals"
```

### 4. Start with Conservative Schedules

```bash
#  BAD: Every second (720 calls/hour, rate limit hit)
--schedule "* * * * * *"

#  GOOD: Every 15 minutes (4 calls/hour)
--schedule "0 */15 * * * *"
```

**Rate Limit Reminder:** Taskforce API has 10 req/min limit. Jobs executing every second will fail.

### 5. Monitor Execution History

```bash
# Daily check
memdoor cron stats

# After adding new job
memdoor cron history <job-id> --limit 5
```

### 6. Handle Failures Gracefully

Jobs should include error recovery in their prompt:

```bash
--message "Generate weekly report. If data missing, note the gap and use available data. Email summary even if incomplete."
```

---

## Examples

### 1. Daily Health Check

```bash
memdoor cron add \
  --id "daily-health-check" \
  --schedule "0 0 9 * * *" \
  --message "Health check: query logs for errors in last 24h, check gateway uptime, verify agent tool access, report status" \
  --enabled
```

**What it does:**
- Runs every day at 9 AM
- Queries event logs (`agent_log` tool)
- Checks gateway health
- Reports findings

---

### 2. Weekly Summary Report

```bash
memdoor cron add \
  --id "weekly-summary" \
  --schedule "0 0 9 * * 1" \
  --message "Generate weekly summary: analyze logs from last 7 days, group errors by type, calculate success rates, create Google Doc with findings, email to team" \
  --enabled
```

**What it does:**
- Runs every Monday at 9 AM
- Analyzes last week's data
- Creates formatted report
- Emails stakeholders

---

### 3. Hourly API Health Monitor

```bash
memdoor cron add \
  --id "api-health-monitor" \
  --schedule "0 0 * * * *" \
  --message "Monitor API health: check endpoint response times, verify 200 status codes, alert if latency > 500ms or errors detected" \
  --enabled
```

**What it does:**
- Runs every hour
- Checks API endpoints
- Alerts on anomalies

---

### 4. Nightly Memory Backup

```bash
memdoor cron add \
  --id "nightly-memory-backup" \
  --schedule "0 0 2 * * *" \
  --message "Backup agent memories: export memory database to JSON, upload to Google Drive folder 'Memdoor Backups', verify upload success" \
  --enabled
```

**What it does:**
- Runs daily at 2 AM
- Backs up agent memories
- Uploads to Google Drive

---

### 5. Morning Standup Reminder

```bash
memdoor cron add \
  --id "standup-reminder" \
  --schedule "0 45 8 * * 1-5" \
  --message "Morning standup reminder: check today's calendar for standup meeting, list my pending tasks, send reminder email 15 min before meeting" \
  --enabled
```

**What it does:**
- Runs weekdays at 8:45 AM
- Checks calendar
- Sends standup prep email

---

### 6. Real-Time Log Monitor (Every 5 Minutes)

```bash
memdoor cron add \
  --id "realtime-log-monitor" \
  --schedule "0 */5 * * * *" \
  --message "Monitor logs: query errors from last 5 minutes, if critical errors found, send alert email with details" \
  --enabled
```

**What it does:**
- Runs every 5 minutes
- Queries recent errors
- Alerts on critical issues

---

### 7. Monthly Metrics Report

```bash
memdoor cron add \
  --id "monthly-metrics" \
  --schedule "0 0 9 1 * *" \
  --message "Generate monthly metrics report: analyze last 30 days, calculate success rates, tool usage stats, error trends, create presentation in Google Slides, share with leadership" \
  --enabled
```

**What it does:**
- Runs first of month at 9 AM
- Full monthly analysis
- Creates executive summary

---

## Troubleshooting

### Job Not Executing

**Symptom:** Job appears in `cron list` but never runs.

**Checks:**
1. Is gateway running? `pgrep -f "memdoor gateway"`
2. Is job enabled? `memdoor cron list` (check ENABLED column)
3. Is cron enabled in config? Check `~/.memdoor/config.yaml`:
   ```yaml
   cron:
     enabled: true
   ```
4. Did you restart gateway after adding? `memdoor gateway restart`

**Debug:**
```bash
# Start gateway in verbose mode
memdoor gateway --verbose

# Watch for cron logs
tail -f ~/.memdoor/logs/*.log | grep -i cron
```

---

### Invalid Cron Expression

**Symptom:** `cron add` fails with "invalid cron schedule".

**Cause:** Cron expression syntax error.

**Examples:**
```bash
#  5-field standard crontab — OK, daily at 9:00 AM
--schedule "0 9 * * *"

#  6-field with seconds — also OK, daily at 9:00:00 AM
--schedule "0 0 9 * * *"

#  seconds only matter for sub-minute schedules
--schedule "*/30 * * * * *"  # every 30 seconds
```

**Validation:**
```bash
# Test expression online
# https://crontab.guru/ (note: 5-field only, add seconds manually)

# Or use Go code validation
go run -exec="cron.Parse" <expr>
```

---

### Job Execution Fails

**Symptom:** Job runs but shows FAILED in history.

**Check history:**
```bash
memdoor cron history <job-id> --limit 10
```

**Common errors:**
- **Rate limit exceeded**: Jobs too frequent (>10 req/min)
- **Timeout**: Job exceeded 10 minutes
- **Tool error**: Tool call failed (file not found, API error, etc.)
- **API error**: Taskforce API returned error

**Debug:**
```bash
# View full error message
memdoor cron history <job-id> --limit 1

# Check session history for details
cat ~/.memdoor/sessions/main/cron:<job-id>.jsonl | tail -20
```

---

### Jobs Run Multiple Times

**Symptom:** Job executes more than once per schedule.

**Cause:** Multiple gateway instances or cron entry duplicates.

**Fix:**
```bash
# Kill all gateway instances
pkill -f "memdoor gateway"

# Restart single instance
memdoor gateway --verbose &

# Verify single job entry
memdoor cron list
```

---

### High Token Usage

**Symptom:** Cron jobs consume excessive tokens.

**Cause:** Jobs maintain full conversation history, growing over time.

**Solution 1: Auto-compaction (automatic)**
- Jobs trigger compaction at 60% context (local pruning)
- Compaction at 90% context (AI summarization)

**Solution 2: Manual reset**
```bash
# Archive old session
mv ~/.memdoor/sessions/main/cron:<job-id>.jsonl \
   ~/.memdoor/sessions/main/cron:<job-id>.jsonl.backup

# Next run starts fresh
```

**Solution 3: Design stateless jobs**
```bash
#  BAD: References past runs
--message "Compare today's metrics to yesterday's"

#  GOOD: Self-contained
--message "Analyze today's metrics: query last 24h, calculate stats, report findings"
```

---

## Advanced Usage

### Multi-Agent Cron Jobs

Assign jobs to different agents:

```bash
# Analyst agent for reports
memdoor cron add \
  --id "analytics-report" \
  --schedule "0 0 9 * * *" \
  --agent "analyst" \
  --message "Generate daily analytics report"

# Monitor agent for health checks
memdoor cron add \
  --id "health-check" \
  --schedule "0 */15 * * * *" \
  --agent "monitor" \
  --message "Check system health"
```

**Benefits:**
- Different tool profiles (analyst has BigQuery, monitor has read-only)
- Isolated conversation contexts
- Separate memory databases

---

### Chained Jobs

Create job sequences using A2A messaging:

```bash
# Job 1: Data collection
memdoor cron add \
  --id "collect-data" \
  --schedule "0 0 8 * * *" \
  --message "Collect daily metrics, store in memory, send 'data-ready' message to analyst agent"

# Job 2: Analysis (triggered by Job 1)
memdoor cron add \
  --id "analyze-data" \
  --schedule "0 5 8 * * *" \
  --message "Wait for 'data-ready' message, retrieve metrics from memory, generate report"
```

---

### Dynamic Job Messages

Use conversation context to vary behavior:

```bash
memdoor cron add \
  --id "adaptive-monitor" \
  --schedule "0 */30 * * * *" \
  --message "If last run detected errors, do deep analysis. Otherwise, quick health check only."
```

Agent uses conversation history to decide action depth.

---

## Related Documentation

- **[CLI Reference](CLI.md)** - Full CLI command reference
- **[Logs System](LOGS.md)** - Event logging and querying
- **[Architecture](ARCHITECTURE.md)** - System architecture overview
- **[Memory Tool](../MEMORY_TOOL.md)** - Agent memory and RAG

---

**Status**: Production Ready  | 100% OpenClaw Cron Parity