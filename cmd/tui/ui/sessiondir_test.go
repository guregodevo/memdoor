package ui

import "testing"

// On another machine the header names it and the directory there, with
// the home as ~: "cloudshell:~", "box:~/srv/app", never a bare base name
// that reads as a user name.
func TestTheHeaderNamesTheBoxAndItsDirectory(t *testing.T) {
	t.Cleanup(func() { SetSessionDir(""); SetSessionPlace("", "") })
	SetSessionDir("/home/me")
	SetSessionPlace("cloudshell", "/home/me")
	if got := dirBase(); got != "cloudshell:~" {
		t.Fatalf("home: %q", got)
	}
	SetSessionDir("/home/me/srv/app")
	if got := dirBase(); got != "cloudshell:~/srv/app" {
		t.Fatalf("under home: %q", got)
	}
	SetSessionDir("/srv/app")
	if got := dirBase(); got != "cloudshell:/srv/app" {
		t.Fatalf("outside home: %q", got)
	}
}
