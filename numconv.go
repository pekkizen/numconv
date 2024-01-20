package numconv

import (
	"strconv"
)

const (
	dot = '.'
	// use_ascii_whitespace = false
)

var float64pow10 = [19]float64{
	1e0, 1e1, 1e2, 1e3, 1e4,
	1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14,
	1e15, 1e16, 1e17, 1e18,
}
var uint64pow10 = [20]uint64{
	1e0, 1e1, 1e2, 1e3, 1e4,
	1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14,
	1e15, 1e16, 1e17, 1e18, 1e19,
}

type err struct {
	s string
}

func (e *err) Error() string {
	return "numconv.Atof: " + e.s
}
func syntaxError(b []byte) error {
	return &err{"parsing " + "\"" + string(b) + "\"" + ": invalid syntax"}
}

// Trim returns a subslice of b with all leading and trailing "white" space removed.
// All characters <= Ascii space (' ' = 32 dec) are left and right trimmed off.
// This includes, among others, standard Ascii white space ' ', '\n', '\f', '\r', '\t',' \v'.
func Trim(b []byte) []byte {

	if len(b) == 0 || (notSpace(b[0]) && notSpace(b[len(b)-1])) { //supposed to be common case
		return b
	}
	for len(b) > 0 && isSpace(b[0]) { //trim left
		b = b[1:]
	}
	for len(b) > 0 && isSpace(b[len(b)-1]) { //trim right
		b = b[:len(b)-1]
	}
	return b
}
func isSpace(c byte) bool {
	return c <= ' '
}
func notSpace(c byte) bool {
	return c > ' '
}

// Trimming default ascii white space:
// var asciiSpace = [256]bool{
// 	'\t': true, '\n': true, '\v': true, '\f': true, '\r': true, ' ': true}
// return asciiSpace[c]
// or
// return c == ' ' || (9 <= c && c <= 13)

// This is not faster than Trim. It is here only for testing.
func TrimIndexing(b []byte) []byte {
	left, right := 0, len(b)-1

	if right == -1 || (b[0] > ' ' && b[right] > ' ') {
		return b
	}
	for left < right && b[left] <= ' ' {
		left++
	}
	for right >= left && b[right] <= ' ' {
		right--
	}
	return b[left : right+1]
}

/*
Atof parses decimal numbers and integers from byte slice b. In XML files the
format is xsd:decimal — Decimal numbers. Valid values include: 123.456,
+1234.456, -1234.456, -.456, or -456. Integers without decimal dot are also
accepted. -.0 -0. and 0 are ok. +-.0, -0-, . , -, + are not.
Error if returned for other number formats. Atof(Trim(b)) is
4 x faster than strconv.ParseFloat(string(bytes.TrimSpace(b)), 64).
Up to 15 digits decimals Atof gives same as strconv.ParseFloat. For more
digits the results may be adjacent float64 number.
*/
func Atof(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, &err{"number is empty"}
	}
	neg := b[0] == '-'
	if (neg || b[0] == '+') && len(b) > 1 && b[1] >= '.' {
		b = b[1:] //skip sign
	}
	r := len(b) - 1
	if r > 18 {
		f, e := AtofFloat(b)
		if neg {
			f = -f
		}
		return f, e
	}
	u := uint64(0)
	dec := 0
	for i := 0; i <= r; i++ {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			u = 10*u + uint64(c-'0')

		case c == dot && dec == 0 && r > 0:
			dec = r - i

		default:
			return 0, syntaxError(b)
		}
	}
	f := float64(u) / float64pow10[dec]
	if neg {
		f = -f
	}
	return f, nil
}

