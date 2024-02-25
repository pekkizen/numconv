package numconv

import (
	"encoding/binary"
)

var (
	TrimTrailingZeros    = false
	SpaceBeforePositives = false
)

const (
	maxDigits = 1e19
	maxUint64 = 1<<64 - 1
	half      = 0.49999999999999994 // better than 0.5
	// half                    = 0.5
)

// TrimZeros trims trailing zeros from byte sclic b.
// Zero adjacent to desimal dot is not removed.
// TrimZeros returns the trimmed slice []byte.
func TrimZeros(b []byte) []byte {

	for len(b) > 2 && b[len(b)-1] == '0' &&
		b[len(b)-2] != dot {
		b = b[:len(b)-1] //drop trailing zero
	}
	return b
}

// ftoaFull appends float64 number f as decimal ASCII digits to byte slice *b.
// sep is char code (eg. '\t' or ',' or ' ') appended to *b after number.
// dec is number of decimals.
func ftoaFull(b *[]byte, f float64, dec int, sep byte) {
	if dec > 18 {
		dec = 18
	}

	if f > 1e17 || dec == 0 { //no non noise decimals
		ftoItoa(b, f, sep)
		return
	}
	w := upow10[dec]
	f *= float64(w)
	for f > maxDigits { //drop noise decimals
		w /= 10
		f /= 10
		dec--
	}
	n := uint64(f + half)
	q := n / w
	utoa(b, q)
	*b = append(*b, dot)
	n -= q * w
	utoa_dec(b, n, dec)

	if TrimTrailingZeros {
		*b = TrimZeros(*b)
	}
	*b = append(*b, sep)
}

func handleSign(b *[]byte, f float64) float64 {
	if f >= 0 {
		if SpaceBeforePositives {
			*b = append(*b, ' ')
		}
		return f
	}
	*b = append(*b, '-')
	return -f
}

func Ftoa(b *[]byte, f float64, dec int, sep byte) {
	f = handleSign(b, f)
	if f >= 1e8 || dec > 8 {
		ftoaFull(b, f, dec, sep)
		return
	}
	if dec == 0 {
		utoa8(b, uint64(f+half))
		*b = append(*b, sep)
		return
	}
	w := upow10[dec]
	f *= float64(w)
	n := uint64(f + half)
	q := n / w
	utoa8(b, q)
	*b = append(*b, dot)
	n -= q * w
	utoa8_dec(b, n, dec)

	if TrimTrailingZeros {
		*b = TrimZeros(*b)
	}
	*b = append(*b, sep)
}

// ftoItoa appends positive float64 f to *b as integer with
// max 19 digits and possible trailing zeros. May later be
// changed to E-format for a lot of zeros.
func ftoItoa(b *[]byte, f float64, sep byte) {
	zeros := 0
	for f > 1e17 {
		f /= 10
		zeros++
	}
	utoa20(b, uint64(f+half))
	if zeros > 3 {
		*b = append(*b, 'e', '+')
		utoa8(b, uint64(zeros))
	} else {
		appendZeros(b, zeros)
	}
	*b = append(*b, sep)
}

// appendZeros appends zeros '0' to *b
func appendZeros(b *[]byte, zeros int) {
	var zb = [8]byte{'0', '0', '0', '0', '0', '0', '0', '0'}
	if zeros <= 8 {
		*b = append(*b, zb[:zeros]...)
		return
	}
	for zeros > 8 {
		*b = append(*b, zb[:]...)
		zeros -= 8
	}
	if zeros > 0 {
		*b = append(*b, zb[:zeros]...)
	}
}

// Ftoa82 is faster special Ftoa for two decimals and f < 1e8
// Ftoa82(&s, f, 0) is 25+ x faster for f=630.68 than
// s = append(s, strconv.FormatFloat(f, 'f', 2, 64)...)
func Ftoa82(b *[]byte, f float64, sep byte) {
	f = handleSign(b, f)
	n := uint64(f*100 + half)
	q := (n * div100) >> exp // q := n/100
	utoa8(b, q)
	n -= q * 100
	q = (n * div10) >> exp // q = n/10
	*b = append(*b, dot, byte('0'+q), byte('0'+n-q*10), sep)
}

