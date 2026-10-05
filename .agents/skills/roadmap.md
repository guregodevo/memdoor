---
name: roadmap
description: Managing and updating the Memdoor roadmap (MUST/SHOULD/COULD priority system)
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: 🗺️
    skillKey: roadmap
    always: false
    os:
      - darwin
      - linux
---

# Roadmap Management Skill

Managing the Memdoor roadmap using the MoSCoW prioritization method (MUST/SHOULD/COULD).

## Roadmap Structure

```
docs/roadmap/
├── ROADMAP.md          # Overview + historical milestones
├── MUST.md            # 🔥 Critical for OSS launch (P0)
├── SHOULD.md          # Important for growth (P1)
└── COULD.md           # Nice to have (P2)
```

### Priority Definitions

**MUST.md** (P0 - Critical)
- Blocking OSS launch
- Security critical
- Core functionality
- Must be done FIRST

**SHOULD.md** (P1 - Important)
- Important for growth
- Significantly improves UX
- Revenue generation
- Do AFTER MUST

**COULD.md** (P2 - Nice to Have)
- Advanced features
- Platform expansion
- Developer experience
- Do AFTER SHOULD

---

## When a Task is Completed

### Step 1: Check Status

```bash
# View MUST tasks
cat docs/roadmap/MUST.md

# View SHOULD tasks
cat docs/roadmap/SHOULD.md

# View COULD tasks
cat docs/roadmap/COULD.md

# Search for specific task
grep -r "Task name" docs/roadmap/
```

### Step 2: Mark as Done

**Remove from priority file:**

```bash
# Edit the appropriate file
# MUST.md, SHOULD.md, or COULD.md

# Remove the completed line:
- [ ] Task description

# Do NOT move to ROADMAP.md - just delete it
```

### Step 3: Update ROADMAP.md (Optional)

**Only for major milestones:**

```bash
# Edit docs/roadmap/ROADMAP.md

# Update "Last Updated" date
**Last Updated**: 2026-03-XX

# Update "Current Focus" if priority changed
**Current Focus**: New focus area

# Add to completed section (if major milestone)
## ✅ Completed
- Task name (Date completed)
```

---

## Adding New Tasks

### Step 1: Determine Priority

**Ask these questions:**

1. **Blocks OSS launch?** → MUST.md
2. **Blocks revenue/growth?** → SHOULD.md
3. **Security critical?** → MUST.md
4. **Core functionality?** → MUST.md
5. **Nice enhancement?** → COULD.md

### Step 2: Add to Correct File

```bash
# Edit appropriate file
vim docs/roadmap/MUST.md
# or
vim docs/roadmap/SHOULD.md
# or
vim docs/roadmap/COULD.md

# Add in logical section:
## Section Name
- [ ] New task description
  - [ ] Sub-task 1
  - [ ] Sub-task 2

# Include "Why" explanation:
**Why MUST**: Explanation of criticality
**Why SHOULD**: Explanation of importance
**Why COULD**: Explanation of value
```

### Step 3: Order by Priority

Within each file, order tasks by:
1. Security critical (top)
2. Blocking dependencies
3. User impact
4. Effort vs value

---

## Workflow Examples

### Scenario 1: Feature Complete

```bash
# 1. Find completed task
grep -r "Direct messages" docs/roadmap/

# Found in: docs/roadmap/MUST.md
# - [ ] Direct messages (Human-to-Human, Human-to-Agent)

# 2. Remove from MUST.md
vim docs/roadmap/MUST.md
# Delete the line

# 3. Commit
git add docs/roadmap/MUST.md
git commit -m "Docs: Remove completed Direct Messages from roadmap"
```

### Scenario 2: New Feature Request

