package cmd

import (
	"fmt"
	"net"
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
            try:
                r = socket.create_connection((rh, int(rp)))
            except OSError:
                c.close(); continue  # as ssh: a forward nobody answers closes that one connection
            threading.Thread(target=pipe, args=(c, r), daemon=True).start()
            threading.Thread(target=pipe, args=(r, c), daemon=True).start()
    threading.Thread(target=serve, daemon=True).start()
    sys.stdin.buffer.read()
    sys.exit(0)
box = os.environ.get("FAKE_SSH_BOX")
if box:
    cmd = args[-1]
    with open(os.path.join(box, "commands.log"), "a") as f: f.write(cmd + "\n")
    installed, running = os.path.join(box, "installed"), os.path.join(box, "running")
    if "command -v memdoor" in cmd:
        if os.path.exists(installed): print("/home/me/.local/bin/memdoor"); sys.exit(0)
        sys.exit(1)
    if "install.sh" in cmd:
        open(installed, "w").close(); print("==> installed"); sys.exit(0)
    if "gateway status" in cmd:
        sys.exit(0 if os.path.exists(running) else 1)
    if "gateway install" in cmd:
        if os.environ.get("FAKE_SSH_NO_SERVICE"):
            sys.stderr.write("systemctl --user daemon-reload: Failed to connect to bus\n"); sys.exit(1)
        open(running, "w").close(); print("  → to keep it running when you are not logged in:  loginctl enable-linger me"); sys.exit(0)
    if "nohup" in cmd:
        open(running, "w").close(); sys.exit(0)
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
	if err == nil || !strings.Contains(err.Error(), "login-direct --gateway http://box.lan:18799") || !strings.Contains(err.Error(), "ssh://you@box.lan?port=18799") || strings.Contains(err.Error(), "not set up yet") {
		t.Fatalf("a remote gateway without a session names the sign-in there: %v", err)
	}
}

// boxOnFakeSSH is a box the fake ssh plays: what memdoor is there is a
// marker file; its gateway answers on port only once "running" exists.
func boxOnFakeSSH(t *testing.T) (dir string, port int, commands func() string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("FAKE_SSH_BOX", dir)
	saved := sshFirstLook
	sshFirstLook = 200 * time.Millisecond
	t.Cleanup(func() { sshFirstLook = saved })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := os.Stat(filepath.Join(dir, "running")); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if _, err := os.Stat(filepath.Join(dir, "slow")); err == nil {
			time.Sleep(600 * time.Millisecond) // running, and slow to answer
		}
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","version":"test"}`))
		})}
		go func() { <-stop; _ = srv.Close() }()
		_ = srv.Serve(l)
	}()
	commands = func() string { b, _ := os.ReadFile(filepath.Join(dir, "commands.log")); return string(b) }
	return dir, port, commands
}

// A box with nothing on it answers the door: no memdoor there, the
// installer puts it; its gateway not running, it is installed as a service
// on the door's port; then the tunnel sees it (Greg, 2026-10-24: "accessible
// from http and ssh like github").
func TestTheSSHDoorSetsUpABoxWithNothingOnIt(t *testing.T) {
	withFakeSSH(t)
	_, port, commands := boxOnFakeSSH(t)
	local, err := openSSHDoor(fmt.Sprintf("ssh://me@box?port=%d", port))
	if err != nil {
		t.Fatalf("open: %v\n%s", err, commands())
	}
	if resp, err := http.Get(local + "/health"); err != nil || resp.StatusCode != 200 {
		t.Fatalf("the box answers through the tunnel: %v %v", resp, err)
	}
	log := commands()
	for _, step := range []string{"command -v memdoor", fmt.Sprintf("install.sh | MEMDOOR_PORT=%d bash", port), "gateway status", fmt.Sprintf("gateway install --port %d", port)} {
		if !strings.Contains(log, step) {
			t.Errorf("the door did not run %q on the box:\n%s", step, log)
		}
	}
	if strings.Index(log, "install.sh") > strings.Index(log, "gateway install") {
		t.Errorf("the installer runs before the service:\n%s", log)
	}
}

// A box that has memdoor but no running gateway and no service manager (a
// container) gets the gateway started in the background instead.
func TestTheSSHDoorStartsTheGatewayWhereThereIsNoServiceManager(t *testing.T) {
	withFakeSSH(t)
	dir, port, commands := boxOnFakeSSH(t)
	if err := os.WriteFile(filepath.Join(dir, "installed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_SSH_NO_SERVICE", "1")
	if _, err := openSSHDoor(fmt.Sprintf("ssh://me@box?port=%d", port)); err != nil {
		t.Fatalf("open: %v\n%s", err, commands())
	}
	log := commands()
	if strings.Contains(log, "install.sh") {
		t.Errorf("memdoor was there, nothing to install:\n%s", log)
	}
	if !strings.Contains(log, "nohup /home/me/.local/bin/memdoor gateway --port") {
		t.Errorf("the gateway is started in the background:\n%s", log)
	}
}

// A gateway that runs and is merely slow to answer is left alone: its own
// status decides, and nothing is installed or restarted.
func TestTheSSHDoorLeavesASlowRunningGatewayAlone(t *testing.T) {
	withFakeSSH(t)
	dir, port, commands := boxOnFakeSSH(t)
	for _, marker := range []string{"installed", "slow", "running"} {
		if err := os.WriteFile(filepath.Join(dir, marker), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := openSSHDoor(fmt.Sprintf("ssh://me@box?port=%d", port)); err != nil {
		t.Fatalf("open: %v\n%s", err, commands())
	}
	if log := commands(); strings.Contains(log, "gateway install") || strings.Contains(log, "nohup") || strings.Contains(log, "install.sh") {
		t.Errorf("a running gateway was touched:\n%s", log)
	}
}
