package numconv

import (
	"encoding/binary"
	"strconv"
)

// https://godbolt.org/z/T1drG54az //assembler code from several compilers.

const (
	maxUint64  = 1<<64 - 1
	maxUint32  = 1<<32 - 1 // ~4.2e9
	maxDecimal = 1e17 - 1
	half       = 0.49999999999999994 // 0.5-ulp, more strconv.FormatFloat compatible than 0.5
	// half = 0.5
	// useStrconv = true
)

var u64pow10 = []uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

/*
Constants for transforming integer div n/10, n/100 etc.
to multiply and bitshift operation. Go compiler do something
similar for constant divs, but these are sligthly faster.
(n * d1) >> e1 == n/10 for  n <= 1<<32 ~ 4.2e9
(n * d2) >> e1 == n/100 for n <= 1<<32
(n * d3) >> e3 == n/1e3 for n <= 1<<32
(n * d4) >> e4 == n/1e4 for n <= 1<<32
(n * d5) >> e5 == n/1e5 for n <  1<<31  	!!
(n * d6) >> e6 == n/1e6 for n <= 1<<32
*/
const (
	e1 = 32 + 3 // 32 + floor(log2(10) = 3.32...)
	e2 = 32 + 6 // 32 + floor(log2(100))
	e3 = 32 + 9 // etc.
	e4 = 32 + 13
	e5 = 32 + 16
	e6 = 32 + 19
	e7 = 32 + 23
	e8 = 32 + 26

	d1 = (1<<e1)/10 + 1
	d2 = (1<<e2)/100 + 1
	d3 = (1<<e3)/1000 + 1
	d4 = (1<<e4)/10000 + 1
	d5 = (1<<e5)/100000 + 1
	d6 = (1<<e6)/1000000 + 1
	d7 = (1<<e7)/10000000 + 1
	d8 = (1<<e8)/100000000 + 1
)

// return uint64(f + half) compiles to this with gc 1.22.1:
// MOVSD $f64.3fdfffffffffffff(SB), X1
// ADDSD X1, X0
// MOVSD $f64.43e0000000000000(SB), X1
// UCOMISD X0, X1
// JLS main_roundToUint64_pc34
// CVTTSD2SQ X0, CX
// NOP
// JMP main_roundToUint64_pc48
// main_roundToUint64_pc34:
// SUBSD X1, X0
// CVTTSD2SQ X0, CX
// BTSQ $63, CX
// main_roundToUint64_pc48:
// MOVQ CX, AX
// RET

func roundToUint64(f float64) uint64 {
	return uint64(int64(f + half)) // sic, see above
	// return uint64(f + half)

	// MOVSD $f64.3fdfffffffffffff(SB), X1
	// ADDSD X1, X0
	// CVTTSD2SQ X0, AX
	// https://godbolt.org/z/3EY8GK7Tf
}

