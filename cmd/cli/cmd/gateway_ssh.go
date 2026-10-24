package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The ssh door (Greg, 2026-10-23: "we can do ssh too"). --gateway
// ssh://[user@]box[:port] (or MEMDOOR_GATEWAY) reaches the gateway on a box
// through ssh: a tunnel is opened to the gateway's port on that box and this
// command talks through it, the window, a run, a workflow, resume alike.
// Nothing on the box is opened to the network, no --host 0.0.0.0, no TLS to
// arrange: the keys ssh already has. ?port=18799 names a gateway on another
// port there.
//
// The tunnel is `ssh -T -L … box cat`: cat on the box reads this process's
// pipe, so the tunnel lives exactly as long as the command and never
// outlives it. BatchMode means a box that would ask for a password fails at
// once with ssh's own reason, instead of a prompt under the window.
type sshDoor struct {
	name   string // the address as typed: the credentials' key and the name in messages
	local  string // http://127.0.0.1:<port> this command talks to
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	exited <-chan error // ssh's exit, waited for once
}

var sshDoorUp *sshDoor

// sshDoorWait is how long the gateway on the box has to answer through the
// tunnel before the door is given up.
const sshDoorWait = 20 * time.Second

// sshFirstLook is how long a gateway on the box has to answer through the
// tunnel before the door looks at the box itself (sshBoxSetup).
var sshFirstLook = 3 * time.Second

const (
	sshScheme             = "ssh://"
	sshDefaultGatewayPort = "18789"
)

func isSSHAddr(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), sshScheme)
}

// sshDoorSpec reads ssh://[user@]host[:sshport][?port=gatewayport].
func sshDoorSpec(addr string) (target, sshPort, gatewayPort string, err error) {
	u, err := url.Parse(strings.TrimSpace(addr))
	if err != nil || u.Hostname() == "" {
		return "", "", "", fmt.Errorf("%q is not an ssh address: ssh://[user@]box[:port]", addr)
	}
	target = u.Hostname()
	if u.User != nil && u.User.Username() != "" {
		target = u.User.Username() + "@" + target
	}
	gatewayPort = sshDefaultGatewayPort
	if p := strings.TrimSpace(u.Query().Get("port")); p != "" {
		if _, err := strconv.Atoi(p); err != nil {
			return "", "", "", fmt.Errorf("%q: ?port= must be the gateway's port number on the box", addr)
		}
		gatewayPort = p
	}
	return target, u.Port(), gatewayPort, nil
}

// sshDoorArgs is the ssh command line for one tunnel.
func sshDoorArgs(target, sshPort, localPort, gatewayPort string) []string {
	args := []string{"-T", "-o", "ExitOnForwardFailure=yes", "-o", "BatchMode=yes", "-o", "ServerAliveInterval=30",
		"-L", "127.0.0.1:" + localPort + ":127.0.0.1:" + gatewayPort}
	if sshPort != "" {
		args = append(args, "-p", sshPort)
	}
	return append(args, target, "cat")
}

