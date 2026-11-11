package shell

import "testing"

func TestAGitBashDrivePathIsTheDrivesPathOnWindows(t *testing.T) {
	for in, want := range map[string]string{
		"/i/Projects/app/x.cs": "I:/Projects/app/x.cs",
		"/c":                   "C:/",
		"/c/":                  "C:/",
		"/cygdrive/d/src/a.go": "D:/src/a.go",
		"/mnt/e/w":             "E:/w",
		"/usr/bin/env":         "/usr/bin/env",
		"/ab/x":                "/ab/x",
		"C:/already/native":    "C:/already/native",
		"relative/path.go":     "relative/path.go",
		"/1/x":                 "/1/x",
		"/cygdrive/":           "/cygdrive/",
	} {
		if got := nativePath("windows", in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if got := nativePath("darwin", "/i/Projects"); got != "/i/Projects" {
		t.Errorf("off Windows a path is unchanged: %q", got)
	}
}