// In divmod-functions the compiler creates quite good code for constant integer
// division with remainder. Compiler code for n/10 - n/1000000 is sligthly slower
// than my code below, but the gain is mostly lost by n < maxUint32 condition
// evaluation. This is not a branch mispredictipion thing.
// All ASM code is from https://godbolt.org/ with x86-64 gc (1.22.6. ++
func divmod1(f float64) (q, r uint64) {
	n := roundToUint64(f * 10)
	if n < maxUint32 {
		return (n * d1) >> e1, n - (n*d1)>>e1*10
	}
	return n / 10, n % 10
}
func divmod2(f float64) (q, r uint64) {
	n := roundToUint64(f * 100)
	if n < maxUint32 {
		return (n * d2) >> e2, n - 100*(n*d2>>e2)
		// MOVL    $2748779070, AX
		// IMULQ   BX, AX
		// SHRQ    $38, AX
		// IMUL3Q  $100, AX, CX
		// SUBQ    CX, BX

		// https://godbolt.org/z/qo9qer91W
	}
	return n / 100, n % 100
	// MOVQ    BX, CX
	// SHRQ    $1, CX
	// MOVQ    $-6640827866535438581, AX
	// MULQ    CX
	// SHRQ    $5, DX
	// IMUL3Q  $100, DX, CX
	// SUBQ    CX, BX
	// MOVQ    DX, AX
}
func divmod3(f float64) (q, r uint64) {
	n := roundToUint64(f * 1000)
	if n < maxUint32 {
		return (n * d3) >> e3, n - 1000*(n*d3>>e3)
	}
	return n / 1000, n % 1000
}
func divmod4(f float64) (q, r uint64) {
	n := roundToUint64(f * 1e4)
	if n < maxUint32 {
		return (n * d4) >> e4, n - 1e4*(n*d4>>e4)
	}
	return n / 1e4, n % 1e4
}
func divmod5(f float64) (q, r uint64) {
	n := roundToUint64(f * 1e5)
	if n < maxUint32/2 { // !!!
		return (n * d5) >> e5, n - 1e5*(n*d5>>e5)
	}
	return n / 1e5, n % 1e5
}
func divmod6(f float64) (q, r uint64) {
	n := roundToUint64(f * 1e6)
	if n < maxUint32 {
		return (n * d6) >> e6, n - 1e6*(n*d6>>e6)
	}
	return n / 1e6, n % 1e6
}

// Ftoa appends float64 number f as decimal ascii digits to byte slice b.
func Ftoa(b []byte, f float64, prec int, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	g := f * fpow10[prec]
	if g > 1e16 {
		b = append(b, strconv.FormatFloat(f, 'f', prec, 64)...)
		return append(b, sep)
	}
	if prec <= 0 {
		return append(utoa(b, roundToUint64(f)), sep) // no dot
	}
	w := u64pow10[prec]
	n := roundToUint64(g)
	q := n / w
	r := n - q*w
	// q, r := n/w, n-n/w*w
	b = utoa(b, q)
	b = append(b, dot)
	if prec <= 8 {
		b = utoa8Dec(b, r, prec)
	} else {
		b = utoaDec(b, r, prec)
	}
	return append(b, sep)
}

// ftoaFull appends float64 number f as decimal ascii digits to byte slice b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number.
// dec is number of decimals.
func FtoaFull(b []byte, f float64, dec int, sep byte) []byte {
	if dec > 16 {
		dec = 16
	}
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	if f > maxUint64 || dec <= 0 {
		return ftoItoa(b, f, sep)
	}
	f *= fpow10[dec]
	for f > maxDecimal {
		f /= 10
		dec--
	}
	w := u64pow10[dec]
	n := roundToUint64(f)
	q := n / w
	b = utoa(b, q)
	b = append(b, dot)
	b = utoaDec(b, n-q*w, dec)
	return append(b, sep)
}

// ftoItoa appends positive float64 f to b as integer with
// max 17 ascii digits and possible trailing zeros or in e-format
func ftoItoa(b []byte, f float64, sep byte) []byte {
	zeros := 0
	for f > maxUint64 {
		f /= 10
		zeros++
	}
	b = utoa(b, roundToUint64(f))
	if zeros > 5 {
		b = append(b, 'e', '+')
		b = utoa8(b, uint64(zeros))
	} else {
		b = appendZeros(b, zeros)
	}
	return append(b, sep)
}

// appendZeros appends zeros '0' to b.
func appendZeros(b []byte, zeros int) []byte {
	var z = [8]byte{'0', '0', '0', '0', '0', '0', '0', '0'}
	for zeros > 8 {
		b = append(b, z[:]...)
		zeros -= 8
	}
	if zeros > 0 {
		b = append(b, z[:zeros]...)
	}
	return b
}

func SetSign(b []byte, f float64) ([]byte, float64) {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	// var signbyte = [2]byte{' ', '-'}
	// b = append(b, signbyte[math.Float64bits(f)>>63])
	// f = math.Float64frombits(math.Float64bits(f) &^ (1 << 63))
	return b, f
}