```bash
# 1. Determine priority
# Example: "Add message reactions"
# - Not blocking launch → Not MUST
# - Improves UX significantly → SHOULD
# - Not critical → Not MUST

# 2. Add to SHOULD.md
vim docs/roadmap/SHOULD.md

## User Experience Improvements
- [ ] Message reactions support
  - [ ] Reaction picker UI
  - [ ] Emoji rendering
  - [ ] Reaction counts
  - [ ] Database schema

**Why SHOULD**: Significantly improves user engagement but not blocking launch.

# 3. Commit
git add docs/roadmap/SHOULD.md
git commit -m "Docs: Add message reactions to SHOULD roadmap"
```

### Scenario 3: Priority Change

```bash
# Task moved from SHOULD → MUST

# 1. Remove from SHOULD.md
vim docs/roadmap/SHOULD.md
# Delete: - [ ] HTTPS / DNS setup

# 2. Add to MUST.md
vim docs/roadmap/MUST.md
## Infrastructure
- [ ] HTTPS / DNS setup

# 3. Update reason
**Why MUST**: Required for production deployment (security critical)

# 4. Commit
git add docs/roadmap/MUST.md docs/roadmap/SHOULD.md
git commit -m "Docs: Move HTTPS setup to MUST (security critical)"
```

---

## Reviewing Roadmap

### Weekly Review Checklist

```bash
# 1. Check completed items
grep -A 2 "\[x\]" docs/roadmap/MUST.md
grep -A 2 "\[x\]" docs/roadmap/SHOULD.md
grep -A 2 "\[x\]" docs/roadmap/COULD.md

# 2. Remove completed items
# Edit files and delete completed tasks

# 3. Re-prioritize if needed
# Check if any SHOULD → MUST
# Check if any COULD → SHOULD

# 4. Update ROADMAP.md current focus
vim docs/roadmap/ROADMAP.md
# Update "Current Focus"

# 5. Commit changes
git add docs/roadmap/
git commit -m "Docs: Weekly roadmap review (YYYY-MM-DD)"
```

### Monthly Review Checklist

- [ ] Review all MUST tasks - still critical?
- [ ] Review all SHOULD tasks - priority changed?
- [ ] Review all COULD tasks - any now important?
- [ ] Update ROADMAP.md milestone progress
- [ ] Archive completed major milestones
- [ ] Update timeline estimates

---

## Roadmap Hygiene

### Keep It Clean

**DO:**
✅ Delete completed tasks immediately
✅ Keep descriptions concise
✅ Include "Why" explanations
✅ Order by priority within sections
✅ Update regularly (weekly)
✅ Remove stale/cancelled tasks

**DON'T:**
❌ Keep completed tasks in files
❌ Add to ROADMAP.md (use priority files)
❌ Mix priorities (put in correct file)
❌ Let tasks go stale
❌ Add without "Why" explanation
❌ Duplicate across files

### Signs of Good Roadmap Hygiene

```bash
# Should return few/no results:
grep -r "\[x\]" docs/roadmap/MUST.md
grep -r "\[x\]" docs/roadmap/SHOULD.md
grep -r "\[x\]" docs/roadmap/COULD.md

# Each file should have clear sections
head -20 docs/roadmap/MUST.md

# Each section should have "Why"
grep -A 5 "Why MUST" docs/roadmap/MUST.md
```

---

## Quick Commands

### Check Priority Files

```bash
# View MUST (critical)
cat docs/roadmap/MUST.md

# View SHOULD (important)
cat docs/roadmap/SHOULD.md

# View COULD (nice to have)
cat docs/roadmap/COULD.md

# Count tasks in each
echo "MUST: $(grep -c '^\- \[ \]' docs/roadmap/MUST.md)"
echo "SHOULD: $(grep -c '^\- \[ \]' docs/roadmap/SHOULD.md)"
echo "COULD: $(grep -c '^\- \[ \]' docs/roadmap/COULD.md)"
```

### Search Roadmap

