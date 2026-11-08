package price

import "testing"

func TestACurrencyIsShownWithItsOwnSign(t *testing.T) {
	for _, tc := range []struct {
		code string
		want string
		kept Currency
	}{
		{"", "$0.20", ""},
		{"usd", "$0.20", ""},
		{"EUR", "€0.20", EUR},
		{" eur ", "€0.20", EUR},
		{"CHF", "CHF 0.20", "CHF"},
	} {
		c := Parse(tc.code)
		if got := c.Format(0.2); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.code, got, tc.want)
		}
		if c.Kept() != tc.kept {
			t.Errorf("%q kept as %q, want %q", tc.code, c.Kept(), tc.kept)
		}
	}
	if !Currency("").IsUSD() || EUR.IsUSD() {
		t.Fatal("blank is dollars, euros are not")
	}
}
