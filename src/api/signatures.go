package api

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"referral-system/src/utils"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	solsha3 "github.com/miguelmota/go-solidity-sha3"
)

func GetCodeSelectionDigest(rc utils.APICodeSelectionPayload) ([32]byte, error) {
	types := []string{"string", "address", "uint256"}
	addr := common.HexToAddress(rc.TraderAddr)
	ts := big.NewInt(int64(rc.CreatedOn))
	values := []interface{}{rc.Code, addr, ts}
	digest0, err := abiEncodeBytes32(types, values...)
	if err != nil {
		return [32]byte{}, err
	}
	var digestBytes32 [32]byte
	copy(digestBytes32[:], solsha3.SoliditySHA3(digest0))
	return digestBytes32, nil
}

func isValidEvmAddr(addr string) bool {
	// Define a regular expression pattern for Ethereum addresses
	// It should start with "0x" followed by 40 hexadecimal characters
	pattern := "^0x[0-9a-fA-F]{40}$"

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Check if the address matches the pattern
	return re.MatchString(addr)
}

// isCurrentTimestamp returns true if timestamp is within 5 minutes
// of UTC timestamp on server
func isCurrentTimestamp(ts uint32) bool {
	currentTime := time.Now().UTC().Unix()
	return ts > uint32(currentTime-60*5) && ts < uint32(currentTime+60*5)
}

func GetReferralDigest(rpl utils.APIReferPayload) ([32]byte, error) {
	types := []string{"address", "address", "uint32", "uint256"}
	addr := common.HexToAddress(rpl.ReferToAddr)
	addrP := common.HexToAddress(rpl.ParentAddr)
	ts := big.NewInt(int64(rpl.CreatedOn))
	values := []interface{}{addrP, addr, rpl.PassOnPercTDF, ts}
	digest0, err := abiEncodeBytes32(types, values...)
	if err != nil {
		return [32]byte{}, err
	}
	var digestBytes32 [32]byte
	copy(digestBytes32[:], solsha3.SoliditySHA3(digest0))
	return digestBytes32, nil
}

func GetCodeDigest(rpl utils.APICodePayload) ([32]byte, error) {
	types := []string{"string", "address", "uint32", "uint256"}
	addrA := common.HexToAddress(rpl.ReferrerAddr) // can be 0
	ts := big.NewInt(int64(rpl.CreatedOn))
	values := []interface{}{rpl.Code, addrA, rpl.PassOnPercTDF, ts}
	digest0, err := abiEncodeBytes32(types, values...)
	if err != nil {
		return [32]byte{}, err
	}
	var digestBytes32 [32]byte
	copy(digestBytes32[:], solsha3.SoliditySHA3(digest0))
	return digestBytes32, nil
}

// RecoverCodeSelectSigAddr recovers the address of a signed APICodeSelectionPayload
// which is sent when a trader selects their code
func RecoverCodeSelectSigAddr(ps utils.APICodeSelectionPayload) (common.Address, error) {
	// Hash the unsigned message using EIP-715
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"CodeSelect": []apitypes.Type{
				{Name: "Code", Type: "string"},
				{Name: "TraderAddr", Type: "address"},
				{Name: "CreatedOn", Type: "uint256"},
			},
		},
		Message: apitypes.TypedDataMessage{
			"Code":       ps.Code,
			"TraderAddr": ps.TraderAddr,
			"CreatedOn":  ps.CreatedOn,
		},
		PrimaryType: "CodeSelect",
	}

	typedDataHash, err := typedData.HashStruct("CodeSelect", typedData.Message)
	if err != nil {
		return common.Address{}, err
	}
	// try to recover
	addr, err := recoverEvmAddressEip715(string(typedDataHash), ps.Signature)

	if err == nil {
		return addr, err
	}

	// recovery using EIP-715 failed - try EIP-191
	digestBytes32, err := GetCodeSelectionDigest(ps)
	if err != nil {
		return common.Address{}, err
	}
	addr, err = recoverEvmAddressEip191(string(digestBytes32[:]), ps.Signature)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}

// RecoverReferralSigAddr recovers the address of a signed APIReferPayload
// which is sent when an agency/broker wants to pass on their referral
func RecoverReferralSigAddr(rpl utils.APIReferPayload) (common.Address, error) {
	// Hash the unsigned message using EIP-715
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"NewReferral": []apitypes.Type{
				{Name: "ParentAddr", Type: "address"},
				{Name: "ReferToAddr", Type: "address"},
				{Name: "PassOnPercTDF", Type: "uint32"},
				{Name: "CreatedOn", Type: "uint256"},
			},
		},
		Message: apitypes.TypedDataMessage{
			"ParentAddr":    rpl.ParentAddr,
			"ReferToAddr":   rpl.ReferToAddr,
			"PassOnPercTDF": rpl.PassOnPercTDF,
			"CreatedOn":     rpl.CreatedOn,
		},
		PrimaryType: "NewReferral",
	}

	typedDataHash, err := typedData.HashStruct("NewReferral", typedData.Message)
	if err != nil {
		return common.Address{}, err
	}
	// try to recover
	addr, err := recoverEvmAddressEip715(string(typedDataHash), rpl.Signature)

	if err == nil {
		return addr, err
	}

	// recovery using EIP-715 failed - try EIP-191
	digestBytes32, err := GetReferralDigest(rpl)
	if err != nil {
		return common.Address{}, err
	}
	addr, err = recoverEvmAddressEip191(string(digestBytes32[:]), rpl.Signature)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}

