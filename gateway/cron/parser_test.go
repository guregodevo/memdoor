package cron

import (
	"testing"
	"time"
)

// cronParser must accept standard 5-field crontab as well as the 6-field
// seconds form (and named descriptors), and reject garbage. Requiring 6 fields
// silently stored-but-never-scheduled a valid-looking 5-field schedule.
func TestCronParserAcceptsFiveAndSixFields(t *testing.T) {
	valid := []string{
		"0 * * * *",      // 5-field: top of every hour
		"*/15 * * * *",   // 5-field: every 15 minutes
		"0 9 * * 1-5",    // 5-field: 9am on weekdays
		"0 * * * * *",    // 6-field: every minute at second 0
		"*/30 * * * * *", // 6-field: every 30 seconds
		"@hourly",        // named descriptor (kept from WithSeconds)
		"@every 1h30m",   // interval descriptor
	}
	for _, expr := range valid {
		if _, err := cronParser.Parse(expr); err != nil {
			t.Errorf("cronParser rejected valid schedule %q: %v", expr, err)
		}
	}

	invalid := []string{
		"",               // empty
		"0 9 * *",        // 4 fields
		"0 * * * * * *",  // 7 fields
		"not a schedule", // words
		"99 * * * *",     // minute out of range
	}
	for _, expr := range invalid {
		if _, err := cronParser.Parse(expr); err == nil {
			t.Errorf("cronParser accepted invalid schedule %q", expr)
		}
	}
}

// ValidateSchedule is the boundary check the create handler uses to reject an
// unschedulable job before persisting it. It must accept what the scheduler
// accepts and error on what it can't run.
func TestValidateSchedule(t *testing.T) {
	for _, ok := range []string{"0 * * * *", "*/30 * * * * *", "@hourly"} {
		if err := ValidateSchedule(ok); err != nil {
			t.Errorf("ValidateSchedule(%q) errored: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "nonsense", "0 9 * *", "99 * * * *"} {
		if err := ValidateSchedule(bad); err == nil {
			t.Errorf("ValidateSchedule(%q) accepted an unschedulable expression", bad)
		}
	}
}

// A 5-field expression must mean what crontab means — minute-first, seconds 0 —
// NOT be misread as a seconds field. "0 * * * *" is the top of every hour, so
// its next two firings are exactly one hour apart, not one minute.
func TestFiveFieldScheduleIsMinuteFirst(t *testing.T) {
	sched, err := cronParser.Parse("0 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	first := sched.Next(from)
	second := sched.Next(first)

	if first.Minute() != 0 || first.Second() != 0 {
		t.Errorf("first firing = %v, want minute 0 second 0", first)
	}
	if gap := second.Sub(first); gap != time.Hour {
		t.Errorf("gap between firings = %v, want 1h (hourly, not per-minute or per-second)", gap)
	}
}
