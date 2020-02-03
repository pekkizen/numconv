package numconv
import "strconv"

// Dot is decimal separator
var	Dot byte = '.'

// Ignore is eg. thousand separator
var	Ignore byte  // = 0

// IEEE754  If true strconv.ParseFloat is used when needed. Parses also 'e' and 'x' formats
var	IEEE754 = false
// var	IEEE754 = true

var float64pow10 = [20]float64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}
var uint64pow10 = [20]uint64{
	1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

type numErr struct {
	s string
}
func (e *numErr) Error() string {
	return "numconv.Atof: " + e.s
}
func numError(b []byte, c byte) error {
	if len(b) == 0 {
		return  &numErr{"number is empty"}
	}
	if len(b) == 1 && c == Dot {
		return &numErr{"single dot is not a number"}
	}
	return &numErr{"number \""+string(b)+"\" has invalid char: \""+string(c)+"\""}
}

// https://play.golang.org/p/8uFiY5qdp0u
// https://golang.org/pkg/strconv/#ParseFloat
// strconv.ParseFloat returns the nearest floating-point number rounded using IEEE754 unbiased rounding.
// http://www.leapsecond.com/tools/fast_atof.c
// http://beedub.com/Sprite093/src/lib/c/stdlib/atof.c
// https://randomascii.wordpress.com/2012/02/25/comparing-floating-point-numbers-2012-edition/

// Atof returns float64 value of ASCII/UTF-8 coded decimal number in 'f' format |+/-|yy.xx .
// All non printable characters <= space (' ' = 32 dec) are left and right trimmed off.
// This includes "normal white space": space, LF, CR, tab (' ', '\n', '\r', '\t'). 
// Atof can parse up to 15 digits decimals and 18 digits decimals integers IEEE754 correctly. 
// More digits are mostly parsed correctly or adjacent float is given.
// Tested comparing to strconv.ParseFloat with random numbers, which ultimately doesn't prove anything.
// For number b := []byte("39.7784") Atof(b) is 3.6 x faster than strconv.ParseFloat(string(b), 64) 
//
func Atof(b []byte) (float64, error) {

	if len(b) == 0 {
		return 0, numError(b, 0)
	}
	r := len(b) - 1
	l := 0
	for b[l] <= ' ' && l < r { //left trim 
		l++
	}
	for b[r] <= ' ' && l < r { //right trim
		r--
	}
	neg := false
	b = b[l: r+1]
	switch  {
	case b[0] >= '0' || len(b) == 1: 
	case b[0] == '-':
		neg = true
		b = b[1:]
	case b[0] == '+':
		b = b[1:]
	}
	r = len(b) - 1

	if r == 0 && b[0] == Dot { 
		return 0, numError(b, Dot)
	}
	if r > 15 && IEEE754 {
		return strconvParseFloat(b, neg)
	}
	if r > 18 {
		return atofBig(b, neg)
	}

	n := uint64(0)
	dec := 0
	for i := 0; i <= r; i++ {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			n = 10*n + uint64(c -'0')

		case c == Dot && dec == 0:
			dec = r - i

		case IEEE754 && (c == 'e' || c == 'E' || c == 'x'):
			return strconvParseFloat(b, neg)

		case c != Ignore:
			return 0, numError(b, c)
		}
	}
	f := float64(n) 
	if neg {
		f = -f
	}
	return f / float64pow10[dec], nil
}

func strconvParseFloat(b []byte, neg bool) (float64, error) {
	f, e := strconv.ParseFloat(string(b), 64)
	if neg {
		f = -f
	}
	return f, e
}

// atofBig is Atof plus logic to handle cases with more than 19 digits.
func atofBig(b []byte, neg bool) (float64, error) {

	r := len(b) - 1
	for b[0] == '0' && r > 0 {  //leading zero(s)
		b = b[1: ]
		r--
	}
	haveDot := b[0] == Dot
	if haveDot {
		b = b[1:]
		r--
	}
	lzeros := 0
	if r > 12 && haveDot && b[0] == '0' { //r > 12 ?
		//skip and count zeros after dot
		lzeros = 1
		for i := 1; i <= r && b[i] == '0' && lzeros < 19; i++ {
			if b[i] != Ignore {
				lzeros++
			}
		}
		b = b[lzeros: ]
		r -= lzeros
	}
	dropped := 0
	if r > 18  {
		//cut length to 19 digits to fit uint64
		if b[18] == Dot {
			//only drop dot in the last position
			r = 17
		} else {
			for i := 19; i <= r && b[i] != Dot && !haveDot; i++ {
				//get number of digits dropped before decimal dot
				if b[i] != Ignore {
					dropped++ 
				}
			}
			r = 18
		}
	}
	n := uint64(0)
	dec := 0
	for i := 0; i <= r; i++ {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			n = 10*n + uint64(c -'0')
			if haveDot {
				dec++
			}
		case c == Dot && !haveDot:
			haveDot = true
		
		case IEEE754 && (c == 'e' || c == 'E' || c == 'x'):
			return strconvParseFloat(b, neg)

		case c != Ignore:
			return 0, numError(b, c)
		}
	}
	f := float64(n) 
	if neg {
		f = -f
	}
	if lzeros > 0 {
		f /= float64pow10[lzeros]
	}
	if dec > 0 {
		return f / float64pow10[dec], nil
	} 
	for dropped > 0 {
		f *= 10
		dropped--
	}
	return f, nil
}

