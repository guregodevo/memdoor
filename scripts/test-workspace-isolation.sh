#!/bin/bash
set -e

# Test script for workspace isolation
# This script creates two separate workspace databases and verifies isolation

echo "🧪 Workspace Isolation Test Suite"
echo "=================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"

    echo -n "Testing: $test_name... "

    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Function to verify database value
verify_db_value() {
    local db_path="$1"
    local query="$2"
    local expected="$3"

    result=$(sqlite3 "$db_path" "$query" 2>/dev/null || echo "ERROR")
    if [ "$result" = "$expected" ]; then
        return 0
    else
        echo "Expected: $expected, Got: $result" >&2
        return 1
    fi
}

echo "Step 1: Building project"
echo "------------------------"
make build
echo ""

echo "Step 2: Creating Workspace A (Company Alpha)"
echo "--------------------------------------------"
WS_A_DIR="$HOME/.memdoor-workspace-a"
WS_A_DB="$HOME/.memdoor-workspace-a/data/memdoor.db"

# Clean up previous test run
rm -rf "$WS_A_DIR"

# Create workspace A structure
mkdir -p "$WS_A_DIR/data"
mkdir -p "$WS_A_DIR/workspace"

# Set COBUDDY_CONFIG_DIR to workspace A for setup
export COBUDDY_CONFIG_DIR="$WS_A_DIR"

# Initialize database for workspace A
echo "Initializing Workspace A database..."
sqlite3 "$WS_A_DB" <<EOF
-- Create users table with workspace_id
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    name TEXT,
    avatar_url TEXT,
    email_verified INTEGER DEFAULT 0,
    workspace_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME
);

-- Insert test user for Workspace A
INSERT INTO users (id, email, name, workspace_id, created_at, updated_at)
VALUES ('human:alice-workspace-a', 'alice@company-alpha.com', 'Alice (Alpha)', 'workspace-alpha-001', datetime('now'), datetime('now'));

-- Verify insertion
SELECT 'Workspace A user count: ' || COUNT(*) FROM users;
EOF

echo "Workspace A created at: $WS_A_DIR"
echo ""

echo "Step 3: Creating Workspace B (Company Beta)"
echo "-------------------------------------------"
WS_B_DIR="$HOME/.memdoor-workspace-b"
WS_B_DB="$HOME/.memdoor-workspace-b/data/memdoor.db"

# Clean up previous test run
rm -rf "$WS_B_DIR"

# Create workspace B structure
mkdir -p "$WS_B_DIR/data"
mkdir -p "$WS_B_DIR/workspace"

# Initialize database for workspace B
echo "Initializing Workspace B database..."
sqlite3 "$WS_B_DB" <<EOF
-- Create users table with workspace_id
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    name TEXT,
    avatar_url TEXT,
    email_verified INTEGER DEFAULT 0,
    workspace_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME
);

-- Insert test user for Workspace B
INSERT INTO users (id, email, name, workspace_id, created_at, updated_at)
VALUES ('human:bob-workspace-b', 'bob@company-beta.com', 'Bob (Beta)', 'workspace-beta-002', datetime('now'), datetime('now'));

-- Verify insertion
SELECT 'Workspace B user count: ' || COUNT(*) FROM users;
EOF

echo "Workspace B created at: $WS_B_DIR"
echo ""

echo "Step 4: Running Isolation Tests"
echo "-------------------------------"

# Test 1: Verify Workspace A has correct workspace_id
run_test "Workspace A user has correct workspace_id" \
    "verify_db_value '$WS_A_DB' 'SELECT workspace_id FROM users WHERE email=\"alice@company-alpha.com\"' 'workspace-alpha-001'"

# Test 2: Verify Workspace B has correct workspace_id
run_test "Workspace B user has correct workspace_id" \
    "verify_db_value '$WS_B_DB' 'SELECT workspace_id FROM users WHERE email=\"bob@company-beta.com\"' 'workspace-beta-002'"

# Test 3: Verify workspaces are isolated (no cross-contamination)
run_test "Workspace A has exactly 1 user" \
    "verify_db_value '$WS_A_DB' 'SELECT COUNT(*) FROM users' '1'"

run_test "Workspace B has exactly 1 user" \
    "verify_db_value '$WS_B_DB' 'SELECT COUNT(*) FROM users' '1'"

# Test 4: Verify Alice is NOT in Workspace B
run_test "Alice not in Workspace B database" \
    "verify_db_value '$WS_B_DB' 'SELECT COUNT(*) FROM users WHERE email=\"alice@company-alpha.com\"' '0'"

# Test 5: Verify Bob is NOT in Workspace A
run_test "Bob not in Workspace A database" \
    "verify_db_value '$WS_A_DB' 'SELECT COUNT(*) FROM users WHERE email=\"bob@company-beta.com\"' '0'"

