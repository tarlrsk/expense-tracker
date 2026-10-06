package domain

import "strconv"

// Money is a total in hundredths of the currency unit (satang for THB), carried exactly, never as
// a float. Unlike a transaction's amount it may be zero or negative (a difference).
type Money int64

// String writes the total with exactly two decimals and a leading minus when negative, such as
// "0.00", "1450.00" or "-12.50".
func (m Money) String() string {
	// Division truncates toward zero, so both parts carry the sign; negating them is exact for
	// every int64, the smallest one included (only the whole value itself could overflow).
	units, cents := int64(m)/100, int64(m)%100
	sign := ""
	if m < 0 {
		units, cents, sign = -units, -cents, "-"
	}
	s := sign + strconv.FormatInt(units, 10) + "."
	if cents < 10 {
		s += "0"
	}
	return s + strconv.FormatInt(cents, 10)
}
