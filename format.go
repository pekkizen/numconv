package numconv

import (
	"encoding/binary"
)

const (
	maxUint64  = 1<<64 - 1
	maxUint32  = 1<<32 - 1
	maxDecimal = 1e17 - 1
	half       = 0.49999999999999994 // more strconv.FormatFloat compatible than 0.5
)

// Constants for transforming integer div n/10, n/100 etc. to multiply and bitshift operation.
// (n * d1) >> e1 == n/10 for  n < 1<<32 ~ 4.2e9
// (n * d2) >> e1 == n/100 for n < 1<<32
// (n * d3) >> e3 == n/1e3 for n < 1<<32
// (n * d4) >> e4 == n/1e4 for n < 1<<32
// (n * d5) >> e5 == n/1e5 for n < 3e9  	!!
// (n * d6) >> e6 == n/1e6 for n < 1<<32
const (
	e1 = 32 + 3 // 32 + floor(log2(10) = 3.32...)
	e2 = 32 + 6 // 32 + floor(log2(100))
	e3 = 32 + 9 // etc.
	e4 = 32 + 13
	e5 = 32 + 16
	e6 = 32 + 19
	d1 = (1<<e1)/10 + 1
	d2 = (1<<e2)/100 + 1
	d3 = (1<<e3)/1000 + 1
	d4 = (1<<e4)/10000 + 1
	d5 = (1<<e5)/100000 + 1
	d6 = (1<<e6)/1000000 + 1
)

func divmod1(n uint64) (quo, rem uint64) {
	if n <= maxUint32 {
		return (n * d1) >> e1, n - (n*d1)>>e1*10
	}
	quo = n / 10
	return quo, n - quo*10
}
func divmod2(n uint64) (quo, rem uint64) {
	if n <= maxUint32 {
		return (n * d2) >> e2, n - (n*d2)>>e2*100
	}
	quo = n / 100
	return quo, n - quo*100
}
func divmod3(n uint64) (quo, rem uint64) {
	if n <= maxUint32 {
		return (n * d3) >> e3, n - (n*d3)>>e3*1000
	}
	quo = n / 1000
	return quo, n - quo*1000
}
func divmod4(n uint64) (quo, rem uint64) {
	if n <= maxUint32 {
		return (n * d4) >> e4, n - (n*d4)>>e4*1e4
	}
	quo = n / 1e4
	return quo, n - quo*1e4
}
func divmod5(n uint64) (quo, rem uint64) {
	if n < 3e9 { // 3e9 <  maxUint32 ~ 4.2e9 !!!
		return (n * d5) >> e5, n - (n*d5)>>e5*1e5
	}
	quo = n / 1e5
	return quo, n - quo*1e5
}
func divmod6(n uint64) (quo, rem uint64) {
	if n <= maxUint32 {
		return (n * d6) >> e6, n - (n*d6)>>e6*1e6

	}
	quo = n / 1e6
	return quo, n - quo*1e6
}

// Ftoa appends float64 number f as decimal ascii digits to byte slice b.
func Ftoa(b []byte, f float64, dec int, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	if f >= 1e8 || dec > 8 {
		return ftoaFull(b, f, dec, sep)
	}
	if dec <= 0 {
		return append(utoa8(b, uint64(f+half)), sep) // no dot
	}
	w := upow10[dec]
	n := uint64(f*fpow10[dec] + half)
	q := n / w
	b = utoa8(b, q)
	b = append(b, dot)
	b = utoa8Dec(b, n-q*w, dec)
	return append(b, sep)
}

