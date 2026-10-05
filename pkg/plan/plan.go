// Package plan is what a workspace has bought: free (the agent, workflows and
// the decision model on your own key) or a paid plan — Pro, the $10/month seat
// with remote control and the hosted workflow state (laptop-closed runs coming), or Enterprise by
// contract. "solo" and "team" are old names for
// Pro and still parse.
package plan

import (
	"fmt"
	"strings"
)

// Plan is a workspace's billing plan.
//
// # These values are STORAGE KEYS and do not change
//
// The middle plan was renamed three times in one day — team, pro, solo — and
// each rename cost a schema migration, a CHECK constraint rebuild and another
// entry in the legacy-alias list, for a word nobody outside the price list ever
// sees. So the rule, from here on:
//
//	the CONSTANT is what is written to disk, and it is frozen
//	the LABEL is what a human reads, and it is free to change
//
// Rename the label, the pricing page, the invoice, the marketing — none of it
// touches a row, a migration, or this file's aliases. TestStorageKeysAreFrozen
// fails if a constant moves, and says this.
type Plan string

const (
	// Free runs the agent on the person's own provider key and holds no seat:
	// it costs us nothing.
	Free Plan = "free"

	// Solo was the developer product's prepaid tier. It is RETIRED (2026-09-05, Greg: "there is no balance…
	// it's now subscription… Free or subscription"). The constant stays
	// because it is a storage key rows were written under, and it reads as
	// Pro: the workspaces that held it are subscribers now.
	Solo Plan = "solo"

	// Enterprise is a company's plan, paid by INVOICE rather than by card: a
	// company has a purchasing department.
	//
	// It is NOT self-serve. It has contractual terms attached, so it is arranged in a conversation and provisioned by
	// hand — the plan, the users and the admins are set on the workspace by an
	// operator. There is deliberately no upgrade button: an upgrade button
	// implies a price a customer can accept alone, and this one has a
	// signature on it.
	Enterprise Plan = "enterprise"

	// Pro is what a customer buys: $10 per SEAT per month (or yearly): remote
	// control through the memdoor.ai relay and the hosted workflow state (laptop-closed runs coming).
	// A seat never supplies a model or decisions. The subscription IS the entitlement;
	// no balance stands behind it. One seat by default; a team buys more.
	Pro Plan = "pro"
)

// Parse validates a plan at the boundary. Empty input is Free: a workspace that
// has never chosen must not be assumed to have bought anything.
func Parse(s string) (Plan, error) {
	switch p := Plan(strings.TrimSpace(strings.ToLower(s))); p {
	case "":
		return Free, nil
	case Free, Enterprise, Pro:
		return p, nil
	case Solo, "team":
		// The prepaid tier is retired: a row written under it is a
		// subscriber (2026-09-05). Storage keys are frozen; what they MEAN
		// is allowed to change, once, with a note. Nothing constructs this
		// value any more — Parse is the only place it appears, and it
		// leaves as Pro.
		return Pro, nil
	default:
		return "", fmt.Errorf("unknown plan %q (want free | pro | enterprise)", s)
	}
}

// String is the STORAGE KEY. Use it for the database, the API and the logs —
// anywhere a value is written down or compared. Never for a user-facing
// surface, which is what Label is for.
func (p Plan) String() string { return string(p) }

// Label is what a person reads. It may be changed freely — the price list can
// call this plan anything at all without a migration, because nothing is
// stored under it.
func (p Plan) Label() string {
	switch p {
	case Free:
		return "Free"
	case Pro:
		return "Pro"
	case Enterprise:
		return "Enterprise"
	default:
		return string(p)
	}
}

// ONE WORKSPACE, ALWAYS. What varies is how many PEOPLE are in it.
//
//	solo        one workspace, one developer
//	enterprise  one workspace, N users
//
// There is no MaxWorkspaces method because no plan varies it. The workspace is
// the unit of billing, identity, memory, skills and access throughout the
// codebase, so a second one is a second of all of those — and the shared
// sessions and skills are the whole point of a team being in one. An
// enterprise buys more CAPACITY and more SEATS inside its workspace, never
// more workspaces.
//
// That is also why growing from solo to enterprise is not a migration: the
// workspace, its memory and its history carry over untouched, and only the
// billing instrument and the number of people change.
//
// Stated here rather than enforced, because nothing can currently create a
// second workspace on SQLite anyway (WorkspaceRepository.Create is a stub).
