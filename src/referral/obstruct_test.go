package referral

import (
	"fmt"
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
