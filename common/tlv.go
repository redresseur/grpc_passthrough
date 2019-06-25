package common

import "errors"

const (
	TYPE_BYTES  =  1
	LENGTH_BYTES = 2 // 0~ 2^16
)

const (
	// tlv - type
	JSON = 1 << iota
	PROTOBUFFER
)

var (
	ErrLengthIsInvalid  = errors.New("the length is invalid")
	ErrLengthIsNotEnough = errors.New("the length is not Enough")
)

type TLV []byte

func TLVAppend(t TLV, b ...byte ) TLV {
	t = append(t, b...)
	l := int(t[1]) << 8 + int(t[2])
	l += len(b)

	t[1] = byte( (l & 0xFF00) >> 8)
	t[2] = byte(l & 0xFF)

	return TLV(t)
}

func (t TLV)Append() {

}

func (t TLV)Len() int {
	return int(t[1]) << 8 + int(t[2])
}

func (t TLV)Type() byte {
	return t[0]
}

func (t TLV)Value() []byte  {
	start := TYPE_BYTES + LENGTH_BYTES
	end := TYPE_BYTES + LENGTH_BYTES + t.Len()
	return t[start: end]
}

func Marshal(t byte, data []byte) []byte  {
	res := make([]byte, TYPE_BYTES + LENGTH_BYTES)
	res[0] = t
	l := len(data)

	res[1] = byte( (l & 0xFF00) >> 8)
	res[2] = byte(l & 0xFF)
	res = append(res, data...)
	return res
}

func Unmarshal(data []byte)(TLV, error){
	t := TLV(data)

	if len(data) < TYPE_BYTES + LENGTH_BYTES{
		return t, ErrLengthIsNotEnough
	}

	l := len(data)
	l -= (TYPE_BYTES + LENGTH_BYTES)

	ll := int(t[1]) << 8 + int(t[2])
	if (ll < l){
		return t, ErrLengthIsNotEnough
	}

	return t, nil
}