// Itoa appends integer i to slice *b
func Itoa(b *[]byte, n int, sep byte) {
	if n < 0 {
		*b = append(*b, '-')
		n = -n
	}
	if n < 10 {
		*b = append(*b, byte('0'+n), sep)
		return
	}
	if n >= 1e8 {
		utoa20(b, uint64(n))
	} else {
		utoa8(b, uint64(n))
	}
	*b = append(*b, sep)
}

// Itoa8Pos appends positive integer i to slice *b.
// Positivity is not checked.
// Number i must be < 1e8. This is inlineable.
func Itoa8Pos(b *[]byte, i int, sep byte) {
	utoa8(b, uint64(i))
	*b = append(*b, sep)
}

// utoa appends positive uint64 n to slice *b.
func utoa(b *[]byte, n uint64) {
	if n >= 1e8 {
		utoa20(b, n)
		return
	}
	utoa8(b, n)
}

// utoa_dec appends positive uint64 n to slice *b left zero padded to wid digits.
func utoa_dec(b *[]byte, n uint64, wid int) {
	if wid > 8 || n >= 1e8 {
		utoa20_dec(b, n, wid)
		return
	}
	utoa8_dec(b, n, wid)
}

func utoa20(b *[]byte, n uint64) {
	var t [20]byte
	i := len(t) - 1
	for n >= 10 {
		q := n / 10
		t[i] = byte('0' + n - q*10)
		n = q
		i--
	}
	t[i] = byte('0' + n)
	*b = append(*b, t[i:]...)
}

func utoa20_dec(b *[]byte, n uint64, wid int) {
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
	*b = append(*b, t[i:]...)
}

const exp = 35
const div10 = (1<<exp)/10 + 1
const div100 = (1<<exp)/100 + 1

// (n * div100) >> exp == n/100 for n <= 1.07e9 ~ 2^30
// (n * div10) >> exp == n/10 for n <= 5.36e9 ~ 2^32.3
// For exp = 35, div10 = 3435973837 ~ 2^31.7
// For any exp, n cannot exceed div10 or div100 much.
// exp = 35, n > 2^32.3 -> (n * div10) overflows uint64.

// This is 2 x faster than utoa20 above.
func utoa8(b *[]byte, n uint64) {
	var u uint64
	d := 1
	for ; n >= 10; d++ {
		q := (n * div10) >> exp // q := n/10
		u |= n + '0' - q*10
		n = q
		u <<= 8
	}
	u |= n + '0'
	*b = binary.LittleEndian.AppendUint64(*b, u)
	*b = (*b)[:len(*b)-8+d]
}

func utoa8_dec(b *[]byte, n uint64, wid int) {
	var u uint64
	d := 1
	for ; n >= 10 || wid > 1; d++ {
		q := (n * div10) >> exp
		u |= n + '0' - q*10
		u <<= 8
		n = q
		wid--
	}
	u |= n + '0'
	if d > 8 {
		d = 8
	}
	*b = binary.LittleEndian.AppendUint64(*b, u)
	*b = (*b)[:len(*b)-8+d]
}

// func utoa2(b *[]byte, n uint64) {
// 	q := (n * div10) >> exp // q = n/10
// 	*b = append(*b, byte('0'+q), byte('0'+n-q*10))
// }

// func utoa4(b *[]byte, n uint64) {
// 	var u uint64
// 	d := 1
// 	for ; n >= 10; d++ {
// 		// q := n / 10
// 		q := (n * 3435973837) >> 35
// 		u |= n + '0' - q*10
// 		n = q
// 		u <<= 8
// 	}
// 	u |= n + '0'
// 	d += len(*b)
// 	*b = append(*b, byte(u), byte(u>>8), byte(u>>16), byte(u>>24))
// 	*b = (*b)[:d]
// }
