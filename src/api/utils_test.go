package api

import (
	"fmt"
	"testing"
)

func TestGetCodeSelectionDigest(t *testing.T) {
	var rc = APIReferralCodeSelectionPayload{
		Code:       "ABCD",
		TraderAddr: "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		CreatedOn:  1696166434,
		Signature:  ""}
	d, err := GetCodeSelectionDigest(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "217e0f063913bf1b9e0f75554eb3b99f116b2657c599efb9ae22a01569fdbcf1" {
		t.Errorf("failed")
		return
	}
}

func TestRecoverCodeSelectSigAddr(t *testing.T) {

	var rc = APIReferralCodeSelectionPayload{
		Code:       "ABCD",
		TraderAddr: "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		CreatedOn:  1696166434,
		Signature:  "0x880e13963c71158fb677740f7c3e645b9d7d856a7fa3f006a8b38bac3be3feb669a7bb3dd054e2b509104a5650614bdbde534dd91331a93e742bd0e8d985c4ed1c",
	}
	addr, err := RecoverCodeSelectSigAddr(rc)
	if err != nil {
		t.Errorf("failed:" + err.Error())
		return
	}
	if addr.String() != "0x0aB6527027EcFF1144dEc3d78154fce309ac838c" {
		t.Errorf("failed: wrong address recovered")
	}
	fmt.Println(addr.String())
}