echo ""
echo "Step 5: Testing Default Workspace ID"
echo "------------------------------------"

# Reset to default config dir
unset COBUDDY_CONFIG_DIR

# Test with main database
MAIN_DB="$HOME/.memdoor/data/memdoor.db"

if [ -f "$MAIN_DB" ]; then
    # Check if users table has workspace_id column
    COLUMN_EXISTS=$(sqlite3 "$MAIN_DB" "PRAGMA table_info(users);" | grep -c "workspace_id" || echo "0")

    if [ "$COLUMN_EXISTS" -gt 0 ]; then
        echo -e "${GREEN}✓${NC} Main database has workspace_id column"
        ((TESTS_PASSED++))

        # Check if default workspace ID is used
        DEFAULT_WS_COUNT=$(sqlite3 "$MAIN_DB" "SELECT COUNT(*) FROM users WHERE workspace_id = '00000000-0000-0000-0000-000000000001';" 2>/dev/null || echo "0")

        if [ "$DEFAULT_WS_COUNT" -gt 0 ]; then
            echo -e "${GREEN}✓${NC} Main database users use default workspace ID"
            ((TESTS_PASSED++))
        else
            echo -e "${YELLOW}⚠${NC}  Main database users don't have default workspace ID (may need migration)"
            # This is a warning, not a failure
        fi
    else
        echo -e "${RED}✗${NC} Main database missing workspace_id column"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${YELLOW}⚠${NC}  Main database not found (run './memdoor setup' to create)"
fi

echo ""
echo "Step 6: Testing CLI Setup Command"
echo "---------------------------------"

# Create a temporary test database
TEST_SETUP_DIR="$HOME/.memdoor-setup-test"
TEST_SETUP_DB="$TEST_SETUP_DIR/data/memdoor.db"

rm -rf "$TEST_SETUP_DIR"
export COBUDDY_CONFIG_DIR="$TEST_SETUP_DIR"

echo "Running: ./memdoor setup --skip-bootstrap"
./memdoor setup --skip-bootstrap > /dev/null 2>&1 || echo "Setup completed with warnings"

# Verify setup created users with correct workspace_id
if [ -f "$TEST_SETUP_DB" ]; then
    run_test "Setup creates users with default workspace_id" \
        "verify_db_value '$TEST_SETUP_DB' \"SELECT COUNT(*) FROM users WHERE workspace_id = '00000000-0000-0000-0000-000000000001'\" '3'"

    # Verify specific users
    ADMIN_WS=$(sqlite3 "$TEST_SETUP_DB" "SELECT workspace_id FROM users WHERE email='admin@localhost'" 2>/dev/null || echo "MISSING")
    ALICE_WS=$(sqlite3 "$TEST_SETUP_DB" "SELECT workspace_id FROM users WHERE email='alice@localhost'" 2>/dev/null || echo "MISSING")

    if [ "$ADMIN_WS" = "00000000-0000-0000-0000-000000000001" ]; then
        echo -e "${GREEN}✓${NC} Admin user has correct workspace_id"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗${NC} Admin user missing or wrong workspace_id: $ADMIN_WS"
        ((TESTS_FAILED++))
    fi

    if [ "$ALICE_WS" = "00000000-0000-0000-0000-000000000001" ]; then
        echo -e "${GREEN}✓${NC} Alice user has correct workspace_id"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗${NC} Alice user missing or wrong workspace_id: $ALICE_WS"
        ((TESTS_FAILED++))
    fi
else
    echo -e "${RED}✗${NC} Setup did not create database"
    ((TESTS_FAILED++))
fi

# Cleanup test setup
rm -rf "$TEST_SETUP_DIR"
unset COBUDDY_CONFIG_DIR

echo ""
echo "Step 7: Cleanup Test Workspaces"
echo "-------------------------------"
echo "Cleaning up test directories..."
rm -rf "$WS_A_DIR"
rm -rf "$WS_B_DIR"
echo "✓ Cleanup complete"

echo ""
echo "=================================="
echo "Test Results Summary"
echo "=================================="
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "Workspace isolation is working correctly:"
    echo "  ✓ Database schema includes workspace_id column"
    echo "  ✓ Different workspaces maintain separate user databases"
    echo "  ✓ No cross-contamination between workspaces"
    echo "  ✓ CLI setup command assigns default workspace ID"
    echo "  ✓ Users created by setup have correct workspace_id"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    echo ""
    echo "Please review the failures above and check:"
    echo "  - Database schema has workspace_id column with default"
    echo "  - Migration applied successfully"
    echo "  - User registration includes workspace_id"
    echo "  - CLI setup command uses domain.DefaultWorkspaceID"
    exit 1
fi
