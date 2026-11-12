package decision

import "testing"

// serialProvider is a local runtime: it takes one request at a time.
type serialProvider struct {
	fakeProvider
	parallel int
}

func (p *serialProvider) Parallel() int { return p.parallel }

// The service answers with what the provider it would pick says, and with
// DefaultParallel whenever it cannot know (issue #288).
func TestServiceParallel(t *testing.T) {
	local := &serialProvider{fakeProvider: fakeProvider{id: "local", ready: true}, parallel: 1}
	hosted := &fakeProvider{id: "hosted", ready: true}
	timed := &serialProvider{fakeProvider: fakeProvider{id: "timed", ready: true}, parallel: 0}
	svcFor := func(ref string, provs ...Provider) *service {
		s, err := NewService(func(string) string { return ref }, provs...)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		return s.(*service)
	}
	cases := []struct {
		name string
		svc  *service
		want int
	}{
		{"a local runtime serves one", svcFor("local", local), 1},
		{"a provider saying nothing serves the default", svcFor("hosted", hosted), DefaultParallel},
		{"a provider saying zero serves the default", svcFor("timed", timed), DefaultParallel},
		{"no model configured", svcFor("", hosted), DefaultParallel},
		{"decisions off", svcFor("off", hosted), DefaultParallel},
		{"unknown provider", svcFor("nope/x", hosted), DefaultParallel},
	}
	for _, c := range cases {
		if got := c.svc.Parallel(""); got != c.want {
			t.Fatalf("%s: Parallel() = %d, want %d", c.name, got, c.want)
		}
	}
}

// The service is what the judge asks, so it must answer through
// ServiceParallelism, not only through a concrete provider.
func TestServiceIsAServiceParallelism(t *testing.T) {
	svc, err := NewService(func(string) string { return "local" }, &serialProvider{fakeProvider: fakeProvider{id: "local", ready: true}, parallel: 1})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	p, ok := svc.(ServiceParallelism)
	if !ok || p.Parallel("") != 1 {
		t.Fatalf("service must answer ServiceParallelism with the provider's number: %v", ok)
	}
}
