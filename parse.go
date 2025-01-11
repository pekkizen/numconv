package numconv

import "strconv"

const (
	dot                 = '.'
	asciiWhitespaceOnly = false
	maxNumberLen        = 16 // 15 digits + dot or 16 digits
	// asciiZero           = int64('0')
)

var f64pow10 = [...]float64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}
var u64pow10 = [...]uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

type err struct {
	s string
}

func (e *err) Error() string {
	return "numconv.Atof: " + e.s
}

//go:noinline
func syntaxErr(b []byte) error {
	return &err{"parsing " + "\"" + string(b) + "\": " + "invalid syntax"}
}

//go:noinline
// func syntaxEmptyNumber(b []byte) error {
// 	return &err{"parsing " + "\"" + string(b) + "\": " + "the number is empty"}
// }

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

// TrimSpace returns a subslice of b with all leading and trailing whitespace removed.
// Adapted from bytes.TrimSpace. Trim above is faster.
func TrimSpace(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	l := 0
	for l = range b {
		if !isWhitespace(b[l]) {
			break
		}
	}
	b = b[l:]
	r := len(b) - 1
	for ; r >= 0; r-- {
		if !isWhitespace(b[r]) {
			break
		}
	}
	return b[:r+1]
}

func TrimTrailingZeros(b []byte) []byte {
	for len(b) > 1 && b[len(b)-1] == '0' && b[len(b)-2] != dot {
		b = b[:len(b)-1]
	}
	return b
}

//go:noinline
func parseFloat(b []byte) (float64, error) {
	return strconv.ParseFloat(string(b), 64)
}

//go:noinline
func parseDigit(c byte) (float64, error) {
	if '0' <= c && c <= '9' {
		return float64(int64(c & 15)), nil
	}
	return 0, syntaxErr([]byte{c})
}

func isDigitOrDot(c byte) bool {
	return ('0' <= c && c <= '9') || c == dot
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
	if len(b) == 1 {
		return parseDigit(b[0])
	}
	d := b
	if b[0] == '-' || b[0] == '+' {
		b = b[1:]
	}
	if len(b) > maxNumberLen {
		return parseFloat(d)
	}
	exp10 := 0
	u := uint64(0)

	for i, c := range b {
		switch {
		case '0' <= c && c <= '9':
			u = u*10 + uint64(c&15)

		case c == dot && exp10 == 0 && len(b) > 1:
			exp10 = len(b) - (i + 1)

		default:
			return parseFloat(d)
		}
	}
	f := float64(int64(u)) / f64pow10[exp10]
	if d[0] == '-' {
		f = -f
	}
	return f, nil
}

/*
AtofFloat can parse up to 15 digits decimals and 16 digits decimals integers (IEEE754) correctly.
AtofFloat uses simple algorithm with only floating point arithmetic. Amount of parsed digits is
limited to 308 but accuracy is still limited to the first 16-18 digits.
Atof is ~1.4 x faster than AtofFloat.
*/
func AtofFloat(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, syntaxErr(b)
	}
	d := b
	neg := b[0] == '-'
	if len(b) > 1 && (neg || b[0] == '+') && isDigitOrDot(b[1]) {
		b = b[1:] // trim sign
	}
	// for len(b) > 2 && b[0] == '0' { //trim left zeros??
	// 	b = b[1:]
	// }
	var (
		hasdot = false
		// trunc   = false
		f       float64
		pow     = float64(1.0)
		powfrac = float64(1.0)
	)
	// if len(b) > 309 {
	// 	b = b[:309] //1e308 < maxFloat64 < 2e308
	// 	trunc = true
	// }
	for i := len(b) - 1; i >= 0; i-- {
		c := b[i]

		switch {
		case '0' <= c && c <= '9':
			f += pow * float64(c-'0')
			pow *= 10

		case c == dot && !hasdot && len(b) > 1:
			powfrac = pow
			hasdot = true

		default:
			return parseFloat(d)
		}
	}
	// if trunc && !hasdot {
	// 	f *= 10 // f -> +Inf
	// }
	if neg {
		f = -f
	}
	return f / powfrac, nil
}
