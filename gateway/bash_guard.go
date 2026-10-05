package gateway

import (
	"regexp"
	"strings"
)

// destructiveBashReason reports whether a shell command is one of the clearly
// catastrophic, irreversible operations — wiping the filesystem root/home,
// formatting or writing a raw disk, or a fork bomb — and a short reason if so.
//
// This is a SAFETY NET for the coding agent (defense in depth), NOT a
// sandbox: a determined bypass is possible (quoting, env indirection). Its job is
// to stop a misfiring model from bricking the machine on an obvious command. Blocked
// commands are returned to the model as an error so it can choose a safe path.
func destructiveBashReason(command string) (string, bool) {
	c := " " + strings.ToLower(strings.Join(strings.Fields(command), " ")) + " "

	// Fork bomb, ignoring whitespace.
	if strings.Contains(strings.ReplaceAll(c, " ", ""), ":(){:|:&};:") {
		return "fork bomb", true
	}
	// Disabling root protection is always a red flag.
	if strings.Contains(c, "--no-preserve-root") {
		return "rm --no-preserve-root", true
	}
	// Recursive force-delete of the filesystem root or home directory.
	if rmRecursiveForce.MatchString(c) && rootTarget.MatchString(c) {
		return "recursive force-delete of a root/home path", true
	}
	// Formatting a filesystem or writing a raw disk device.
	if strings.Contains(c, " mkfs") || strings.Contains(c, "mkfs.") {
		return "mkfs formats a filesystem", true
	}
	if rawDeviceWrite.MatchString(c) {
		return "write to a raw disk device", true
	}
	return "", false
}

var (
	// rm with recursive+force flags in any order/combination (-rf, -fr, -r -f).
	rmRecursiveForce = regexp.MustCompile(`\brm\b.*(-[a-z]*r[a-z]*f|-[a-z]*f[a-z]*r|-r[a-z]* .*-f|-f[a-z]* .*-r)`)
	// A delete target that IS the filesystem root or home dir (not a subpath).
	rootTarget = regexp.MustCompile(`(^| )(/|/\*|~|~/|\$home|\$home/)( |$)`)
	// dd of=/dev/... or a redirect to a raw disk device node.
	rawDeviceWrite = regexp.MustCompile(`(of=/dev/|> ?/dev/(sd|disk|rdisk|nvme|hd|vd))`)
)
