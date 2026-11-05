package gateway

import (
	"encoding/json"
	"os"
	"strings"

	"memdoor/pkg/auth"
	"memdoor/pkg/shared"
)

// The machine's own user is the one ~/.memdoor/local-login names: the first
// run generated it, and its sessions do not run out (#88, pkg/auth).
func init() { auth.MachineUser = machineUserEmail }

// localLoginFile is the generated login, written by the CLI at the first run.
const localLoginFile = "local-login"

// machineUserEmail is the email in ~/.memdoor/local-login, "" when there is
// no such file (a gateway whose users were all made by hand).
func machineUserEmail() string {
	b, err := os.ReadFile(shared.MemdoorHome(localLoginFile))
	if err != nil {
		return ""
	}
	var ll struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(b, &ll) != nil {
		return ""
	}
	return strings.TrimSpace(ll.Email)
}
