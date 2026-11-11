package shell

import "runtime"

// NativePath turns a drive path spelled the POSIX-shell way into the Windows
// form, on Windows: /i/x, /cygdrive/i/x and /mnt/i/x become I:/x. On Windows
// the bash tool is Git Bash, so pwd, ls and find print /i/Projects/app, and
// the model copies that into its file tools. Windows does not take it as
// absolute, so it was joined under the workdir (I:\Projects\app\i\Projects\…)
// and the call failed again and again. Any other path, and every path off
// Windows, comes back unchanged.
func NativePath(p string) string { return nativePath(runtime.GOOS, p) }

func nativePath(goos, p string) string {
	if goos != "windows" {
		return p
	}
	rest := p
	for _, prefix := range []string{"/cygdrive/", "/mnt/"} {
		if len(rest) > len(prefix) && rest[:len(prefix)] == prefix {
			rest = rest[len(prefix)-1:]
			break
		}
	}
	// /x or /x/... with a single letter is a drive.
	if len(rest) < 2 || rest[0] != '/' || !isLetter(rest[1]) || (len(rest) > 2 && rest[2] != '/') {
		return p
	}
	drive := rest[1]
	if drive >= 'a' {
		drive -= 'a' - 'A'
	}
	if len(rest) == 2 {
		return string(drive) + ":/"
	}
	return string(drive) + ":" + rest[2:]
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
