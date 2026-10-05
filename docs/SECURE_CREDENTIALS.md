# Secure Credential Management for Browser Automation

This guide explains how to safely use credentials with Memdoor's browser automation tools.

The examples below use `./scripts/check-portal.sh` and an `internal-portal` host as placeholders for your own automation script and the site it logs into.

## Quick Start

### 1. Set Up Environment Variables

```bash
# Copy the example file
cp .env.example .env

# Edit .env with your credentials (NEVER commit this file!)
nano .env

# Load credentials
source .env

# Run automation
./scripts/check-portal.sh
```

### 2. Running an Automation Script

```bash
# Set credentials (option 1: inline)
export PORTAL_USERNAME="your-username"
export PORTAL_PASSWORD="your-password"
./scripts/check-portal.sh

# Set credentials (option 2: from .env file)
source .env
./scripts/check-portal.sh

# Set credentials (option 3: one-liner)
PORTAL_USERNAME=user PORTAL_PASSWORD=pass ./scripts/check-portal.sh
```

## Security Methods (Best to Worst)

### ⭐ Method 1: macOS Keychain (Most Secure)

```bash
# Store password in keychain (one-time setup)
security add-generic-password \
  -a "$USER" \
  -s "internal-portal" \
  -w "your-password"

# Use in scripts
export PORTAL_USERNAME="your-username"
export PORTAL_PASSWORD=$(security find-generic-password \
  -a "$USER" \
  -s "internal-portal" \
  -w)

./scripts/check-portal.sh
```

**Pros:**
- OS-level encryption
- Not stored in plaintext
- Secure against file access

### ⭐ Method 2: .env File (Good for Development)

```bash
# Create .env file (already in .gitignore)
cat > .env <<EOF
PORTAL_USERNAME=your-username
PORTAL_PASSWORD=your-password
EOF

# Secure the file
chmod 600 .env

# Use it
source .env
./scripts/check-portal.sh
```

**Pros:**
- Easy to use
- Not in shell history
- Per-project credentials

**Cons:**
- Risk of accidental commit
- Plaintext file

### ⭐ Method 3: Environment Variables (OK for Testing)

```bash
# Set temporarily (lost after closing terminal)
export PORTAL_USERNAME="your-username"
export PORTAL_PASSWORD="your-password"

./scripts/check-portal.sh
```

**Pros:**
- Quick and simple
- No files to manage

**Cons:**
- Visible in process list
- Lost when terminal closes
- Saved in shell history

###  Method 4: Hardcoded (NEVER DO THIS)

```bash
#  BAD - Don't do this!
./login.sh "username" "password"

#  BAD - Don't do this!
PASS="secret123" ./script.sh
```

## Security Best Practices

###  DO:

1. **Use .env for development**
   ```bash
   cp .env.example .env
   chmod 600 .env
   ```

2. **Check .gitignore**
   ```bash
   grep ".env" .gitignore  # Should show .env
   ```

3. **Use restrictive permissions**
   ```bash
   chmod 600 .env
   chmod 700 ~/.portal-creds
   ```

4. **Clear sensitive variables**
   ```bash
   unset PORTAL_PASSWORD
   ```

5. **Mask passwords in logs**
   ```bash
   echo "Login as: ${PORTAL_USERNAME}"  #  Good
   # Don't log: ${PORTAL_PASSWORD}      #  Bad
   ```

###  DON'T:

1. **Never commit credentials**
   ```bash
   git add .env           #  NO!
   git add secrets.txt    #  NO!
   ```

2. **Never hardcode passwords**
   ```javascript
   const password = "secret123";  //  NO!
   ```

3. **Never share .env files**
   ```bash
   scp .env server:/app/  #  NO!
   ```

4. **Never use 777 permissions**
   ```bash
   chmod 777 .env         #  NO!
   ```

## Example: Complete Secure Workflow

```bash
#!/bin/bash
# Example: Secure automation script

set -euo pipefail  # Fail fast

# 1. Load credentials from secure source
if [ -f .env ]; then
    source .env
elif [ -f ~/.portal-creds ]; then
    source ~/.portal-creds
else
    echo " No credentials found"
    exit 1
fi

# 2. Validate credentials exist
: "${PORTAL_USERNAME:?Error: PORTAL_USERNAME not set}"
: "${PORTAL_PASSWORD:?Error: PORTAL_PASSWORD not set}"

# 3. Run automation
echo "🔐 Logging in as: ${PORTAL_USERNAME}"
./memdoor chrome run <<EOF
nav https://internal-portal.your-company.com/pr-inbox
type input[name="username"] ${PORTAL_USERNAME}
type input[name="password"] ${PORTAL_PASSWORD}
click button
EOF

# 4. Clean up
unset PORTAL_PASSWORD
echo " Done (password cleared from memory)"
```

## Troubleshooting

### "Credentials not found"
```bash
# Check if variables are set
echo $PORTAL_USERNAME
echo $PORTAL_PASSWORD

# If empty, load them
source .env
```

### "Permission denied"
```bash
# Fix .env permissions
chmod 600 .env
```

### "Command not found: ./scripts/check-portal.sh"
```bash
# Make script executable
chmod +x scripts/check-portal.sh
```

## Advanced: Using OAuth Tokens

Instead of passwords, you can use OAuth tokens for better security:

```bash
# 1. Extract token from browser (manual step)
# Open DevTools → Application → Local Storage → okta-token

# 2. Set as environment variable
export PORTAL_OAUTH_TOKEN="your-token-here"

# 3. Use in automation
./memdoor chrome run <<EOF
nav https://internal-portal.your-company.com/pr-inbox
wait 2
exec localStorage.setItem('okta-token-storage', '{"accessToken":"${PORTAL_OAUTH_TOKEN}"}')
exec location.reload()
wait 5
EOF
```

## Production Deployment

For production use, consider:

1. **AWS Secrets Manager / HashiCorp Vault**
   - Centralized secret storage
   - Automatic rotation
   - Audit logging

2. **CI/CD Secret Management**
   - GitHub Secrets
   - GitLab CI/CD Variables
   - Jenkins Credentials

3. **Service Accounts**
   - Dedicated automation accounts
   - Limited permissions
   - Easy to revoke

## Questions?

- Security concerns: Contact your company's security team
- Technical issues: See `docs/features/BROWSER_AUTOMATION.md`
- General questions: Ask in #memdoor-dev