// openSSHDoor opens the tunnel and answers the local address to talk to.
// Opened once per process; a second call answers the same address.
func openSSHDoor(addr string) (string, error) {
	if sshDoorUp != nil {
		return sshDoorUp.local, nil
	}
	target, sshPort, gatewayPort, err := sshDoorSpec(addr)
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("no local port for the ssh tunnel: %v", err)
	}
	localPort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cmd := exec.Command("ssh", sshDoorArgs(target, sshPort, strconv.Itoa(localPort), gatewayPort)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("ssh is not on this machine (%v): the ssh door needs it", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	giveUp := func(err error) (string, error) {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		<-exited // ssh is done writing its stderr once it has exited
		return "", err
	}
	deadline := time.Now().Add(sshDoorWait)
	firstLook, looked := time.Now().Add(sshFirstLook), false
	for {
		select {
		case <-exited:
			return "", fmt.Errorf("ssh to %s: %s", target, sshReason(target, stderr.String()))
		default:
		}
		if ok, _ := probeGatewayHealth("127.0.0.1", localPort); ok {
			break
		}
		if !looked && time.Now().After(firstLook) {
			looked = true
			if err := sshBoxSetup(target, sshPort, gatewayPort); err != nil {
				return giveUp(err)
			}
			deadline = time.Now().Add(sshDoorWait)
		}
		if time.Now().After(deadline) {
			why := strings.TrimSpace(lastLine(stderr.String()))
			if why != "" {
				why = " (" + why + ")"
			}
			return giveUp(fmt.Errorf("ssh reached %s but no gateway answered on its port %s within %s%s: on the box, memdoor gateway status", target, gatewayPort, sshDoorWait, why))
		}
		time.Sleep(200 * time.Millisecond)
	}
	sshDoorUp = &sshDoor{name: strings.TrimSpace(addr), local: "http://127.0.0.1:" + strconv.Itoa(localPort), stdin: stdin, cmd: cmd, exited: exited}
	return sshDoorUp.local, nil
}

// closeSSHDoor ends the tunnel: the pipe closes, cat on the box ends, ssh
// exits. The process's own exit does the same without being asked.
func closeSSHDoor() {
	if sshDoorUp == nil {
		return
	}
	_ = sshDoorUp.stdin.Close()
	select {
	case <-sshDoorUp.exited:
	case <-time.After(2 * time.Second):
		_ = sshDoorUp.cmd.Process.Kill()
		<-sshDoorUp.exited
	}
	sshDoorUp = nil
}

// sshReason is ssh's own last word on a failed sign-in, with what to do
// when it asked for a password this door cannot give.
func sshReason(target, stderr string) string {
	why := strings.TrimSpace(lastLine(stderr))
	if why == "" {
		why = "ssh exited before the tunnel was up"
	}
	switch {
	case strings.Contains(why, "Permission denied"):
		why += "; the ssh door signs in with a key, never a prompt: ssh-copy-id " + target
	case strings.Contains(why, "Host key verification failed"):
		why += "; the ssh door never answers a host-key question: ssh " + target + " once by hand first"
	}
	return why
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// sshRun runs one command on the box, with the keys ssh has and no prompt,
// and answers its stdout; an error carries the command's or ssh's last line.
func sshRun(target, sshPort, command string) (string, error) {
	args := []string{"-T", "-o", "BatchMode=yes"}
	if sshPort != "" {
		args = append(args, "-p", sshPort)
	}
	args = append(args, target, command)
	var stdout, stderr bytes.Buffer
	c := exec.Command("ssh", args...)
	c.Stdout, c.Stderr = &stdout, &stderr
	if err := c.Run(); err != nil {
		why := strings.TrimSpace(lastLine(stderr.String()))
		if why == "" {
			why = strings.TrimSpace(lastLine(stdout.String()))
		}
		if why == "" {
			why = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s", why)
	}
	return stdout.String(), nil
}

// sshFindMemdoor is the shell line that names memdoor on the box: on its
// PATH, else where the installer puts it.
const sshFindMemdoor = `command -v memdoor 2>/dev/null || { [ -x "$HOME/.local/bin/memdoor" ] && echo "$HOME/.local/bin/memdoor"; }`

// sshBoxSetup makes a box answer the door (Greg, 2026-10-24: "it should be
// accessible from http and ssh like github", "add memdoor --gateway
// ssh://you@box run"). When no gateway answers through the tunnel, the box
// is looked at over the same ssh: no memdoor there, and memdoor.ai's
// installer puts it in ~/.local/bin, as on a laptop; its gateway not
// running, and `memdoor gateway install --port <p>` makes it a service
// there (systemd --user, launchd), or, where there is no service manager,
// starts it in the background. Each step says what it does on stderr. A
// gateway that runs and is merely slow is left alone: its own `gateway
// status` decides. The box has no model until a key is connected there:
// `memdoor --gateway ssh://you@box connect <provider>`.
func sshBoxSetup(target, sshPort, gatewayPort string) error {
	say := func(format string, a ...any) { fmt.Fprintf(os.Stderr, "  "+format+"\n", a...) }
	find := func() string {
		out, _ := sshRun(target, sshPort, sshFindMemdoor)
		return strings.TrimSpace(lastLine(out))
	}
	bin := find()
	if bin == "" {
		say("%s has no memdoor: installing it there from %s (~/.local/bin, no sudo)", target, installScriptURL)
		// MEMDOOR_PORT: the installer's upgrade-in-place block looks for a
		// running gateway on that port and restarts it on the new binary;
		// pointed at the door's port it touches only the gateway this door
		// is for (2026-10-24: run against a scratch box on the developer's
		// own machine, it found the developer's gateway on 18789 and
		// restarted it under the scratch home).
		if _, err := sshRun(target, sshPort, "curl -fsSL "+installScriptURL+" | MEMDOOR_PORT="+gatewayPort+" bash"); err != nil {
			return fmt.Errorf("installing memdoor on %s: %v", target, err)
		}
		if bin = find(); bin == "" {
			return fmt.Errorf("the installer ran on %s but left no memdoor on its PATH or in ~/.local/bin", target)
		}
	}
	status := bin + " --gateway http://127.0.0.1:" + gatewayPort + " gateway status >/dev/null 2>&1"
	if _, err := sshRun(target, sshPort, status); err == nil {
		return nil
	}
	say("the gateway on %s is not running: installing it as a service there, on port %s", target, gatewayPort)
	out, err := sshRun(target, sshPort, bin+" gateway install --port "+gatewayPort)
	if err != nil {
		say("no service manager on %s (%v): starting the gateway in the background there instead", target, err)
		start := "mkdir -p ~/.memdoor && nohup " + bin + " gateway --port " + gatewayPort + " >> ~/.memdoor/gateway.log 2>&1 </dev/null &"
		if _, err := sshRun(target, sshPort, start); err != nil {
			return fmt.Errorf("starting the gateway on %s: %v", target, err)
		}
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "enable-linger") {
			say("on %s, %s", target, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "→")))
		}
	}
	return nil
}