```bash
# Find task by keyword
grep -r "authentication" docs/roadmap/

# Find security tasks
grep -r "security" docs/roadmap/ | grep -i "\[ \]"

# Find UI tasks
grep -r "UI\|UX" docs/roadmap/ | grep -i "\[ \]"

# Find infrastructure tasks
grep -r "Infrastructure\|Database\|Deploy" docs/roadmap/
```

### Bulk Operations

```bash
# Remove all completed from MUST
sed -i '' '/\[x\]/d' docs/roadmap/MUST.md

# Remove all completed from SHOULD
sed -i '' '/\[x\]/d' docs/roadmap/SHOULD.md

# Remove all completed from COULD
sed -i '' '/\[x\]/d' docs/roadmap/COULD.md
```

---

## Integration with Development

### Before Starting Work

```bash
# 1. Check priority
cat docs/roadmap/MUST.md  # Work on MUST first!

# 2. Mark as in progress (optional)
# - [x] Task name (In Progress)

# 3. Create branch
git checkout -b feature/task-name
```

### After Completing Work

```bash
# 1. Remove from roadmap
vim docs/roadmap/MUST.md
# Delete completed task

# 2. Commit roadmap update
git add docs/roadmap/MUST.md
git commit -m "Docs: Remove completed task from MUST"

# 3. Merge feature branch
git checkout master
git merge feature/task-name
```

---

## Common Patterns

### Adding Feature with Sub-tasks

```markdown
## Section Name
- [ ] Main feature
  - [ ] Sub-task 1 (backend)
  - [ ] Sub-task 2 (frontend)
  - [ ] Sub-task 3 (tests)
  - [ ] Sub-task 4 (docs)

**Why MUST**: Clear reason for priority
```

### Grouping Related Tasks

```markdown
## Infrastructure 🔥
- [ ] PostgreSQL Migration
  - [ ] Database abstraction layer
  - [ ] Migration scripts from SQLite
  - [ ] Connection pooling
  - [ ] Support DATABASE_URL
```

### Security Critical Tasks

```markdown
## Authentication & Authorization 🔥
- [ ] Secrets Authorization - SECURITY CRITICAL
  - Only the agent's owner or an admin can set its secrets
```

---

## Best Practices

### Writing Task Descriptions

**Good:**
```markdown
- [ ] Direct messages (Human-to-Human, Human-to-Agent)
- [ ] HTTPS / DNS setup
- [ ] PostgreSQL migration with Row-Level Security
```

**Bad:**
```markdown
- [ ] DMs (unclear what DM means)
- [ ] Fix security (too vague)
- [ ] Update database (no specifics)
```

### Writing "Why" Explanations

**Good:**
```markdown
**Why MUST**: Security critical for production deployment
**Why SHOULD**: Important for viral growth, but core works without it
**Why COULD**: Enhances UX but not essential for launch
```

**Bad:**
```markdown
**Why MUST**: Important feature
**Why SHOULD**: Good to have
**Why COULD**: Maybe later
```

---

## Roadmap Review Checklist

**Before committing roadmap changes:**

- [ ] Completed tasks removed (not moved to ROADMAP.md)
- [ ] New tasks in correct priority file
- [ ] "Why" explanation included for new tasks
- [ ] Tasks ordered by priority within sections
- [ ] No duplicate tasks across files
- [ ] No stale/cancelled tasks
- [ ] Clear, specific task descriptions
- [ ] Sub-tasks indented properly

---

## Related Documentation

- `docs/roadmap/ROADMAP.md` - Roadmap overview and history
- `docs/roadmap/MUST.md` - Critical tasks (P0)
- `docs/roadmap/SHOULD.md` - Important tasks (P1)
- `docs/roadmap/COULD.md` - Nice to have (P2)
- `docs/roadmap/STRATEGY.md` - Current product strategy (positioning, public/private model, GTM, pricing). Renamed from `OPEN_SOURCE_STRATEGY.md` 2026-04-11; the product is no longer OSS.
- `AGENTS.md` - Development guidelines