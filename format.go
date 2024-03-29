package numconv

import (
	"encoding/binary"
)

const (
	maxUint64  = 1<<64 - 1
	maxDecimal = 1e17 - 1
	half       = 0.49999999999999994 // better than 0.5
)

// TrimZeros trims trailing zeros from byte slice b.
// Zero adjacent to desimal dot is not removed.
// TrimZeros returns the trimmed slice []byte.
func TrimZeros(b []byte) []byte {

	for len(b) > 2 && b[len(b)-1] == '0' && b[len(b)-2] != dot {
		b = b[:len(b)-1] //drop trailing zero
	}
	return b
}

// ftoaFull appends float64 number f as decimal ascii digits to byte slice b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number.
// dec is number of decimals.
func ftoaFull(b []byte, f float64, dec int, sep byte) []byte {
	if dec > 17 {
		dec = 17
	}
	if f > maxDecimal || dec == 0 {
		return ftoItoa(b, f, sep)
	}
	f *= fpow10[dec]
	for f > maxDecimal {
		f /= 10
		dec--
	}
	w := upow10[dec] //is possible dec < 0?
	n := uint64(f + half)
	q := n / w
	b = utoa(b, q)
	b = append(b, dot)
	b = utoaDec(b, n-q*w, dec)
	return append(b, sep)
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
		b = utoa8(b, uint64(f+half))
		return append(b, sep)
	}
	w := upow10[dec]
	n := uint64(f*fpow10[dec] + half)
	q := n / w
	b = utoa8(b, q)
	b = append(b, dot)
	b = utoa8Dec(b, n-q*w, dec)
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
	var zb = [8]byte{'0', '0', '0', '0', '0', '0', '0', '0'}
	if zeros <= 8 {
		return append(b, zb[:zeros]...)
	}
	for zeros > 8 {
		b = append(b, zb[:]...)
		zeros -= 8
	}
	if zeros > 0 {
		b = append(b, zb[:zeros]...)
	}
	return b
}
func div100(n uint64) (q, r uint64) {
	if n < 1<<32 {
		return (n * d2) >> e2, n - (n*d2)>>e2*100
	}
	q = n / 100
	return q, n - q*100
}

func div1e3(n uint64) (q, r uint64) {
	if n < 1<<32 {
		return (n * d3) >> e3, n - (n*d3)>>e3*1000
	}
	q = n / 1000
	return q, n - q*1000
}
func div1e4(n uint64) (q, r uint64) {
	if n < 1<<32 {
		return (n * d4) >> e4, n - (n*d4)>>e4*1e4
	}
	q = n / 1e4
	return q, n - q*1e4
}
func div1e5(n uint64) (q, r uint64) {
	if n < 1e9 {
		return (n * d5) >> e5, n - (n*d5)>>e5*1e5
	}
	q = n / 1e5
	return q, n - q*1e5
}
func div1e6(n uint64) (q, r uint64) {
	if n < 1e9 {
		return (n * d6) >> e6, n - (n*d6)>>e6*1e6
	}
	q = n / 1e6
	return q, n - q*1e6
}

// Ftoa82 is a faster special Ftoa for two decimals and f < 1e8.
// s = Ftoa82(s, f, 0) is 35 x faster for f=630.68 than
// s = append(s, strconv.FormatFloat(f, 'f', 2, 64)...)
func Ftoa82(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := div100(uint64(f*100 + half))
	b = utoa8(b, q)
	return append(b, dot,
		byte('0'+(r*d1)>>e1),
		byte('0'+r-(r*d1)>>e1*10), sep)
}

// Ftoa83 is a faster special Ftoa for three decimals and f < 1e8.
func Ftoa83(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := div1e3(uint64(f*1e3 + half))
	b = utoa8(b, q)
	b = append(b, dot)
	b = utoa3Dec(b, r)
	return append(b, sep)
}

// Ftoa84 is a faster special Ftoa for four decimals and f < 1e8.
func Ftoa84(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := div1e4(uint64(f*1e4 + half))
	b = utoa8(b, q)
	b = append(b, dot)
	b = utoa4Dec(b, r)
	return append(b, sep)
}

// Ftoa85 is a faster special Ftoa for five decimals and f < 1e8.
func Ftoa85(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := div1e5(uint64(f*1e5 + half))
	b = utoa8(b, q)
	b = append(b, dot)
	q1, q2, q3, q4 :=
		(r*d1)>>e1, (r*d2)>>e2, (r*d3)>>e3, (r*d4)>>e4
	u := (r-q1*10)<<32 +
		(q1-q2*10)<<24 +
		(q2-q3*10)<<16 +
		(q3-q4*10)<<8 +
		q4 + asciiZeros
	b = binary.LittleEndian.AppendUint64(b, u)[:len(b)+5]
	return append(b, sep)
}

// Ftoa86 is a faster special Ftoa for six decimals and f < 1e8.
func Ftoa86(b []byte, f float64, sep byte) []byte {
	if f < 0 {
		b = append(b, '-')
		f = -f
	}
	q, r := div1e6(uint64(f*1e6 + half))
	b = utoa8(b, q)
	b = append(b, dot)
	q1, q2, q3, q4, q5 :=
		(r*d1)>>e1, (r*d2)>>e2, (r*d3)>>e3, (r*d4)>>e4, (r*d5)>>e5
	u := (r-q1*10)<<40 +
		(q1-q2*10)<<32 +
		(q2-q3*10)<<24 +
		(q3-q4*10)<<16 +
		(q4-q5*10)<<8 +
		q5 + asciiZeros
	b = binary.LittleEndian.AppendUint64(b, u)[:len(b)+6]
	return append(b, sep)
}

