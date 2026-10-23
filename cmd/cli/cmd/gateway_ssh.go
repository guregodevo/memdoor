package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
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
	deadline := time.Now().Add(sshDoorWait)
	for {
		select {
		case <-exited:
			return "", fmt.Errorf("ssh to %s: %s", target, sshReason(target, stderr.String()))
		default:
		}
		if ok, _ := probeGatewayHealth("127.0.0.1", localPort); ok {
			break
		}
		if time.Now().After(deadline) {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			<-exited // ssh is done writing its stderr once it has exited
			why := strings.TrimSpace(lastLine(stderr.String()))
			if why != "" {
				why = " (" + why + ")"
			}
			return "", fmt.Errorf("ssh reached %s but no gateway answered on its port %s within %s%s: on the box, memdoor gateway status", target, gatewayPort, sshDoorWait, why)
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

// sshRenewSession signs in to the gateway on the box with the login that
// box generated for itself (~/.memdoor/local-login there), read over the
// same ssh: whoever may ssh in as that user may read it, so the door asks
// for no password of its own. The session is saved under the ssh address.
func sshRenewSession() error {
	if sshDoorUp == nil {
		return fmt.Errorf("no ssh door")
	}
	target, sshPort, _, err := sshDoorSpec(sshDoorUp.name)
	if err != nil {
		return err
	}
	args := []string{"-T", "-o", "BatchMode=yes"}
	if sshPort != "" {
		args = append(args, "-p", sshPort)
	}
	args = append(args, target, "cat ~/.memdoor/"+localLoginFile)
	out, err := exec.Command("ssh", args...).Output()
	if err != nil {
		return fmt.Errorf("no generated login on %s: %v", target, err)
	}
	var ll localLogin
	if json.Unmarshal(out, &ll) != nil || ll.Password == "" {
		return fmt.Errorf("the generated login on %s is unreadable", target)
	}
	return loginLocal(ll.Email, ll.Password)
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
