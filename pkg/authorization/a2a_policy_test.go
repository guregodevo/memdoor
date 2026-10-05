package authorization

import (
	"testing"
)

// TestDefaultA2APolicy pins the default for the local single-user product: A2A is
// ON with a *→* allow rule (the seeded agents collaborate — one @-mentions
// another), bounded to fire-and-forget by MaxPingPongTurns=0. Stricter
// rosters come from config (loadA2APolicy), which replaces this default.
func TestDefaultA2APolicy(t *testing.T) {
	policy := DefaultA2APolicy()

	if !policy.Enabled {
		t.Error("Default policy should be enabled (seeded agents collaborate)")
	}

	if len(policy.Allow) != 1 || policy.Allow[0].From != "*" || policy.Allow[0].To != "*" {
		t.Errorf("Default policy should have the single *→* rule, got %+v", policy.Allow)
	}

	if policy.MaxPingPongTurns != 0 {
		t.Errorf("Default MaxPingPongTurns should be 0 (fire-and-forget), got %d", policy.MaxPingPongTurns)
	}

	if !policy.IsAllowed("main", "work") {
		t.Error("Default policy should allow agent pairs (the *→* rule)")
	}
}

// TestIsAllowed_Disabled tests that disabled policy denies everything
func TestIsAllowed_Disabled(t *testing.T) {
	policy := &A2APolicy{
		Enabled: false,
		Allow: []A2ARule{
			{From: "*", To: "*"}, // Even with allow-all rule, disabled means deny
		},
	}

	if policy.IsAllowed("main", "work") {
		t.Error("Disabled policy should deny even with allow-all rule")
	}
}

// TestIsAllowed_NoRules tests that enabled policy with no rules denies all
func TestIsAllowed_NoRules(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow:   []A2ARule{}, // No rules = deny all
	}

	if policy.IsAllowed("main", "work") {
		t.Error("Policy with no rules should deny all")
	}
}

// TestIsAllowed_ExactMatch tests exact agent ID matching
func TestIsAllowed_ExactMatch(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow: []A2ARule{
			{From: "main", To: "work"},
		},
	}

	// Should allow exact match
	if !policy.IsAllowed("main", "work") {
		t.Error("Policy should allow exact match: main → work")
	}

	// Should deny reverse
	if policy.IsAllowed("work", "main") {
		t.Error("Policy should deny reverse: work → main")
	}

	// Should deny different pairs
	if policy.IsAllowed("main", "data") {
		t.Error("Policy should deny non-matching pair: main → data")
	}
}

// TestIsAllowed_FromWildcard tests wildcard in 'from' field
func TestIsAllowed_FromWildcard(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow: []A2ARule{
			{From: "*", To: "work"}, // Any agent can message work
		},
	}

	// Any agent → work should be allowed
	if !policy.IsAllowed("main", "work") {
		t.Error("Policy should allow main → work (from wildcard)")
	}

	if !policy.IsAllowed("data", "work") {
		t.Error("Policy should allow data → work (from wildcard)")
	}

	if !policy.IsAllowed("foo", "work") {
		t.Error("Policy should allow foo → work (from wildcard)")
	}

	// Any agent → other targets should be denied
	if policy.IsAllowed("main", "data") {
		t.Error("Policy should deny main → data (target not work)")
	}
}

// TestIsAllowed_ToWildcard tests wildcard in 'to' field
func TestIsAllowed_ToWildcard(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow: []A2ARule{
			{From: "main", To: "*"}, // main can message any agent
		},
	}

	// main → any agent should be allowed
	if !policy.IsAllowed("main", "work") {
		t.Error("Policy should allow main → work (to wildcard)")
	}

	if !policy.IsAllowed("main", "data") {
		t.Error("Policy should allow main → data (to wildcard)")
	}

	if !policy.IsAllowed("main", "foo") {
		t.Error("Policy should allow main → foo (to wildcard)")
	}

	// Other agents → any should be denied
	if policy.IsAllowed("work", "main") {
		t.Error("Policy should deny work → main (from not main)")
	}
}

// TestIsAllowed_BothWildcards tests allow-all policy
func TestIsAllowed_BothWildcards(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow: []A2ARule{
			{From: "*", To: "*"}, // Allow all agent pairs
		},
	}

	// Everything should be allowed
	testCases := []struct {
		from string
		to   string
	}{
		{"main", "work"},
		{"work", "main"},
		{"data", "monitoring"},
		{"foo", "bar"},
	}

	for _, tc := range testCases {
		if !policy.IsAllowed(tc.from, tc.to) {
			t.Errorf("Policy should allow %s → %s (both wildcards)", tc.from, tc.to)
		}
	}
}