// AtofFloat can parse up to 15 digits decimals and 16 digits decimals integers (IEEE754) correctly. 
// Otherwise it mostly behaves like like Atof but gives more adjacent and few next to adjacent values.
// FloatAtof uses very simple algorithm with only floating point arithmetic. 
//
func AtofFloat(b []byte) (float64, error) {

	if len(b) == 0 {
		return 0, numError(b, 0)
	}
	if len(b) > 310 {
		b = b[ :310]
	}
	r := len(b) - 1
	l := 0
	for b[l] <= ' ' && l < r { //left trim
		l++
	}
	for b[r] <= ' ' && l < r { //right trim
		r-- 
	}
	neg := false
	b = b[l: r+1]
	switch  {
	case b[0] >= '0' || len(b) == 1: 
	case b[0] == '-':
		neg = true
		b = b[1:]
	case b[0] == '+':
		b = b[1:]
	}
	r = len(b) - 1
	if r == 0 && b[0] == Dot {
		return 0, numError(b, Dot)
	}
	f := 0.0
	pow, div := 1.0, 1.0
	haveDot := false

	for i := r; i >= 0; i-- {
		c := b[i]
		switch {
		case '0' <= c && c <= '9':
			f += pow * float64(c - '0')
			pow *= 10 

		case c == Dot && !haveDot:
			haveDot = true
			div = pow

		case c != Ignore:
			return 0, numError(b, c)
		}
	}
	if neg {
		f = -f
	}
	return f / div, nil
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
//
func Ftoa(b *[]byte, f float64, dec int, sep byte) { 
	const significand  = 1<<53 - 1  // 15.95 decimal digits accuracy // todo 1<<52 - 1??
	const droplim = 10 * significand 
	const accuFloat  = 1<<53 - 1

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
	for g > droplim  {
		//drop noise decimals
		w /= 10
		dec--
		g = f * float64(w) 
	}
	n := uint64(g + 0.5)
	q := n / w
	itoaPos(b, q, 0, Dot)

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
func roundToPow10(n, w, scope uint64) uint64 {
	const tol = 18
	if n <= scope {
		return n
	}
	k := (n + tol) % scope
	if k > 2 * tol {
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
	for n % 10 == 0 && n > 9 {
		n /= 10
		w /= 10
		dec--
	}
	if n <= scope {
		return 
	}
	i = (n + tol) % scope
	if i > 2 * tol {
		return 
	}
	i = n + (tol - i)
	if i >= w {
		return 
	}
	n = i
	for n % 100 == 0 && n > 99 {
		n /= 100
		dec -= 2
	}
	if n % 10 == 0 && n > 9 {
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
		f *=  10
		zeros++
	}
	appendZeros(b, zeros)
	n := uint64(f * 1e17 + 0.5)
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
		*b = append(*b, zerobytes[ :zeros]...)
		return
	}
	for zeros > maxzerobytes {
		*b = append(*b, zerobytes[:]...)
		zeros -= maxzerobytes
	}
	if zeros > 0 {
		*b = append(*b, zerobytes[ :zeros]...)
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
	itoaPos(b, q, 0, Dot)
	n -= q * 100
	q = n / 10
	*b = append(*b, byte('0'+ q), byte('0'+ n - q*10))
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
	const significand  = 1<<53 - 1  
	const droplim = 10 * significand  

	zeros := 0
	for f > droplim  {
		f /= 10
		zeros++
	}
	itoaPos(b, uint64(f + 0.5), 0, 0)
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
		t[i] = byte('0'+ n - q*10)
		n = q
		dec--
		i--
	}
	t[i] = byte('0'+ n)
	*b = append(*b, t[i: ]...)
}

