# Stdin Piping (Killer Feature)

Memdoor's unique ability to pipe stdin directly to agents for real-time analysis and processing.

## Why It's Powerful

Traditional AI assistants require copy-pasting data. Memdoor integrates directly into your Unix workflow, allowing you to pipe any command output directly to agents for analysis.

## Usage Pattern

```bash
command | ./memdoor agent -m "your question about the input" -c general -a coder
```

The agent receives both your question and the piped input as context.

## Real-World Examples

### 1. Production Error Analysis

Analyze errors as they happen in production:

```bash
tail -f production.log | grep ERROR | ./memdoor agent -m "Analyze these errors and suggest fixes" -c general -a coder
```

The agent will:
- Identify error patterns
- Suggest root causes
- Recommend fixes
- Prioritize by severity

### 2. Real-time Log Monitoring

Continuous monitoring with intelligent summarization:

```bash
kubectl logs -f pod-name | ./memdoor agent -m "Summarize system activity" -c general -a coder
```

Perfect for:
- Deployment monitoring
- Anomaly detection
- Performance analysis
- Security monitoring

### 3. Test Failure Debugging

Understand why tests are failing:

```bash
npm test 2>&1 | ./memdoor agent -m "Why are these tests failing?" -c general -a coder
```

Agent provides:
- Test failure analysis
- Root cause identification
- Fix suggestions
- Related test patterns

### 4. Code Pattern Analysis

Find and prioritize technical debt:

```bash
grep -r "TODO" src/ | ./memdoor agent -m "Prioritize these TODOs by impact" -c general -a coder
```

Agent helps with:
- Impact assessment
- Effort estimation
- Dependency analysis
- Prioritization

### 5. Git Diff Review

Automated code review:

```bash
git diff | ./memdoor agent -m "Review this code change for issues" -c general -a coder
```

Agent checks for:
- Logic errors
- Security vulnerabilities
- Style violations
- Best practice deviations

## Advanced Patterns

### Combining Multiple Commands

```bash
# Find large files and ask for cleanup strategy
find . -type f -size +10M | ./memdoor agent -m "Which files should I delete?" -c general -a coder
```

### Processing Structured Data

```bash
# Analyze JSON logs
cat logs.json | jq '.errors' | ./memdoor agent -m "What are the most common error types?" -c general -a coder
```

### Continuous Integration

```bash
# Monitor build output
make build 2>&1 | ./memdoor agent -m "Are there any warnings I should fix?" -c general -a coder
```

### System Administration

```bash
# Analyze disk usage
df -h | ./memdoor agent -m "Which partitions need attention?" -c general -a coder
```

### Performance Profiling

```bash
# Profile analysis
go tool pprof -text profile.pb.gz | ./memdoor agent -m "Where are the bottlenecks?" -c general -a coder
```

## How It Works

1. **Input Detection**: CLI detects data on stdin
2. **Context Building**: Stdin content added to message context
3. **Agent Processing**: Agent receives both question and input
4. **Response**: Agent analyzes and responds with insights

## Best Practices

### 1. Keep Input Focused

```bash
# Good: Filter first, then analyze
grep ERROR production.log | ./memdoor agent -m "Analyze errors" -c general -a coder

# Bad: Send everything
cat production.log | ./memdoor agent -m "Find errors" -c general -a coder  # Too much data
```

### 2. Ask Specific Questions

```bash
# Good: Specific question
git diff | ./memdoor agent -m "Are there any security issues in this diff?" -c general -a coder

# Bad: Vague question
git diff | ./memdoor agent -m "What do you think?" -c general -a coder
```

### 3. Use Time Windows

```bash
# Analyze recent activity only
tail -100 production.log | ./memdoor agent -m "Any issues?" -c general -a coder
```

### 4. Combine with Other Tools

```bash
# Extract, transform, then analyze
cat data.json | jq '.errors' | sort | uniq -c | ./memdoor agent -m "Summarize error patterns" -c general -a coder
```

## Limitations

### Input Size
- **Soft limit**: 100KB recommended
- **Hard limit**: 1MB (model context limits)
- **Best practice**: Filter input before piping

### Token Limits
Large inputs consume token budget:
- Pre-filter with `head`, `tail`, `grep`
- Use `jq` to extract relevant JSON fields
- Sample large datasets

### Binary Data
Don't pipe binary data - the agent expects text:
```bash
# Bad
cat image.png | ./memdoor agent -m "What is this?" -c general -a coder

# Good
file image.png | ./memdoor agent -m "What type of file?" -c general -a coder
```

## Integration Ideas

### Git Hooks

Pre-commit hook for automatic review:
```bash
#!/bin/bash
git diff --cached | ./memdoor agent -m "Any issues with this commit?" -c general -a coder > review.txt
```

### CI/CD Pipeline

```yaml
# .github/workflows/analyze.yml
- name: Analyze test failures
  run: |
    npm test 2>&1 | ./memdoor agent -m "Summarize failures" -c general -a coder > analysis.md
```

### Monitoring Scripts

```bash
#!/bin/bash
# monitor.sh - Continuous log analysis
tail -f /var/log/app.log | while read line; do
  echo "$line" | ./memdoor agent -m "Is this concerning?" -c general -a coder
done
```

### Development Workflow

```bash
# Alias for quick error checking
alias checkerr='grep -i error | ./memdoor agent -m "Explain these errors" -c general -a coder'

# Usage
npm run build 2>&1 | checkerr
```

## Examples by Domain

### DevOps
```bash
docker ps --format "table {{.Names}}\t{{.Status}}" | ./memdoor agent -m "Any unhealthy containers?" -c general -a coder
```

### Security
```bash
sudo tail -f /var/log/auth.log | grep -i fail | ./memdoor agent -m "Any suspicious login attempts?" -c general -a coder
```

### Data Analysis
```bash
cat sales.csv | ./memdoor agent -m "What are the trends in this sales data?" -c general -a coder
```

### System Monitoring
```bash
top -b -n 1 | ./memdoor agent -m "Any performance issues?" -c general -a coder
```

## See Also

- [CLI Reference](../reference/CLI.md)
- [Agent Configuration](../developers/BUILDING_AGENTS.md)
- [Chat Command](../reference/CLI.md#chat)