// Itoa appends integer i to slice b as ascii digits.
func Itoa(b []byte, n int, sep byte) []byte {
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	if n < 1e8 {
		b = utoa8(b, uint64(n))
	} else {
		b = utoa20(b, uint64(n))
	}
	return append(b, sep)
}

// Itoa8Pos appends positive integer i < 1e8 to slice b as ascii digits.
// Positivity is not checked. Number i must be < 1e8. This is inlineable.
func Itoa8Pos(b []byte, i int, sep byte) []byte {
	b = utoa8(b, uint64(i))
	return append(b, sep)
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
	if dec > 8 || n >= 1e8 {
		return utoa20Dec(b, n, dec)
	}
	return utoa8Dec(b, n, dec)
}

// utoa20 appends positive integer n as ascii digits to slice b.
// utoa20 is adapted from \src\internal\itoa\itoa.go.
// It returns []byte instead of string.
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

// Constants for transforming n/10, n/100 and n/1000 to multiply and bitshift operation.
// (n * d1) >> e1 == n/10 for n <= 1<<32-1 ~ 4.2e9
// (n * d2) >> e1 == n/100 for n <= 1<<32-1
// (n * d3) >> e3 == n/1e3 for n <= 1<<32-1
// (n * d4) >> e4 == n/1e4 for n <= 1<<32-1
// (n * d5) >> e5 == n/1e5 for n <= 1e9
// (n * d6) >> e6 == n/1e6 for n <= 1e9

const (
	e1 = 35 // 32 + floor(log2(10))
	e2 = 38 // 32 + floor(log2(100))
	e3 = 41 // 32 + floor(log2(1000))
	e4 = 45 // etc.
	e5 = 48
	e6 = 52
	d1 = (1<<e1)/10 + 1
	d2 = (1<<e2)/100 + 1
	d3 = (1<<e3)/1000 + 1
	d4 = (1<<e4)/10000 + 1
	d5 = (1<<e5)/100000 + 1
	d6 = (1<<e6)/1000000 + 1
)
const asciiZeros = 0x3030303030303030 // '0'== 0x30

// utoa8 appends positive integer n < 1e8 as ascii digits to slice b.
// utoa8 is 2.5 x faster than utoa20 above. And 8 x faster
// than b = append(b, strconv.FormatInt(int64(n), 10)...)
func utoa8(b []byte, n uint64) []byte {
	digits := 1
	u := uint64(0)
	for n >= 10 {
		u = (u + n - (n*d1)>>e1*10) << 8
		n = (n * d1) >> e1
		digits++
	}
	u += n +
		asciiZeros //adds '0'= 0x30 to each byte
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+digits]
}

// utoa8Dec appends positive integer n < 1e8 to slice b
// left zero padded to dec ascii digits.
func utoa8Dec(b []byte, n uint64, dec int) []byte {
	digits := 1
	u := uint64(0)
	for n >= 10 || dec > 1 {
		u = (u + n - (n*d1)>>e1*10) << 8
		n = (n * d1) >> e1
		dec--
		digits++
	}
	u += n + asciiZeros
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+digits]
}

// utoa3Dec appends n < 1000 as three left zero padded ascii digits to slice b.
func utoa3Dec(b []byte, n uint64) []byte {
	q1, q2 := (n*d1)>>e1, (n*d2)>>e2
	u := (n-q1*10)<<16 +
		(q1-q2*10)<<8 +
		q2 + 0x303030
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+3]
}

// utoa4Dec appends n < 10000 as four left zero padded ascii digits to slice b.
func utoa4Dec(b []byte, n uint64) []byte {
	q1, q2, q3 := (n*d1)>>e1, (n*d2)>>e2, (n*d3)>>e3
	u := (n-q1*10)<<24 +
		(q1-q2*10)<<16 +
		(q2-q3*10)<<8 +
		q3 + 0x30303030
	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+4]
}

// func utoa4Dec(b []byte, n uint64) []byte {
// 	u := (n-(n*d1)>>e1*10)<<24 +
// 		((n*d1)>>e1-(n*d2)>>e2*10)<<16 +
// 		((n*d2)>>e2-(n*d3)>>e3*10)<<8 +
// 		(n*d3)>>e3 +
// 		0x30303030
// 	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+4]
// }

// func utoa3Dec(b []byte, n uint64) []byte {
// 	u := (n-(n*d1)>>e1*10)<<16 +
// 		((n*d1)>>e1-(n*d2)>>e2*10)<<8 +
// 		(n*d2)>>e2 +
// 		0x303030
// 	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+3]
// }

// func utoa2Dec(b []byte, n uint64) []byte {
// 	u := (n-(n*d1)>>e1*10)<<8 +
// 		(n*d1)>>e1 +
// 		0x3030
// 	return binary.LittleEndian.AppendUint64(b, u)[:len(b)+2]
// }