// RecoverCodeSigAddr recovers the address of a signed APICodePayload
// which is sent when a referrer creates their code
func RecoverCodeSigAddr(cp utils.APICodePayload) (common.Address, error) {
	// Hash the unsigned message using EIP-715
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"NewCode": []apitypes.Type{
				{Name: "Code", Type: "string"},
				{Name: "ReferrerAddr", Type: "address"},
				{Name: "PassOnPercTDF", Type: "uint32"},
				{Name: "CreatedOn", Type: "uint256"},
			},
		},
		Message: apitypes.TypedDataMessage{
			"Code":          cp.Code,
			"ReferrerAddr":  cp.ReferrerAddr,
			"PassOnPercTDF": cp.PassOnPercTDF,
			"CreatedOn":     cp.CreatedOn,
		},
		PrimaryType: "NewCode",
	}

	typedDataHash, err := typedData.HashStruct("NewCode", typedData.Message)
	if err != nil {
		return common.Address{}, err
	}
	// try to recover
	addr, err := recoverEvmAddressEip715(string(typedDataHash), cp.Signature)

	if err == nil {
		return addr, err
	}

	// recovery using EIP-715 failed - try EIP-191
	digestBytes32, err := GetCodeDigest(cp)
	if err != nil {
		return common.Address{}, err
	}
	addr, err = recoverEvmAddressEip191(string(digestBytes32[:]), cp.Signature)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}

func bytesFromHexString(hexNumber string) ([]byte, error) {
	data, err := hex.DecodeString(strings.TrimPrefix(hexNumber, "0x"))
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// func recoverEvmAddress(data string, signature string) (common.Address, error) {
// 	addr, err := recoverEvmAddressEip715(data, signature)
// 	if err == nil {
// 		return addr, nil
// 	}
// 	addr, err = recoverEvmAddressEip191(data, signature)
// 	if err != nil {
// 		return common.Address{}, err
// 	}
// 	return addr, nil
// }

func recoverEvmAddressEip191(data string, signature string) (common.Address, error) {
	// Hash the unsigned message using EIP-191
	hashedMessage := []byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(data)) + data)
	hash := crypto.Keccak256Hash(hashedMessage)

	decodedMessage := hexutil.MustDecode(signature)
	// Handles cases where EIP-115 is not implemented (most wallets don't implement it)
	if decodedMessage[64] == 27 || decodedMessage[64] == 28 {
		decodedMessage[64] -= 27
	}
	// Recover a public key from the signed message
	sigPublicKeyECDSA, err := crypto.SigToPub(hash.Bytes(), decodedMessage)
	if sigPublicKeyECDSA == nil {
		err = errors.New("could not get a public get from the message signature")
	}
	if err != nil {
		return common.Address{}, err
	}
	addr := crypto.PubkeyToAddress(*sigPublicKeyECDSA)
	return addr, nil
}

func recoverEvmAddressEip715(data string, signature string) (common.Address, error) {
	typedDataDomain := apitypes.TypedData{}
	domainSeparator, _ := typedDataDomain.HashStruct("EIP712Domain", nil) // not used

	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), data))
	hash := crypto.Keccak256Hash(rawData)

	decodedMessage := hexutil.MustDecode(signature)
	// Handles cases where EIP-115 is not implemented (most wallets don't implement it)
	if decodedMessage[64] == 27 || decodedMessage[64] == 28 {
		decodedMessage[64] -= 27
	}
	// Recover a public key from the signed message
	pubKeyRaw, err := crypto.Ecrecover(hash.Bytes(), decodedMessage)
	if pubKeyRaw == nil {
		err = errors.New("could not get a public key from the message signature")
	}
	if err != nil {
		return common.Address{}, err
	}

	pubKey, err := crypto.UnmarshalPubkey(pubKeyRaw)

	if err != nil {
		return common.Address{}, err
	}
	addr := crypto.PubkeyToAddress(*pubKey)
	return addr, nil
}

func WashCode(rawCode string) string {
	// Create a regular expression to match characters that are not a-z, A-Z, 0-9, _, or -
	regex := regexp.MustCompile("[^a-zA-Z0-9_-]")

	// Use the ReplaceAllString function to replace matching characters with an empty string
	cleanedCode := regex.ReplaceAllString(rawCode, "")

	// Convert the result to uppercase
	cleanedCode = strings.ToUpper(cleanedCode)

	return cleanedCode
}

func abiEncodeBytes32(types []string, values ...interface{}) ([]byte, error) {
	if len(types) != len(values) {
		return []byte{}, fmt.Errorf("number of types and values do not match")
	}
	byteSlice, err := abiEncode(types, values...)
	if err != nil {
		return []byte{}, err
	}
	return byteSlice, nil
}

// abiEncode encodes the provided types (e.g., string, uint256, int32) and
// corresponding values into a hex-string for EVM
func abiEncode(types []string, values ...interface{}) ([]byte, error) {
	if len(types) != len(values) {
		return []byte{}, fmt.Errorf("number of types and values do not match")
	}

	arguments := abi.Arguments{}
	for _, typ := range types {
		t, err := abi.NewType(typ, "", nil)
		if err != nil {
			return []byte{}, fmt.Errorf("failed to create ABI type: %v", err)
		}
		arguments = append(arguments, abi.Argument{Type: t})
	}

	bytes, err := arguments.Pack(values...)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to encode arguments: %v", err)
	}
	return bytes, nil

}