// AtofFloat can parse up to 15 digits decimals and 16 digits decimals integers (IEEE754) correctly.
// Otherwise it mostly behaves like like Atof but gives more adjacent and few next to adjacent values.
// AtofFloat uses simple algorithm with only floating point arithmetic. Amount of parsed digits is
// limited to 310. Accuracy is still limited to first 16-17 digits
// AtofFloat is very slightly slower than Atof.
func AtofFloat(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, &err{"number is empty"}
	}
	neg := b[0] == '-'
	if (neg || b[0] == '+') && len(b) > 1 && b[1] >= '.' {
		b = b[1:] //skip sign
	}
	if len(b) > 309 {
		b = b[:309]
	}
	r := len(b) - 1
	pow, div := 1.0, 1.0
	f := 0.0
	for i := r; i >= 0; i-- {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			f += pow * float64(c-'0')
			pow *= 10

		case c == dot && div == 1 && r > 0:
			div = pow

		default:
			return 0, syntaxError(b)
		}
	}
	if neg {
		f = -f
	}
	return f / div, nil // Inf/Inf = NaN
}

/*
ParseFloat returns float64 value of ASCII/UTF-8 coded decimal number in decimal
format |+/-|yy.xx . If number is in other format strconv.ParseFloat is used.
*/
func ParseFloat(b []byte) (float64, error) {
	f, e := Atof(b)
	if e != nil {
		f, e = strconv.ParseFloat(string(b), 64)
	}
	return f, e
}

// Following functions formats and appends ascii digits of int/float64 number to slice []bytes.
// These functions are significantly faster than strconv FormatFloat and faster than FormatIn.
// Ftoa(&b, 3.14159, 3, 0) is 9 x faster than b = append(b, strconv.FormatFloat(3.14159, 'f', 3, 64)...)
// Some of the speed gain come from directly "writing" to (output) buffer b.
// Up to 15 digits Ftoa gives same ascii decimal as strconv.FormatFloat. Of 16 digits numbers
// 99.9% are same and for the rest Ftoa gives adjacent decimal number.

// Ftoa appends float64 number f as decimal ASCII digits to byte slice *b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number. sep = 0 -> append nothing.
// dec is number of decimals.
// Decimals or significant digits are given only to 16 digits (float64 accuracy 15.95 digits).
// ftoa handles rounding "normal" way: +/-1.95xxx with one decimal gives +/-2.0
func Ftoa(b *[]byte, f float64, dec int, sep byte) {
	const significand = 1<<53 - 1 // 15.95 decimal digits accuracy
	const droplim = 10 * significand

	if dec > 17 {
		dec = 17
	}
	fullprec := false
	if dec < 0 {
		fullprec = true
		dec = 17
	}
	if f < 0 {
		*b = append(*b, '-')
		f = -f
	}
	if f < 1 && fullprec {
		ftoaSmallFullPrec(b, f, sep)
		return
	}
	if f > significand || dec == 0 {
		//can't give any non noise decimals
		ftoItoa(b, f, sep)
		return
	}
	w := uint64pow10[dec]
	g := f * float64(w)
	for g > droplim {
		//drop noise decimals
		w /= 10
		dec--
		g = f * float64(w)
	}
	n := uint64(g + 0.5)
	q := n / w
	itoaPos(b, q, 0, dot)

	n -= q * w
	if fullprec {
		n, dec = roundToMillionsAndDropZeros(n, w, 1e8, dec)
		if n == 0 {
			dec = 0
		}
	}
	itoaPos(b, n, dec, sep)
}

// roundToPow10 rounds integers go nearest "million", if already near enough.
// eg. 199999982 - 200000018 rounds to 200000000 with near tol = 18.
// n is remainder of something > w divided by w. See Ftoa above. Max rounded value is w-1,
// because these are decimal parts and we dont want/are not able anymore to carry rounding
// to the integer part. scope is 10^k, where k is number of rightmost digits in rounding.
func RoundToPow10(n, w, scope uint64) uint64 {
	const tol = 18
	if n <= scope {
		return n
	}
	k := (n + tol) % scope
	if k > 2*tol {
		return n
	}
	k = n + (tol - k)
	if k < w {
		return k
	}
	return n
}

