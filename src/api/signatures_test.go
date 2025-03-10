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

func TestGetCodeSelectionTypedDataHash(t *testing.T) {
	var rc = utils.APICodeSelectionPayload{
		Code:       "ABCD",
		TraderAddr: "0x337A3778244159F37C016196a8E1038A811a34C9",
		CreatedOn:  1696166434,
		Signature:  ""}
	d, err := GetCodeSelectionTypedDataHash(rc)
	if err != nil {
		t.Errorf("typed data failed: %v", err)
		return
	}
	hashHex := fmt.Sprintf("%x", d)
	if hashHex != "bb91571eb64e6a1ab52d60f6a1c9918c9335ee0a614489c6b9b690e183a9a91c" {
		t.Errorf("failed, hash:" + hashHex)
		return
	}
}

func TestRecoverCodeSelectSigAddr1(t *testing.T) {

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

func TestRecoverCodeSelectSigAddr2(t *testing.T) {

	var rc = utils.APICodeSelectionPayload{
		Code:       "DOUBLE_AG",
		TraderAddr: "0x337A3778244159F37C016196a8E1038A811a34C9",
		CreatedOn:  1724088163,
		Signature:  "0xfcc543727b2629db226c8fa0da936e35d4bad2594d69371dddfd185a5c63059f3bd8ff3224c7dec69856fb6f14cbc3135019a3df20918fe88a7e9f5964a662ee1b",
	}
	addr, err := RecoverCodeSelectSigAddr(rc)
	if err != nil {
		t.Errorf("failed:" + err.Error())
		return
	}
	if addr.String() != "0x337A3778244159F37C016196a8E1038A811a34C9" {
		t.Errorf("failed: wrong address recovered:" + addr.String())
	}
	fmt.Println(addr.String())
}

func TestNewCodeDigest(t *testing.T) {
	var rc = utils.APICodePayload{
		Code:          "ABCD",
		ReferrerAddr:  "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     ""}
	d, err := GetCodeDigest(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "61a01769ac972fff3f8b608e5ce62b2d9557306b2680ec2c8bcf4194ab7d6a87" {
		t.Errorf("failed")
		return
	}
}

func TestNewCodeTypedDataHash(t *testing.T) {
	var rc = utils.APICodePayload{
		Code:          "ABCD",
		ReferrerAddr:  "0x337A3778244159F37C016196a8E1038A811a34C9",
		CreatedOn:     1696166434,
		PassOnPercTDF: 333,
		Signature:     ""}
	d, err := GetCodeTypedDataHash(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "f97ad4e2794c669374c95e1374eec3eb47f3eeaea204b5d4e4038febdb065f20" {
		t.Errorf("failed: received:" + digestHex)
		return
	}
}

func TestRecoverAddrNewCode1(t *testing.T) {
	// payload below was signed using eip-712
	var rc = utils.APICodePayload{
		Code:          "MYTEST4",
		ReferrerAddr:  "0xA131CF69D5456142E96A92583E11bb7c123eaa26",
		CreatedOn:     1736784636,
		PassOnPercTDF: 2500,
		Signature:     "0x16de8b9b53b3475086733c558acce4add7776220a3fa01138e8109c513b1588d310cc77d1aae1f8f58b85a27eb2cf81a7da9360de60b3c48a4cedee861a304f01b"}
	d, err := RecoverCodeSigAddr(rc)
	if err != nil {
		t.Errorf("recover failed order: %v", err)
		return
	}
	// address.String() is capitalization normalized (checksum encoded)
	if d.String() != "0xA131CF69D5456142E96A92583E11bb7c123eaa26" {
		t.Errorf("failed: %v", d.String())
		return
	}
}

func TestRecoverAddrNewCode2(t *testing.T) {
	var rc = utils.APICodePayload{
		Code:          "ABCD",
		ReferrerAddr:  "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     "0xb11b9af69b85719093be154bd9a9a23792d1ecb64f70b34dd69fdbec6c7cdf7048d62c6a6d94ee9f65e78aafad2ea45d94765e285a18485b879f814fde17c6b01b"}
	// signature is different from case 1 (from typed data)
	d, err := RecoverCodeSigAddr(rc)
	if err != nil {
		t.Errorf("recover failed order: %v", err)
		return
	}
	addr := fmt.Sprintf("%x", d)
	if addr != strings.ToLower("0aB6527027EcFF1144dEc3d78154fce309ac838c") {
		t.Errorf("failed")
		return
	}
}

func TestGetReferralTypedDataHash(t *testing.T) {
	var rc = utils.APIReferPayload{
		ParentAddr:    "0x337A3778244159F37C016196a8E1038A811a34C9",
		ReferToAddr:   "0x863ad9ce46acf07fd9390147b619893461036194",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     "0xb52d4433677023c57eb2a56ca70cde2498154c88cd295451628298b71599032f408b4776911e977dea5122b82dce1e2615dc48fb360f5386b8d20fede2acf7d01b"}
	d, err := GetReferralTypedDataHash(rc)
	if err != nil {
		t.Errorf("digest failed order: %v", err)
		return
	}
	digestHex := fmt.Sprintf("%x", d)
	if digestHex != "b37f4cc03960b29db2240f8540372a4df8b76eb19d37da7379725c301d7a4d69" {
		t.Errorf("failed: received:" + digestHex)
		return
	}
}

func TestRecoverReferralAddr1(t *testing.T) {
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

func TestRecoverReferralAddr2(t *testing.T) {
	var rc = utils.APIReferPayload{
		ParentAddr:    "0x337A3778244159F37C016196a8E1038A811a34C9",
		ReferToAddr:   "0x863ad9ce46acf07fd9390147b619893461036194",
		CreatedOn:     1696166434,
		PassOnPercTDF: 225,
		Signature:     "0xb52d4433677023c57eb2a56ca70cde2498154c88cd295451628298b71599032f408b4776911e977dea5122b82dce1e2615dc48fb360f5386b8d20fede2acf7d01b"}
	d, err := RecoverReferralSigAddr(rc)
	if err != nil {
		t.Errorf("recover failed order: %v", err)
		return
	}
	addrHex := fmt.Sprintf("%x", d)
	if "0x"+addrHex != strings.ToLower(rc.ParentAddr) {
		t.Errorf("failed:" + addrHex)
		return
	}
}