// ftoaFull appends float64 number f as decimal ascii digits to byte slice b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number.
// dec is number of decimals.
func ftoaFull(b []byte, f float64, dec int, sep byte) []byte {
	if dec > 17 {
		dec = 17
	}
	if f > maxDecimal || dec <= 0 {
		return ftoItoa(b, f, sep)
	}
	f *= fpow10[dec]
	for f > maxDecimal {
		f /= 10
		dec--
	}
	w := upow10[dec] // 10^dec
	n := uint64(f + half)
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
	b = utoa20(b, uint64(f+half))
	if zeros > 3 {
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
// This is inlineable.
func Ftoa80(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	return append(utoa8(b, uint64(f+half)), sep) // no dot
}

func Ftoa81(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod1(uint64(f*10 + half))
	b = utoa8(b, q)
	return append(b, dot, byte('0'+r), sep)
}

// Ftoa82 is a faster special Ftoa for two decimals and f < 1e8.
// s = Ftoa82(s, f, 0) is 40 x faster for f=12.34 than
// s = append(s, strconv.FormatFloat(f, 'f', 2, 64)...)
// s = append(s, 0)
func Ftoa82(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod2(uint64(f*100 + half))
	b = utoa8(b, q)
	return utoaDec2(b, r, sep) // dot, digits, sep
}

// Ftoa83 is a faster special Ftoa for three decimals and f < 1e8.
func Ftoa83(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod3(uint64(f*1e3 + half))
	b = utoa8(b, q)
	return utoaDec3(b, r, sep)
}

// Ftoa84 is a faster special Ftoa for four decimals and f < 1e8.
func Ftoa84(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod4(uint64(f*1e4 + half))
	b = utoa8(b, q)
	return utoaDec4(b, r, sep)
}

// Ftoa85 is a faster special Ftoa for five decimals and f < 1e8.
func Ftoa85(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod5(uint64(f*1e5 + half))
	b = utoa8(b, q)
	return utoaDec5(b, r, sep)
}

// Ftoa86 is a faster special Ftoa for six decimals and f < 1e8.
func Ftoa86(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := divmod6(uint64(f*1e6 + half))
	b = utoa8(b, q)
	// utoa6Dec is too big to be inlined. Manually inlined here.
	q1, q2, q3, q4, q5 := (r*d1)>>e1, (r*d2)>>e2, (r*d3)>>e3, (r*d4)>>e4, (r*d5)>>e5
	return append(b, dot,
		byte('0'+q5),
		byte('0'+q4-q5*10),
		byte('0'+q3-q4*10),
		byte('0'+q2-q3*10),
		byte('0'+q1-q2*10),
		byte('0'+r-q1*10), sep)
}

// Itoa appends integer i to slice b as ascii digits.
func Itoa(b []byte, n int, sep byte) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	if n < 1e8 {
		return append(utoa8(b, uint64(n)), sep)
	}
	return append(utoa20(b, uint64(n)), sep)
}

func Itoa8(b []byte, n int, sep byte) []byte {
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
// This is a kind of standard way to do this.
func utoa20(b []byte, n uint64) []byte {
	// const d = maxUint64/10 + 1
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

// utoa20Dec appends positive integer n to slice b left zero padded to dec slice digits.
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
// utoa8 is 3 x faster than utoa20 above. And 10 x faster
// than b = append(b, strconv.FormatInt(int64(n), 10)...)
func utoa8(b []byte, n uint64) []byte {
	var u uint64
	digits := 1
	for n >= 10 {
		u, n = (u+n-(n*d1)>>e1*10)<<8, (n*d1)>>e1
		// u = (u + n - (n*d1)>>e1*10) << 8
		// n = (n * d1) >> e1
		digits++
	}
	u += n + asciiZeros //adds '0'= 0x30 to each byte
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+digits]
}

// go test -bench=Utoa8 -test.run=xxx -benchtime=2000000000x
// go test -bench=Butoa8 -test.run=xxx -benchtime=2000000000x

// utoa8Dec appends positive integer n < 1e8 to slice b
// left zero padded to dec ascii digits.
func utoa8Dec(b []byte, n uint64, dec int) []byte {
	var u uint64
	digits := 1
	for n >= 10 || dec > 1 {
		u, n = (u+n-(n*d1)>>e1*10)<<8, (n*d1)>>e1
		// u = (u + n - (n*d1)>>e1*10) << 8
		// n = (n * d1) >> e1
		digits++
		dec--
	}
	u += n + asciiZeros //adds '0'= 0x30 to each byte
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
