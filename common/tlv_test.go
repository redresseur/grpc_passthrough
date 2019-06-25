package common

import "testing"

func TestMarshal(t *testing.T) {
	tm := Marshal(2, []byte("test"))
	tum, _ := Unmarshal(tm)
	t.Logf("type %d len %d data %s", tum.Type(), tum.Len(), string(tum.Value()))
}

func TestTLV_Append(t *testing.T) {
	tm := Marshal(2, []byte("test"))
	tum, _ := Unmarshal(tm)
	tum = TLVAppend(tum, []byte("test")...)
	t.Logf("type %d len %d data %s", tum.Type(), tum.Len(), string(tum.Value()))
}