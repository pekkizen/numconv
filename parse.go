package numconv

const (
	dot                 = '.'
	asciiWhitespaceOnly = false
)

var fpow10 = [...]float64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}
var upow10 = [...]uint64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

type err struct {
	s string
}

func (e *err) Error() string {
	return "numconv.Atof: " + e.s
}
func syntax(b []byte, s string) string {
	return "parsing " + "\"" + string(b) + "\": " + s
}

// Trim returns a subslice of b with all leading and trailing whitespace removed.
// All characters <= Ascii space (' ' = 32 dec) are left and right trimmed off.
// This includes, among others, standard Ascii white space ' ', '\t', '\n', '\v', '\f', '\r'.
func Trim(b []byte) []byte {

	for len(b) > 1 && space(b[0]) { //left trim
		b = b[1:]
	}
	for len(b) > 0 && space(b[len(b)-1]) { //right rim
		b = b[:len(b)-1]
	}
	return b
}

func space(c byte) bool {
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
		if !space(b[l]) {
			break
		}
	}
	b = b[l:]
	r := len(b) - 1
	for ; r >= 0; r-- {
		if !space(b[r]) {
			break
		}
	}
	return b[:r+1]
}

/*
Atof parses decimal numbers and integers from byte slice b. In XML files the
format is xsd:decimal — Decimal numbers. Valid values include: 123.456,
+1234.456, -1234.456, -.456, or -456. Integers without decimal dot are also
accepted. -.0 , -00. 000 and 0 are ok. Use Trim function to remove leading and
trailing white space. Atof regards it as an error.
Error is returned for other number formats. Atof(b) is 4+ x faster
than strconv.ParseFloat(string(b), 64). Up to 15 digits decimals
Atof gives same as strconv.ParseFloat. For more digits the results
may be adjacent float64 number.
*/
func Atof(b []byte) (float64, error) {
	// if useStrconv {
	// 	return strconv.ParseFloat(string(b), 64)
	// }
	if len(b) == 0 {
		return 0, &err{syntax(b, "number is empty")}
	}
	s := b //keep original for error or AtofFloat

	neg := b[0] == '-'
	if (neg || b[0] == '+') && len(b) > 1 && b[1] >= dot {
		b = b[1:] //trim sign if adjacent to dot/number
	}
	// Unint64 can hold 19 base 10 digits and we hope that there will be a dot in 20
	// bytes long number. If it has 20 or more digits and no dot, AtofFloat is used.
	if len(b) > 20 {
		b = b[:20]
	}
	hasDot := false
	u := uint64(0)
	dec := 0
	for i, c := range b {
		switch {
		case '0' <= c && c <= '9':
			u = 10*u + uint64(c-'0')

		case c == dot && !hasDot && len(b) > 1:
			dec = len(b) - 1 - i //# decimals
			hasDot = true

		default:
			return 0, &err{syntax(s, "invalid syntax")}
		}
	}
	if len(b) == 20 && !hasDot {
		return AtofFloat(s)
	}
	f := float64(u) / fpow10[dec]
	if neg {
		f = -f
	}
	return f, nil
}

/*
AtofFloat can parse up to 15 digits decimals and 16 digits decimals integers (IEEE754) correctly.
Otherwise it mostly behaves like like Atof but gives more adjacent and few next to adjacent values.
AtofFloat uses simple algorithm with only floating point arithmetic. Amount of parsed digits is
limited to 308 but accuracy is still limited to the first 16-18 digits.
Atof is ~1.4 x faster than AtofFloat.
*/
func AtofFloat(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, &err{syntax(b, "number is empty")}
	}
	neg := b[0] == '-'
	if (neg || b[0] == '+') && len(b) > 1 && b[1] >= dot { //trim sign
		b = b[1:]
	}
	for len(b) > 2 && b[0] == '0' { //trim left zeros
		b = b[1:]
	}
	var (
		hasdot = false
		trunc  = false
		f      float64
		pow    = float64(1.0)
		powdec = float64(1.0)
	)
	if len(b) > 309 {
		b = b[:309] //1e308 < maxFloat64 < 2e308
		trunc = true
	}
	for i := len(b) - 1; i >= 0; i-- {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			f += pow * float64(c-'0')
			pow *= 10

		case c == dot && !hasdot && len(b) > 1:
			powdec = pow
			hasdot = true

		default:
			return 0, &err{syntax(b, "invalid syntax")}
		}
	}
	if trunc && !hasdot {
		f *= 10 // f -> +Inf
	}
	if neg {
		f = -f
	}
	return f / powdec, nil
}
