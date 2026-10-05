package gateway

import "testing"

func TestDestructiveBashReason(t *testing.T) {
	blocked := []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -rf ~",
		"rm -rf $HOME",
		"rm -fr /",
		"rm -r -f /",
		"sudo rm -rf --no-preserve-root /",
		"rm --no-preserve-root -rf /",
		"mkfs.ext4 /dev/sda1",
		"dd if=/dev/zero of=/dev/sda bs=1M",
		"echo boom > /dev/sda",
		":(){ :|:& };:",
	}
	for _, cmd := range blocked {
		if _, isBad := destructiveBashReason(cmd); !isBad {
			t.Errorf("should BLOCK: %q", cmd)
		}
	}

	allowed := []string{
		"rm -rf /tmp/build",
		"rm -rf ./node_modules",
		"rm -rf build/ dist/",
		"rm -f main.go",
		"go build ./...",
		"go test ./...",
		"ls -la /",
		"cat /etc/hosts",
		"grep -rf pattern .", // -rf here is grep flags, not rm; no root target
		"git rm -r internal/old",
		"dd if=input of=output.img", // not a /dev/ target
	}
	for _, cmd := range allowed {
		if reason, isBad := destructiveBashReason(cmd); isBad {
			t.Errorf("should ALLOW: %q (blocked as %q)", cmd, reason)
		}
	}
}