// sshRenewSession signs in to the gateway on the box with the login that
// box generated for itself (~/.memdoor/local-login there), read over the
// same ssh: whoever may ssh in as that user may read it, so the door asks
// for no password of its own. The session is saved under the ssh address.
func sshRenewSession() error {
	if sshDoorUp == nil {
		return fmt.Errorf("no ssh door")
	}
	target, sshPort, gwPort, err := sshDoorSpec(sshDoorUp.name)
	if err != nil {
		return err
	}
	out, err := sshRun(target, sshPort, "cat ~/.memdoor/"+localLoginFile)
	if err != nil {
		return fmt.Errorf("no generated login on %s: %v", target, err)
	}
	var ll localLogin
	if json.Unmarshal([]byte(out), &ll) != nil || ll.Password == "" {
		return fmt.Errorf("the generated login on %s is unreadable", target)
	}
	if err := loginLocal(ll.Email, ll.Password); err != nil {
		return err
	}
	// One sign-in serves both doors onto the box: the session the ssh door
	// just made is kept for the box's own address too, so `--gateway box`
	// over its network needs no password the person never saw.
	if creds, err := loadCredentials(); err == nil {
		_ = saveCredentialsFor("http://"+net.JoinHostPort(sshDoorHost(), gwPort), creds)
	}
	return nil
}

// sshDoorHint is the ssh address of the gateway --gateway names over http,
// for a line a person retypes: ssh://you@box, ?port= when the gateway is
// not on 18789 there.
func sshDoorHint() string {
	u, err := url.Parse(gatewayAddr)
	if err != nil || u.Hostname() == "" {
		return "ssh://you@" + gatewayAddr
	}
	hint := "ssh://you@" + u.Hostname()
	if p := u.Port(); p != "" && p != sshDefaultGatewayPort {
		hint += "?port=" + p
	}
	return hint
}

// sshDoorHost is the box's name as typed, without the user: what a phone
// on the same network or VPN dials.
func sshDoorHost() string {
	if sshDoorUp == nil {
		return ""
	}
	if u, err := url.Parse(sshDoorUp.name); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return sshDoorUp.name
}

// gatewayKey names the gateway in the credentials file: the address as
// typed for an ssh door (its tunnel port changes every run), else the
// address itself.
func gatewayKey() string {
	if sshDoorUp != nil {
		return sshDoorUp.name
	}
	return gatewayAddr
}

// gatewayShown is the gateway's address in a line a person reads or retypes.
func gatewayShown() string {
	if sshDoorUp != nil {
		return sshDoorUp.name
	}
	return gatewayAddr
}

// envOr is the variable's value, or def when it is unset or blank.
func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}
