# Windows check (temporary branch): the window in a real Windows pseudo-terminal
# (ConPTY via pywinpty, rendered with pyte), and a coder turn on a machine with
# no Git Bash. Prints a report; exits 1 when a check fails.
import os, subprocess, sys, threading, time, urllib.request
import pyte
from winpty import PtyProcess

EXE = os.path.abspath(sys.argv[1])
ROOT = os.path.abspath(sys.argv[2])
KEY = os.environ["OPEN_ROUTER_API_KEY"]
results = []

def check(name, ok, detail=""):
    results.append((name, ok))
    print(("PASS " if ok else "FAIL ") + name + (("  — " + detail) if detail else ""), flush=True)

def wait_status(port, secs=120):
    for _ in range(secs):
        try:
            urllib.request.urlopen(f"http://localhost:{port}/api/status", timeout=2); return True
        except Exception:
            time.sleep(1)
    return False

def gateway(name, port, extra_env=None):
    home = os.path.join(ROOT, name, "home"); work = os.path.join(ROOT, name, "work")
    os.makedirs(home, exist_ok=True); os.makedirs(work, exist_ok=True)
    subprocess.run(["git", "init", "-q"], cwd=work)
    env = dict(os.environ, HOME=home, USERPROFILE=home, MEMDOOR_NO_BROWSER="1", OPEN_ROUTER_API_KEY=KEY)
    env.update(extra_env or {})
    log = open(os.path.join(ROOT, name, "gw.log"), "w")
    p = subprocess.Popen([EXE, "gateway", "--port", str(port)], cwd=os.path.join(ROOT, name), env=env, stdout=log, stderr=subprocess.STDOUT)
    ok = wait_status(port)
    check(f"{name}: the gateway answers on :{port}", ok)
    r = subprocess.run([EXE, "--gateway", f"http://localhost:{port}", "setup", "--admin-email", "t@example.com",
                        "--admin-password", "win-check-pass-1", "--workspace-name", name], env=env, capture_output=True, text=True, timeout=120)
    check(f"{name}: setup", r.returncode == 0, (r.stdout + r.stderr).strip().splitlines()[-1] if (r.stdout + r.stderr).strip() else "")
    return p, env, work

class Screen:
    def __init__(self, argv, cwd, env):
        self.screen = pyte.Screen(150, 46); self.stream = pyte.Stream(self.screen)
        self.lock = threading.Lock()
        self.p = PtyProcess.spawn(argv, cwd=cwd, env=env, dimensions=(46, 150))
        threading.Thread(target=self._read, daemon=True).start()
    def _read(self):
        while True:
            try:
                data = self.p.read(4096)
            except Exception:
                return
            if not data:
                time.sleep(0.05); continue
            with self.lock:
                self.stream.feed(data)
    def text(self):
        with self.lock:
            return "\n".join(line.rstrip() for line in self.screen.display)
    def send(self, s):
        self.p.write(s)
    def type(self, s):
        for ch in s:
            self.p.write(ch); time.sleep(0.01)
    def wait_for(self, needle, secs):
        end = time.time() + secs
        while time.time() < end:
            if needle in self.text(): return True
            time.sleep(0.5)
        return False
    def wait_turn(self, secs=300):
        time.sleep(4)
        end = time.time() + secs; quiet = 0
        while time.time() < end and quiet < 4:
            quiet = 0 if "esc to interrupt" in self.text() else quiet + 1
            time.sleep(1.5)
        return quiet >= 4
    def dump(self, title):
        print(f"----- screen: {title}\n" + "\n".join(l for l in self.text().splitlines() if l.strip()) + "\n-----", flush=True)

# 1. The window on Windows.
gw, env, work = gateway("tui", 18931)
url = "http://localhost:18931"
tui = Screen([EXE, "--gateway", url, "tui"], work, dict(env, TERM="xterm-256color"))
check("tui: the window draws its prompt and footer", tui.wait_for("/ commands", 60))
tui.dump("first screen")

tui.type("Create hello.txt containing the single line hi, with the apply_patch tool, then reply with the word done.")
tui.send("\r")
check("tui: a turn runs and ends", tui.wait_turn(300))
tui.dump("after the turn")
p = os.path.join(work, "hello.txt")
check("tui: the coder wrote hello.txt in the project", os.path.exists(p) and open(p).read().strip() == "hi",
      repr(open(p).read()) if os.path.exists(p) else "missing")

tui.send("/")
time.sleep(2)
tui.dump("slash menu")
check("tui: '/' opens the command menu", "/model" in tui.text() and "/help" in tui.text())
tui.send("\x1b"); time.sleep(1)
# clear the input line
for _ in range(3): tui.send("\x7f")

tui.type("Run exactly this one command with the bash tool and nothing else: sleep 120")
tui.send("\r")
started = tui.wait_for("esc to interrupt", 90)
check("tui: a long command shows it is running", started)
time.sleep(6)
tui.send("\x1b")
stopped = tui.wait_for("Interrupted", 30)
tui.dump("after esc")
check("tui: Esc interrupts the running turn", stopped and "esc to interrupt" not in tui.text())

tui.send("\x03"); time.sleep(2); tui.send("\x03"); time.sleep(3)
check("tui: ctrl+c quits the window", not tui.p.isalive())
gw.terminate()

# 2. A machine with no Git Bash: the coder's commands go to PowerShell.
sysroot = os.environ.get("SystemRoot", r"C:\Windows")
nogit = os.path.join(ROOT, "nogit-programfiles"); os.makedirs(nogit, exist_ok=True)
psenv = {
    "PATH": ";".join([os.path.join(sysroot, "System32"), sysroot, os.path.join(sysroot, r"System32\WindowsPowerShell\v1.0"), os.path.dirname(EXE)]),
    "ProgramFiles": nogit, "ProgramFiles(x86)": nogit, "ProgramW6432": nogit, "LOCALAPPDATA": nogit,
}
r = subprocess.run(["where", "bash"], env=dict(os.environ, **psenv), capture_output=True, text=True)
print("where bash (no-Git env):", (r.stdout or r.stderr).strip(), flush=True)
gw2, env2, work2 = gateway("pwsh", 18932, psenv)
r = subprocess.run([EXE, "--gateway", "http://localhost:18932", "run",
                    "Use the bash tool (not apply_patch) to run a shell command that creates notes.txt containing the line hello, "
                    "then run a shell command that lists the folder. Report both commands exactly as you ran them."],
                   cwd=work2, env=env2, capture_output=True, text=True, timeout=400)
print("----- memdoor run (no Git Bash)\n" + (r.stdout + r.stderr)[-3000:] + "\n-----", flush=True)
n = os.path.join(work2, "notes.txt")
check("pwsh: the coder created notes.txt with a PowerShell command", os.path.exists(n) and "hello" in open(n, encoding="utf-8", errors="replace").read(),
      repr(open(n, "rb").read()[:60]) if os.path.exists(n) else "missing")
gw2.terminate()

failed = [n for n, ok in results if not ok]
print(f"\n{len(results) - len(failed)}/{len(results)} checks passed" + (": failed " + "; ".join(failed) if failed else ""))
sys.exit(1 if failed else 0)
