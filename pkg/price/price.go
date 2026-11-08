// Package price is the currency a model's list price is stated in. Most
// providers state dollars; EUrouter states some models in euros, and a euro
// figure shown with a dollar sign, or compared with a dollar one, is wrong.
package price

import (
	"fmt"
	"strings"
)

// Currency is an ISO 4217 code. The zero value is dollars, so a price with
// no currency stated reads as it always has.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
)

// Parse is the currency a provider names, upper-cased; blank is dollars.
func Parse(code string) Currency {
	c := Currency(strings.ToUpper(strings.TrimSpace(code)))
	if c == "" {
		return USD
	}
	return c
}

// IsUSD says the price is in dollars, the zero value included.
func (c Currency) IsUSD() bool { return c == "" || c == USD }

// Kept is the currency as a model carries it: blank for dollars, so a
// dollar price serialises as it did before currencies were kept.
func (c Currency) Kept() Currency {
	if c.IsUSD() {
		return ""
	}
	return c
}

// Symbol is what goes before the figure: $, €, else the code and a space.
func (c Currency) Symbol() string {
	switch {
	case c.IsUSD():
		return "$"
	case c == EUR:
		return "€"
	}
	return string(c) + " "
}

// Format is a price per million tokens as it is shown: "$0.20", "€0.09".
func (c Currency) Format(perMillion float64) string {
	return fmt.Sprintf("%s%.2f", c.Symbol(), perMillion)
}