// roundToMillionsAndDropZeros is roundToPow10 plus trailing zeroes cleaning.
func roundToMillionsAndDropZeros(i, w, scope uint64, d int) (n uint64, dec int) {
	const tol = 18
	n = i
	dec = d
	for n%10 == 0 && n > 9 {
		n /= 10
		w /= 10
		dec--
	}
	if n <= scope {
		return
	}
	i = (n + tol) % scope
	if i > 2*tol {
		return
	}
	i = n + (tol - i)
	if i >= w {
		return
	}
	n = i
	for n%100 == 0 && n > 99 {
		n /= 100
		dec -= 2
	}
	if n%10 == 0 && n > 9 {
		n /= 10
		dec--
	}
	return
}

// ftoaSmallFullPrec appends f < 1 to *b with 16/17 digits precision and unlimited number of leading zeroes.
func ftoaSmallFullPrec(b *[]byte, f float64, sep byte) {

	*b = append(*b, '0', '.')
	zeros := 0
	for f < 0.1 {
		f *= 10
		zeros++
	}
	appendZeros(b, zeros)
	n := uint64(f*1e17 + 0.5)
	if zeros < 10 {
		n, _ = roundToMillionsAndDropZeros(n, 1e17, 1e8, 0)
	}
	itoaPos(b, n, 0, sep)
}

// appendZeros appends zeros '0' to *b
func appendZeros(b *[]byte, zeros int) {
	const maxzerobytes = 10
	var zerobytes = [maxzerobytes]byte{
		'0', '0', '0', '0', '0', '0', '0', '0', '0', '0',
	}
	if zeros <= maxzerobytes {
		*b = append(*b, zerobytes[:zeros]...)
		return
	}
	for zeros > maxzerobytes {
		*b = append(*b, zerobytes[:]...)
		zeros -= maxzerobytes
	}
	if zeros > 0 {
		*b = append(*b, zerobytes[:zeros]...)
	}
}

// Ftoa2 is faster special Ftoa for two decimals.
// This is 18 x faster than strconv.FormatFloat.
func Ftoa2(b *[]byte, f float64, sep byte) {
	const maxUint64 = 1<<64 - 1
	if f < 0 {
		*b = append(*b, '-')
		f = -f
	}
	g := f*100 + 0.5
	if g > maxUint64 {
		Ftoa(b, f, 2, sep)
		return
	}
	n := uint64(g)
	q := n / 100
	itoaPos(b, q, 0, dot)
	n -= q * 100
	q = n / 10
	*b = append(*b, byte('0'+q), byte('0'+n-q*10))
	if sep > 0 {
		*b = append(*b, sep)
	}
}

// Itoa appends integer i to slice *b
func Itoa(b *[]byte, i int, sep byte) {
	if i < 0 {
		i = -i
		*b = append(*b, '-')
	}
	itoaPos(b, uint64(i), 0, sep)
}

// ftoItoa appends positive float64 f to *b with max 16/17 digits and possible trailing zeros.
func ftoItoa(b *[]byte, f float64, sep byte) {
	const significand = 1<<53 - 1
	const droplim = 10 * significand

	zeros := 0
	for f > droplim {
		f /= 10
		zeros++
	}
	itoaPos(b, uint64(f+0.5), 0, 0)
	appendZeros(b, zeros)
	if sep > 0 {
		*b = append(*b, sep)
	}
}

// itoaPos appends positive uint64 n to slice *b left zero padded to dec digits.
// adapted from stdlib /log/log.go itoa.
func itoaPos(b *[]byte, n uint64, dec int, sep byte) {
	var t [21]byte

	i := 20
	if sep > 0 {
		t[i] = sep
		i--
	}
	for n >= 10 || dec > 1 {
		q := n / 10
		t[i] = byte('0' + n - q*10)
		n = q
		dec--
		i--
	}
	t[i] = byte('0' + n)
	*b = append(*b, t[i:]...)
}
