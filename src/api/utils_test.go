package api

import (
	"fmt"
	"testing"
)

func TestGetCodeSelectionDigest(t *testing.T) {
	var rc = APIReferralCodeSelectionPayload{
		Code:       "ABCD",
		TraderAddr: "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
		CreatedOn:  1696166434,
		Signature:  ""}
	d, err := GetCodeSelectionDigest(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "69aed51714620168256cf910e5d81ef53e6b6224224069369900d5650b2b8b15" {
		t.Errorf("failed")
		return
	}
}
