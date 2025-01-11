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
	half       = 0.4999999999999994 // more strconv.FormatFloat compatible than 0.5
	// half = 0.5
	// useStrconv = true
)

/*
Constants for transforming integer div n/10, n/100 etc.
to multiply and bitshift operation. Go compiler do something
similar for constant divs and mods, but these are sligthly faster.
(n * d1) >> e1 == n/10 for  n <= 1<<32 ~ 4.2e9
(n * d2) >> e1 == n/100 for n <= 1<<32
(n * d3) >> e3 == n/1e3 for n <= 1<<32
(n * d4) >> e4 == n/1e4 for n <= 1<<32
(n * d5) >> e5 == n/1e5 for n <  3e9  	!!
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
	// MOVSD $f64.3fdfffffffffffff(SB), X1
	// ADDSD X1, X0
	// CVTTSD2SQ X0, AX
	// RET
}

// In divmod-functions the compiler creates quite good code for constant integer
// division and remainder. Compiler code for n/10 - n/1000000 is 20 to 50% slower
// than my code above. This gain is mostly lost if we have to check for n < maxUint32, even
// this is 100% of the case. All ASM code is from https://godbolt.org/ with x86-64 gc 1.22.6.

func divmod1(f float64) (q, r uint64) {
	n := roundToUint64(f * 10)
	return (n * d1) >> e1, n - (n*d1)>>e1*10
	// return n / 10, n % 10
}

func divmod2(f float64) (uint64, uint64) {
	n := roundToUint64(f * 100)
	return n / 100, n % 100
	// MOVQ CX, BX
	// SHRQ $1, CX
	// MOVQ $-6640827866535438581, AX
	// MULQ CX
	// SHRQ $5, DX
	// IMUL3Q $100, DX, CX
	// SUBQ CX, BX
	// MOVQ DX, AX
	// RET
}

func divmod3(f float64) (q, r uint64) {
	n := roundToUint64(f * 1000)
	return n / 1000, n % 1000
}

func divmod4(f float64) (q, r uint64) {
	n := roundToUint64(f * 10000)
	return n / 10000, n % 10000
	// MOVQ $-3335171328526686932, AX
	// MULQ BX
	// SHRQ $13, DX
	// IMUL3Q $10000, DX, CX
	// SUBQ CX, BX
	// MOVQ DX, AX
	// RET
}

func divmod5(f float64) (q, r uint64) {
	n := roundToUint64(f * 100000)
	return n / 100000, n % 100000
}

func divmod6(f float64) (q, r uint64) {
	n := roundToUint64(f * 1000000)
	return n / 1000000, n % 1000000
}

// Ftoa appends float64 number f as decimal ascii digits to byte slice b.
func Ftoa(b []byte, f float64, dec int, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	if f >= 1e8 || dec > 8 {
		b = append(b, strconv.FormatFloat(f, 'f', dec, 64)...)
		return append(b, sep)
	}
	if dec <= 0 {
		return append(utoa8(b, roundToUint64(f)), sep) // no dot
	}
	w := u64pow10[dec]
	n := roundToUint64(f * f64pow10[dec])
	q := n / w
	b = utoa8(b, q)
	b = append(b, dot)
	b = utoa8Dec(b, n-q*w, dec)
	return append(b, sep)
}

// ftoaFull appends float64 number f as decimal ascii digits to byte slice b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number.
// dec is number of decimals.
func FtoaFull(b []byte, f float64, dec int, sep byte) []byte {
	if dec > 17 {
		dec = 17
	}
	if f > maxDecimal || dec <= 0 {
		return ftoItoa(b, f, sep)
	}
	f *= f64pow10[dec]
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
	for f > maxDecimal {
		f /= 10
		zeros++
	}
	b = utoa20(b, roundToUint64(f))
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

// Ftoa80 is a faster special Ftoa for f < 1e8 and 0 decimals
func Ftoa80(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	b = utoa8(b, uint64(int64(f+half)))
	return append(b, sep) // no dot
}

// Ftoa81 is a faster special Ftoa for f < 1e8 and 1 decimal
func Ftoa81(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod1(f)
	b = utoa8(b, q)
	return append(b, dot, byte('0'+r), sep)
}

// Ftoa82 is a faster special Ftoa for two decimals and f < 1e8.
// s = Ftoa82(s, f, 0) is 45 x faster for f=12.34 than
// s = append(s, strconv.FormatFloat(f, 'f', 2, 64)...)
// s = append(s, 0)
func Ftoa82(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod2(f)
	b = utoa8(b, q)
	return utoaDec2(b, r, sep) // dot, digits, sep
}

// Ftoa83 is a faster special Ftoa for three decimals and f < 1e8.
func Ftoa83(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod3(f)
	b = utoa8(b, q)
	return utoaDec3(b, r, sep)
}

// Ftoa84 is a faster special Ftoa for four decimals and f < 1e8.
func Ftoa84(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod4(f)
	b = utoa8(b, q)
	return utoaDec4(b, r, sep)
}

// Ftoa85 is a faster special Ftoa for five decimals and f < 1e8.
func Ftoa85(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod5(f)
	b = utoa8(b, q)
	return utoaDec5(b, r, sep)
}

// Ftoa86 is a faster special Ftoa for six decimals and f < 1e8.
func Ftoa86(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod6(f)
	b = utoa8(b, q)
	// utoa6Dec would be too big to be inlined.
	q1, q2, q3, q4, q5 := (r*d1)>>e1, (r*d2)>>e2, (r*d3)>>e3, (r*d4)>>e4, (r*d5)>>e5
	return append(b, dot,
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
	return append(utoa20(b, uint64(n)), sep)
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

func Utoa(b []byte, n uint64, dec int, sep byte) []byte {
	return append(utoa(b, n), sep)
}

// utoa appends uint64 n to slice b as ascii digits.
func utoa(b []byte, n uint64) []byte {
	if n < 1e8 {
		return utoa8(b, n)
	}
	return utoa20(b, n)
}

// utoaDec appends uint64 n to slice b left zero padded to dec ascii digits.
func utoaDec(b []byte, n uint64, dec int) []byte {
	if dec < 8 && n < 1e8 {
		return utoa8Dec(b, n, dec)
	}
	return utoa20Dec(b, n, dec)
}

// utoa20 appends positive integer n as ascii digits to slice b.
func utoa20(b []byte, n uint64) []byte {
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

// utoa20Dec appends positive integer n to slice b left
// zero padded to dec digits.
func utoa20Dec(b []byte, n uint64, dec int) []byte {
	var t [20]byte
	i := len(t) - 1
	for n >= 10 || dec > 1 {
		q := n / 10
		t[i] = byte('0' + n - q*10)
		n = q
		dec--
		i--
	}
	t[i] = byte('0' + n)
	return append(b, t[i:]...)
}

const asciiZeros = 0x3030303030303030 // '0'== 0x30

// utoa8 appends positive integer n < 1e8 as ascii digits to slice b.
// utoa8 is 3+ x faster for 3 digists than utoa20 above. And 10+ x
// faster than b = append(b, strconv.Itoa(123)...)
func utoa8(b []byte, n uint64) []byte {
	var u uint64
	digits := 1
	for n >= 10 {
		u, n = (u+n-(n*d1)>>e1*10)<<8, (n*d1)>>e1
		digits++
	}
	u += n + asciiZeros //adds '0'= 0x30 to each byte
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+digits]
}

// utoa3 appends positive integer n < 100 as ascii digits to slice b.
// func utoa3(b []byte, n uint64) []byte {
// 	if n < 10 {
// 		return append(b, byte('0'+n))
// 	}
// 	q1, q2 := (n*d1)>>e1, (n*d2)>>e2
// 	if n < 100 {
// 		return append(b, byte('0'+q1), byte('0'+n-q1*10))
// 	}
// 	return append(b, byte('0'+q2), byte('0'+q1-q2*10), byte('0'+n-q1*10))
// }

// utoa8Dec appends positive integer n < 1e8 to slice b
// left zero padded to dec ascii digits.
func utoa8Dec(b []byte, n uint64, dec int) []byte {
	var u uint64
	digits := 1
	for n >= 10 || dec > 1 {
		u, n = (u+n-(n*d1)>>e1*10)<<8, (n*d1)>>e1
		digits++
		dec--
	}
	u += n + asciiZeros
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+digits]
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

// func utoaDec3(b []byte, n uint64, sep uint64) []byte { // 30% slower than utoaDec3
// 	q1, q2 := (n*d1)>>e1, (n*d2)>>e2
// 	u := dot + q2<<8 + (q1-q2*10)<<16 + (n-q1*10)<<24 + sep<<32 + 0x30303000
// 	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+5]
// }
// func utoa3(b []byte, n uint64) []byte { // 30% slower than utoaDec3
// 	q1, q2 := (n*d1)>>e1, (n*d2)>>e2
// 	u := q2 + (q1-q2*10)<<6 + (n-q1*10)<<16 + 0x30303000
// 	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+3]
// }

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