// Ftoa80 is a faster special Ftoa for f < 1e8 and 0 decimals
func Ftoa80(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}

	return append(utoa8(b, roundToUint64(f)), sep) // no dot
}

// Ftoa81 is a faster special Ftoa for f < 1e8 and 1 decimal
func Ftoa81(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod1(f)
	return append(utoa8(b, q), dot, byte('0'+r), sep)
}

// Ftoa82 is a faster special Ftoa for two decimals and f < 1e8.
// s = Ftoa82(s, f, sep) is 50 x faster for f=12.34 than
// s = append(s, strconv.FormatFloat(f, 'f', 2, 64)...)
func Ftoa82(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod2(f)
	return utoaDec2(utoa8(b, q), r, sep) // dot, digits, sep
}

// Ftoa83 is a faster special Ftoa for three decimals and f < 1e8.
func Ftoa83(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod3(f)
	return utoaDec3(utoa8(b, q), r, sep)
}

// Ftoa84 is a faster special Ftoa for four decimals and f < 1e8.
func Ftoa84(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod4(f)
	return utoaDec4(utoa8(b, q), r, sep)
}

// Ftoa85 is a faster special Ftoa for five decimals and f < 1e8.
func Ftoa85(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod5(f)
	return utoaDec5(utoa8(b, q), r, sep)
}

// Ftoa86 is a faster special Ftoa for six decimals and f < 1e8.
func Ftoa86(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod6(f)
	// utoa6Dec would be too big to be inlined.
	q1, q2, q3, q4, q5 := (r*d1)>>e1, (r*d2)>>e2, (r*d3)>>e3, (r*d4)>>e4, (r*d5)>>e5
	return append(utoa8(b, q), dot,
		byte('0'+q5),
		byte('0'+q4-q5*10),
		byte('0'+q3-q4*10),
		byte('0'+q2-q3*10),
		byte('0'+q1-q2*10),
		byte('0'+r-q1*10), sep)
}

// Itoa appends int64 n to slice b as ascii digits.
func Itoa(b []byte, n int64, sep byte) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	if n < 1e8 {
		return append(utoa8(b, uint64(n)), sep)
	}
	return append(utoa(b, uint64(n)), sep)
}

func Itoa8(b []byte, n int64, sep byte) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	return append(utoa8(b, uint64(n)), sep)
}

// Utoa8 appends uint64 n < 1e8 to slice b as ascii digits.
func Utoa8(b []byte, n uint64, sep byte) []byte {
	return append(utoa8(b, n), sep)
}

func Utoa(b []byte, n uint64, sep byte) []byte {
	return append(utoa(b, n), sep)
}

// utoaX appends uint64 n to slice b as ascii digits.
// func utoaX(b []byte, n uint64) []byte {
// 	if n < 1e8 {
// 		return utoa8(b, n)
// 	}
// 	return utoa(b, n)
// }

// utoaDecX appends uint64 n to slice b left zero padded to dec ascii digits.
// func utoaDecX(b []byte, n uint64, dec int) []byte {
// 	if dec < 8 && n < 1e8 {
// 		return utoa8Dec(b, n, dec)
// 	}
// 	return utoaDec(b, n, dec)
// }

// utoa appends positive integer n as ascii digits to slice b.
func Zutoa(b []byte, n uint64) []byte {
	if n < 1e8 {
		return utoa8(b, n)
	}
	var t [20]byte
	i := len(t) - 1
	for n >= 10 {
		q := n / 10
		t[i] = byte('0' + n - q*10)
		n = q
		i--
	}
	t[i] = byte('0' + n)
	return append(b, t[i:]...)
}

