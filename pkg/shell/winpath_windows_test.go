//go:build windows

package shell

import "testing"

// #337: the model is shown the working directory as Git Bash writes it.
func TestAWindowsPathIsShownAsGitBashWritesIt(t *testing.T) {
	for in, want := range map[string]string{
		`D:\a\_temp\e2e\proj`: "/d/a/_temp/e2e/proj",
		`C:\`:                 "/c/",
		`C:/Users/me`:         "/c/Users/me",
		`\\host\share\x`:      "//host/share/x",
	} {
		if got := gitBashPath(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	// The way back: what the model writes from it is the same folder to the file tools.
	if got := NativePath(gitBashPath(`D:\a\proj`)); got != "D:/a/proj" {
		t.Errorf("round trip: %q", got)
	}
}
