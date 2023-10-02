package api

import (
	"fmt"
	"referral-system/src/utils"
	"strings"
	"testing"
)

func TestGetCodeSelectionDigest(t *testing.T) {
	var rc = utils.APICodeSelectionPayload{
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

	var rc = utils.APICodeSelectionPayload{
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

func TestNewCodeDigest(t *testing.T) {
	var rc = utils.APICodePayload{
		Code:          "ABCD",
		ReferrerAddr:  "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		AgencyAddr:    "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     ""}
	d, err := GetCodeDigest(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "2ed027b173ffdfdc4d0e0e0d98e258d538cfdb0016e02b0b1c65ee8b1570c1c2" {
		t.Errorf("failed")
		return
	}
}
func TestRecoverAddrNewCode(t *testing.T) {
	var rc = utils.APICodePayload{
		Code:          "ABCD",
		ReferrerAddr:  "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		AgencyAddr:    "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     "0x11fd0995864812c3c8f55ddf15a04511213b99f5376e288d94a7aa2e903793e33abaeb6132621880cf1177bb3909c99625cfd1669d6f366597bbd63fa67671a81b"}
	d, err := RecoverCodeSigAddr(rc)
	if err != nil {
		t.Errorf("recover failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != strings.ToLower("0aB6527027EcFF1144dEc3d78154fce309ac838c") {
		t.Errorf("failed")
		return
	}
}

func TestRecoverReferralAddr(t *testing.T) {
	var rc = utils.APIReferPayload{
		ParentAddr:    "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		ReferToAddr:   "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     "0xf49ac0e85fe2c1c2f0598a02b1bd53078e74bc62354cdbdd827941dc9f9a777d6a2cd99ec660b72083f23a0417aa487b2fd0f4d620c728c752137df3ce12bf971c"}
	d, err := RecoverReferralSigAddr(rc)
	if err != nil {
		t.Errorf("recover failed order: %v", err)
		return
	}
	addrHex := fmt.Sprintf("%x", d)
	if addrHex != strings.ToLower("0aB6527027EcFF1144dEc3d78154fce309ac838c") {
		t.Errorf("failed")
		return
	}
}
