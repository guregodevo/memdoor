package shell

import (
	"path/filepath"
	"runtime"
	"strings"
)

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

// PromptPath is a path as the model is shown it: in the form the bash tool's
// shell takes. Under Git Bash on Windows that is the form Git Bash prints,
// /d/a/proj for D:\a\proj: the model copies the path it is shown into its
// commands, and bash reads a backslash as an escape. `> D:\a\_temp\proj\src\greet.txt`
// wrote a file named D:a_tempprojsrcgreet.txt in the working directory, and
// D:/a/proj became /D:/a/proj, another folder (#337). The file tools take the
// Git Bash form too (NativePath). PowerShell and cmd keep D:\a\proj, and off
// Windows a path is unchanged.
func PromptPath(p string) string {
	if runtime.GOOS != "windows" || (Name() != Bash && Name() != Sh) {
		return p
	}
	return gitBashPath(p)
}

// gitBashPath is D:\a\proj as Git Bash writes it, /d/a/proj. filepath.VolumeName
// is "" off Windows, so there the path only has its separators turned.
func gitBashPath(p string) string {
	vol := filepath.VolumeName(p)
	if len(vol) != 2 || vol[1] != ':' {
		return filepath.ToSlash(p)
	}
	return "/" + strings.ToLower(vol[:1]) + filepath.ToSlash(p[2:])
}

// PathHint is the line the model gets under Git Bash on Windows about the
// paths it writes; "" elsewhere.
func PathHint() string {
	if runtime.GOOS != "windows" || (Name() != Bash && Name() != Sh) {
		return ""
	}
	return "The bash tool's shell is Git Bash: write paths as it prints them, /d/a/proj/src/x.txt, with forward slashes. A backslash is an escape in bash: the file lands under another name."
}
