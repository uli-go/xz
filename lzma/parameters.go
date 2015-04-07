package lzma

import (
	"io"

	"github.com/uli-go/xz/lzbase"
)

// Parameters contain all information required to decode or encode an LZMA
// stream.
//
// The DictSize will be limited by MaxInt32 on 32-bit platforms.
type Parameters struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// size of the dictionary in bytes
	DictSize int64
	// size of uncompressed data in bytes
	Size int64
	// header includes unpacked size
	SizeInHeader bool
	// end-of-stream marker requested
	EOS bool
	// buffer size
	BufferSize int64
}

// Properties returns LC, LP and PB as Properties value.
func (p *Parameters) Properties() lzbase.Properties {
	props, err := lzbase.NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LP and PB fields.
func (p *Parameters) SetProperties(props lzbase.Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// normalizeSize puts the size on a normalized size. If DictSize and BufferSize
// are zero, then it is set to the value in Default. If both size values are
// too small they will set to the minimum size possible. Note that a buffer
// size less then zero will be ignored and will cause an error by
// verifyParameters.
func normalizeSizes(p *Parameters) {
	if p.DictSize == 0 {
		p.DictSize = Default.DictSize
	}
	if p.DictSize < lzbase.MinDictSize {
		p.DictSize = lzbase.MinDictSize
	}
	if p.BufferSize == 0 {
		p.BufferSize = Default.BufferSize
	}
	if 0 <= p.BufferSize && p.BufferSize < lzbase.MinLength {
		p.BufferSize = lzbase.MaxLength
	}
}

// verifyParameters checks parameters for errors.
func verifyParameters(p *Parameters) error {
	if p == nil {
		return newError("parameters must be non-nil")
	}
	if err := lzbase.VerifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(lzbase.MinDictSize <= p.DictSize &&
		p.DictSize <= lzbase.MaxDictSize) {
		return newError("DictSize out of range")
	}
	hlen := int(p.DictSize)
	if hlen < 0 {
		return newError("DictSize cannot be converted into int")
	}
	if p.Size < 0 {
		return newError("length must not be negative")
	}
	return nil
}

// Default defines the parameters used by NewWriter.
var Default = Parameters{
	LC:         3,
	LP:         0,
	PB:         2,
	DictSize:   lzbase.MinDictSize,
	BufferSize: 4096,
}

// getUint32LE reads an uint32 integer from a byte slize
func getUint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// putUint32LE puts an uint32 integer into a byte slice that must have at least
// a lenght of 4 bytes.
func putUint32LE(b []byte, x uint32) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// noHeaderLen defines the value of the length field in the LZMA header.
const noHeaderLen uint64 = 1<<64 - 1

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Parameters, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	p = new(Parameters)
	props := lzbase.Properties(b[0])
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
	p.DictSize = int64(getUint32LE(b[1:]))
	u := getUint64LE(b[5:])
	if u == noHeaderLen {
		p.Size = 0
		p.EOS = true
		p.SizeInHeader = false
	} else {
		p.Size = int64(u)
		if p.Size < 0 {
			return nil, newError(
				"unpack length in header not supported by" +
					" int64")
		}
		p.EOS = false
		p.SizeInHeader = true
	}

	normalizeSizes(p)
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Parameters) error {
	var err error
	if err = verifyParameters(p); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = byte(p.Properties())
	if p.DictSize > lzbase.MaxDictSize {
		return newError("DictSize exceeds maximum value")
	}
	putUint32LE(b[1:5], uint32(p.DictSize))
	var l uint64
	if p.SizeInHeader {
		l = uint64(p.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err = w.Write(b)
	return err
}
