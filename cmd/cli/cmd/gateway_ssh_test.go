package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeSSH is an ssh that forwards the -L port it is given and lives until
// its stdin ends, as the real one with `cat` on the box does; with
// FAKE_SSH_DENY set it refuses the way a box without our key does.
const fakeSSH = `#!/usr/bin/env python3
import os, socket, sys, threading
args = sys.argv[1:]
if os.environ.get("FAKE_SSH_DENY"):
    sys.stderr.write("me@box: Permission denied (publickey).\n"); sys.exit(255)
if args[-1] == "cat":
    spec = args[args.index("-L") + 1]
    lh, lp, rh, rp = spec.rsplit(":", 3)
    srv = socket.socket(); srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((lh, int(lp))); srv.listen(8)
    def pipe(a, b):
        try:
            while True:
                d = a.recv(65536)
                if not d: break
                b.sendall(d)
        except OSError: pass
        finally:
            try: b.shutdown(socket.SHUT_WR)
            except OSError: pass
    def serve():
        while True:
            c, _ = srv.accept()
            r = socket.create_connection((rh, int(rp)))
            threading.Thread(target=pipe, args=(c, r), daemon=True).start()
            threading.Thread(target=pipe, args=(r, c), daemon=True).start()
    threading.Thread(target=serve, daemon=True).start()
    sys.stdin.buffer.read()
    sys.exit(0)
sys.stdout.write(os.environ.get("FAKE_SSH_LOGIN", ""))
`

func withFakeSSH(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake ssh is a script")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3 for the fake ssh")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(fakeSSH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(closeSSHDoor)
}

// --gateway ssh://box opens a tunnel to the gateway on that box and the
// command talks through it: the address answered is local, the gateway is
// not, the credentials are kept under the ssh address, and the tunnel ends
// with the door.
func TestTheSSHDoorTunnelsToTheGatewayOnTheBox(t *testing.T) {
	withFakeSSH(t)
	box := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"test"}`))
	}))
	defer box.Close()
	port := strings.TrimPrefix(box.URL, "http://127.0.0.1:")
	addr := "ssh://me@box?port=" + port

	local, err := openSSHDoor(addr)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !strings.HasPrefix(local, "http://127.0.0.1:") {
		t.Fatalf("the door answers a local address, got %q", local)
	}
	resp, err := http.Get(local + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("the box answers through the tunnel: %v %v", resp, err)
	}
	resp.Body.Close()

	saved := gatewayAddr
	gatewayAddr = local
	t.Cleanup(func() { gatewayAddr = saved })
	if _, isLocal := localGatewayPort(); isLocal {
		t.Fatal("a gateway through the ssh door is on another machine")
	}
	if gatewayKey() != addr || gatewayShown() != addr || gatewayHostOnly() != addr {
		t.Fatalf("the ssh address names the gateway: key=%q shown=%q", gatewayKey(), gatewayShown())
	}
	if again, _ := openSSHDoor(addr); again != local {
		t.Fatalf("one door per process: %q then %q", local, again)
	}
	proc := sshDoorUp.cmd.Process
	closeSSHDoor()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := http.Get(local + "/health"); err != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := http.Get(local + "/health"); err == nil {
		t.Fatal("the tunnel outlived the door")
	}
	_ = proc
}

// A box that would ask for a password is refused at once, with ssh's own
// reason and the command that installs the key.
func TestTheSSHDoorSaysWhyItCouldNotSignIn(t *testing.T) {
	withFakeSSH(t)
	t.Setenv("FAKE_SSH_DENY", "1")
	_, err := openSSHDoor("ssh://me@box")
	if err == nil || !strings.Contains(err.Error(), "Permission denied") || !strings.Contains(err.Error(), "ssh-copy-id me@box") {
		t.Fatalf("the refusal names ssh's reason and the fix: %v", err)
	}
	if sshDoorUp != nil {
		t.Fatal("no door after a refusal")
	}
}

// The ssh address: user, host, ssh port and the gateway's port on the box.
func TestSSHDoorSpecAndArgs(t *testing.T) {
	target, sshPort, gw, err := sshDoorSpec("ssh://greg@box.lan:2222?port=18799")
	if err != nil || target != "greg@box.lan" || sshPort != "2222" || gw != "18799" {
		t.Fatalf("spec: %q %q %q %v", target, sshPort, gw, err)
	}
	if target, sshPort, gw, _ = sshDoorSpec("ssh://box"); target != "box" || sshPort != "" || gw != "18789" {
		t.Fatalf("defaults: %q %q %q", target, sshPort, gw)
	}
	if _, _, _, err := sshDoorSpec("ssh://box?port=x"); err == nil {
		t.Fatal("a port that is not a number is refused")
	}
	if _, _, _, err := sshDoorSpec("ssh://"); err == nil {
		t.Fatal("no host is refused")
	}
	args := strings.Join(sshDoorArgs("greg@box.lan", "2222", "40001", "18799"), " ")
	for _, want := range []string{"-T", "BatchMode=yes", "ExitOnForwardFailure=yes", "-L 127.0.0.1:40001:127.0.0.1:18799", "-p 2222", "greg@box.lan cat"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args %q lack %q", args, want)
		}
	}
	if !isSSHAddr(" SSH://box ") || isSSHAddr("http://box") {
		t.Fatal("isSSHAddr")
	}
	_ = url.Values{}
}

// A box whose host key ssh has never seen is refused with the step that
// records it; a gateway on another machine with no session says how to sign
// in there, not "set this machine up".
func TestTheSSHDoorAndARemoteGatewaySayTheNextStep(t *testing.T) {
	if got := sshReason("me@box", "Host key verification failed.\n"); !strings.Contains(got, "ssh me@box once by hand") {
		t.Fatalf("host key: %q", got)
	}
	if got := sshReason("me@box", ""); got != "ssh exited before the tunnel was up" {
		t.Fatalf("no stderr: %q", got)
	}
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	gatewayAddr = "http://box.lan:18799"
	t.Cleanup(func() { gatewayAddr = saved })
	err := sessionProblem()
	if err == nil || !strings.Contains(err.Error(), "login-direct --gateway http://box.lan:18799") || strings.Contains(err.Error(), "not set up yet") {
		t.Fatalf("a remote gateway without a session names the sign-in there: %v", err)
	}
}