// TestIsAllowed_MultipleRules tests policy with multiple rules
func TestIsAllowed_MultipleRules(t *testing.T) {
	policy := &A2APolicy{
		Enabled: true,
		Allow: []A2ARule{
			{From: "main", To: "work"},
			{From: "main", To: "data"},
			{From: "work", To: "data"},
		},
	}

	// Allowed pairs
	allowed := [][2]string{
		{"main", "work"},
		{"main", "data"},
		{"work", "data"},
	}

	for _, pair := range allowed {
		if !policy.IsAllowed(pair[0], pair[1]) {
			t.Errorf("Policy should allow %s → %s", pair[0], pair[1])
		}
	}

	// Denied pairs
	denied := [][2]string{
		{"work", "main"}, // Reverse
		{"data", "main"}, // Reverse
		{"data", "work"}, // Reverse
		{"main", "foo"},  // Not in rules
		{"foo", "bar"},   // Not in rules
	}

	for _, pair := range denied {
		if policy.IsAllowed(pair[0], pair[1]) {
			t.Errorf("Policy should deny %s → %s", pair[0], pair[1])
		}
	}
}

// TestValidate_ValidPolicies tests validation of valid policies
func TestValidate_ValidPolicies(t *testing.T) {
	validPolicies := []*A2APolicy{
		{
			Enabled: true,
			Allow: []A2ARule{
				{From: "main", To: "work"},
			},
			MaxPingPongTurns: 5,
		},
		{
			Enabled: true,
			Allow: []A2ARule{
				{From: "*", To: "*"},
			},
			MaxPingPongTurns: 0,
		},
		{
			Enabled:          false,
			Allow:            []A2ARule{},
			MaxPingPongTurns: 0,
		},
	}

	for i, policy := range validPolicies {
		if err := policy.Validate(); err != nil {
			t.Errorf("Policy %d should be valid, got error: %v", i, err)
		}
	}
}

// TestValidate_InvalidRules tests validation catches invalid rules
func TestValidate_InvalidRules(t *testing.T) {
	testCases := []struct {
		name   string
		policy *A2APolicy
	}{
		{
			name: "Empty from field",
			policy: &A2APolicy{
				Enabled: true,
				Allow: []A2ARule{
					{From: "", To: "work"},
				},
			},
		},
		{
			name: "Empty to field",
			policy: &A2APolicy{
				Enabled: true,
				Allow: []A2ARule{
					{From: "main", To: ""},
				},
			},
		},
		{
			name: "Negative MaxPingPongTurns",
			policy: &A2APolicy{
				Enabled:          true,
				Allow:            []A2ARule{{From: "main", To: "work"}},
				MaxPingPongTurns: -1,
			},
		},
		{
			name: "MaxPingPongTurns too high",
			policy: &A2APolicy{
				Enabled:          true,
				Allow:            []A2ARule{{From: "main", To: "work"}},
				MaxPingPongTurns: 11,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.policy.Validate(); err == nil {
				t.Errorf("Expected validation error for: %s", tc.name)
			}
		})
	}
}

// TestGetMaxPingPongTurns tests getting max turns
func TestGetMaxPingPongTurns(t *testing.T) {
	policy := &A2APolicy{
		Enabled:          true,
		Allow:            []A2ARule{{From: "main", To: "work"}},
		MaxPingPongTurns: 5,
	}

	if policy.GetMaxPingPongTurns() != 5 {
		t.Errorf("Expected MaxPingPongTurns 5, got %d", policy.GetMaxPingPongTurns())
	}
}

// TestString tests the String() method for human-readable output
func TestString(t *testing.T) {
	testCases := []struct {
		name     string
		policy   *A2APolicy
		contains string
	}{
		{
			name:     "Disabled policy",
			policy:   &A2APolicy{Enabled: false},
			contains: "DISABLED",
		},
		{
			name: "Enabled with no rules",
			policy: &A2APolicy{
				Enabled: true,
				Allow:   []A2ARule{},
			},
			contains: "no rules",
		},
		{
			name: "Enabled with rules",
			policy: &A2APolicy{
				Enabled: true,
				Allow: []A2ARule{
					{From: "main", To: "work"},
					{From: "main", To: "data"},
				},
			},
			contains: "2 rule(s)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			str := tc.policy.String()
			if str == "" {
				t.Error("String() should not return empty string")
			}
			// Just verify it doesn't crash and returns something meaningful
			t.Logf("Policy string: %s", str)
		})
	}
}
