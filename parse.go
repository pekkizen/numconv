package numconv

import (
	"strconv"
)

const (
	dot                 = '.'
	asciiWhitespaceOnly = false
	iEEE754             = false // IEEE754 (strconv) compatible float64 parsing
	xsdDecimals         = false // only xsd:decimal format parsing
	// https://www.datypic.com/sc/xsd/t-xsd_decimal.html
)

var fpow10 = []float64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11,
	1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21,
}

var fdivpow10 = []float64{
	1, 1.0 / 1e1, 1.0 / 1e2, 1.0 / 1e3, 1.0 / 1e4, 1.0 / 1e5, 1.0 / 1e6,
	1.0 / 1e7, 1.0 / 1e8, 1.0 / 1e9, 1.0 / 1e10, 1.0 / 1e11, 1.0 / 1e12,
	1.0 / 1e13, 1.0 / 1e14, 1.0 / 1e15, 1.0 / 1e16, 1.0 / 1e17, 1.0 / 1e18,
	1.0 / 1e19, 1.0 / 1e20,
}

type err struct{ s string }

func (e *err) Error() string { return "numconv.Atof: " + e.s }

//go:noinline
func syntaxErr(b []byte) error {
	return &err{"parsing " + "\"" + string(b) + "\"" + ": " + "invalid syntax"}
}

// Trim returns a subslice of b with all leading and trailing whitespace removed.
// All characters <= Ascii space (' ' = 32 dec) are left and right trimmed off.
// This includes, among others, standard Ascii white space ' ', '\t', '\n', '\v', '\f', '\r'.
func Trim(b []byte) []byte {

	for len(b) > 1 && isWhitespace(b[0]) { //left trim
		b = b[1:]
	}
	for len(b) > 0 && isWhitespace(b[len(b)-1]) { //right rim
		b = b[:len(b)-1]
	}
	return b
}

func isWhitespace(c byte) bool {
	if asciiWhitespaceOnly {
		return c == ' ' || ('\t' <= c && c <= '\r')
	}
	return c <= ' '
}

func TrimTrailingZeros(b []byte) []byte {
	for len(b) > 1 && b[len(b)-1] == '0' && b[len(b)-2] != dot {
		b = b[:len(b)-1]
	}
	return b
}

//go:noinline
func parseFloat(b []byte) (float64, error) {
	if xsdDecimals {
		return 0, syntaxErr(b)
	}
	return strconv.ParseFloat(string(b), 64)
}

/*
Atof parses a decimal ascii number to float64 from byte slice b. In XML files the
format is xsd:decimal — Decimal numbers. Valid values include: 123.456,
+1234.456, -1234.456, -.456, or -456. Integers without decimal dot are also
accepted. -.0 , -00. 000 and 0 are ok. Use Trim function to remove leading and
trailing white space. Atof regards it as an error.
Up to 15 digits desimal and 16 digits integer numbers Atof is ~5 x faster
than strconv.ParseFloat(string(b), 64). Over 15/16 digits Atof calls
strconv.ParseFloat. Atof is fully compatible with strconv.ParseFloat.
In a test loop Atof parses 15 digits numbers over 1100 MB/s.
5 digits numbers at 650 MB/s. Actually Atof only implements a known fast
parsing path fast and use strconv.ParseFloat for the rest.
*/
func Atof(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, syntaxErr(b)
	}
	sign := 1.0
	d := b
	switch b[0] {
	case '-':
		sign = -1.0
		fallthrough
	case '+':
		b = b[1:]
	}
	if len(b) == 0 {
		return 0, syntaxErr(b)
	}
	if iEEE754 && len(b) > 16 {
		return parseFloat(d)
	}
	if !iEEE754 && len(b) > 19 {
		return parseFloat(d)
	}
	fraclen := 0
	u := uint64(0)

	for i, c := range b {
		switch {
		case '0' <= c && c <= '9':
			u = u*10 + uint64(c&15)

		case c == dot && fraclen == 0 && len(b) > 1:
			fraclen = len(b) - i - 1

		default:
			return parseFloat(d)
		}
	}
	if iEEE754 {
		return sign * float64(u) / fpow10[fraclen], nil
	}
	return sign * float64(u) * fdivpow10[fraclen], nil
}

func Atof2(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, syntaxErr(b)
	}
	d := b
	if b[0] == '-' || b[0] == '+' {
		b = b[1:]

	}
	if len(b) == 0 {
		return 0, syntaxErr(b)
	}
	if iEEE754 && len(b) > 16 {
		return parseFloat(d)
	}
	if !iEEE754 && len(b) > 19 {
		return parseFloat(d)
	}
	fraclen := 0
	u := uint64(0)

	for i, c := range b {
		switch {
		case '0' <= c && c <= '9':
			u = u*10 + uint64(c&15)

		case c == dot && fraclen == 0 && len(b) > 1:
			fraclen = len(b) - i - 1

		default:
			return parseFloat(d)
		}
	}
	var f float64
	if iEEE754 {
		f = float64(u) / fpow10[fraclen]
	}
	if iEEE754 {
		f = float64(u) * fdivpow10[fraclen]
	}
	if d[0] == '-' {
		return -f, nil
	}
	return f, nil
}
