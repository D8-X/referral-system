package referral

import (
	"fmt"
	"strconv"
	"testing"
)

func TestObstructCode(t *testing.T) {
	txt := "HELLO_THIS-IS_CODE"
	code := obstructCode(txt)
	fmt.Println("coded = ", code)
	decoded := deObstructCode(code)
	fmt.Println("decoded = ", decoded)
	if txt != decoded {
		t.Error("encoding/decoding failed")
	}
}

func TestObstructCode2(t *testing.T) {
	txt := "INVALID&^sCHARS*"
	txt2 := "INVALID---CHARS-"
	code := obstructCode(txt)
	fmt.Println("coded = ", code)
	decoded := deObstructCode(code)
	fmt.Println("decoded = ", decoded)
	if txt2 != decoded {
		t.Error("encoding/decoding failed")
	}
}

func TestEncodePaymentInfo(t *testing.T) {
	batchTs := "1699702424"
	code := "HELLO-123_"
	poolId := 1
	encoded := encodePaymentInfo(batchTs, code, poolId)
	fmt.Println("encoded msg = ", encoded)
	decoded := decodePaymentInfo(encoded)
	fmt.Println("decoded msg = ", encoded)
	if batchTs != decoded[0] {
		t.Error("encoding/decoding failed")
	}
	if code != decoded[1] {
		t.Error("encoding/decoding failed")
	}
	if strconv.Itoa(poolId) != decoded[2] {
		t.Error("encoding/decoding failed")
	}
	// send some rubbish
	res := decodePaymentInfo("...")
	fmt.Println(res)
}

func TestEncodePatternInfo(t *testing.T) {
	//batchTs.<code>.<poolId>.<encodingversion>
	v := isV1Pattern("1699702424.ABRAKADABRA-_--7.1.1")
	if !v {
		t.Error("pattern 1 failed")
	}
	v = isV1Pattern("1699702424.ABRAKADABRA-_--7.1.22")
	if v {
		t.Error("pattern 1 failed")
	}
	v = isV0Pattern("1699702424.ABRAKADABRA-_--7.1.1")
	if v {
		t.Error("pattern 0 failed")
	}

	v0Code := "1699702424.ABRAKADABRA-_--7.1"
	v = isV1Pattern(v0Code)
	if v {
		t.Error("pattern 1 failed")
	}
	v = isV0Pattern(v0Code)
	if !v {
		t.Error("pattern 0 failed")
	}
}