func utoa(b []byte, n uint64) []byte {
	if n < 1e8 {
		return utoa8(b, n)
	}
	var r uint64
	wid := 0
	if n > 1e16 {
		n, r = n/1e16, n%1e16
		b = utoa8(b, n)
		n = r
		wid = 8
	}
	if n > 1e8 {
		n, r = n/1e8, n%1e8
		b = utoa8Dec(b, n, wid)
	}
	return utoa8Dec(b, r, 8)
}

// utoaDec appends positive integer n to slice b left
// zero padded to dec digits.
func utoaDec(b []byte, n uint64, wid int) []byte {
	var t [20]byte

	i := len(t) - 1
	for n >= 10 || wid > 1 {
		q := n / 10
		t[i] = byte('0' + n - q*10)
		n = q
		wid--
		i--
	}
	t[i] = byte('0' + n)
	return append(b, t[i:]...)
}

const asciiZeros = 0x3030303030303030 // '0'== 0x30

// https://godbolt.org/z/MvjTzYPab
// utoa8 appends positive integer n < 1e8 as ascii digits to slice b.
// utoa8 is 3+ x faster for 3 digists than utoa20 above. And 6 x
// faster than b = append(b, strconv.Itoa(int(n))...)
func utoa8(b []byte, n uint64) []byte {
	var u uint64
	l := len(b) + 1
	for n >= 10 {
		u, n = (u+n-((n*d1)>>e1)*10)<<8, (n*d1)>>e1
		l++
	}
	u += n + asciiZeros //adds '0'= 0x30 to each byte
	return binary.LittleEndian.AppendUint64(b, u)[:l]
}

// utoa8Dec appends positive integer n < 1e8 to slice b
// left zero padded to wid digits.
func utoa8Dec(b []byte, n uint64, wid int) []byte {
	var u uint64
	l := 1
	for n >= 10 || wid > 1 {
		u, n = (u+n-((n*d1)>>e1)*10)<<8, (n*d1)>>e1
		l++
		wid--
	}
	if l > 8 {
		panic("utoa8Dec: too many digits and/or too much width")
	}
	u += n + asciiZeros
	l += len(b)
	return binary.LittleEndian.AppendUint64(b, u)[:l]
}

// utoaDec2 appends n < 100 as two left zero padded ascii digits to slice b.
// Plus decimal dot and number separator sep.
func utoaDec2(b []byte, n uint64, sep byte) []byte {
	return append(b, dot,
		byte('0'+(n*d1)>>e1),
		byte('0'+n-(n*d1)>>e1*10), sep)
}

// utoaDec3 appends n < 1000 as three left zero padded ascii digits to slice b.
// Plus decimal dot and number separator sep.
func utoaDec3(b []byte, n uint64, sep byte) []byte {
	q1, q2 := (n*d1)>>e1, (n*d2)>>e2
	return append(b, dot,
		byte('0'+q2),
		byte('0'+q1-q2*10),
		byte('0'+n-q1*10), sep)
}

// utoaDec4 appends n < 10000 as four left zero padded ascii digits to slice b.
// Plus decimal dot and number separator sep.
func utoaDec4(b []byte, n uint64, sep byte) []byte {
	q1, q2, q3 := (n*d1)>>e1, (n*d2)>>e2, (n*d3)>>e3
	return append(b, dot,
		byte('0'+q3),
		byte('0'+q2-q3*10),
		byte('0'+q1-q2*10),
		byte('0'+n-q1*10), sep)
}

// utoaDec5 appends n < 100000 as five left zero padded ascii digits to slice b.
// Plus decimal dot and number separator sep.
func utoaDec5(b []byte, n uint64, sep byte) []byte {
	q1, q2, q3, q4 := (n*d1)>>e1, (n*d2)>>e2, (n*d3)>>e3, (n*d4)>>e4
	return append(b, dot,
		byte('0'+q4),
		byte('0'+q3-q4*10),
		byte('0'+q2-q3*10),
		byte('0'+q1-q2*10),
		byte('0'+n-q1*10), sep)
}
