package api

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	solsha3 "github.com/miguelmota/go-solidity-sha3"
)

func GetCodeSelectionDigest(rc APIReferralCodeSelectionPayload) ([32]byte, error) {
	types := []string{"string", "address", "uint256"}
	addr := common.HexToAddress(rc.TraderAddr)
	ts := big.NewInt(int64(rc.CreatedOn))
	values := []interface{}{rc.Code, addr, ts}
	digest0, err := AbiEncodeBytes32(types, values...)
	if err != nil {
		return [32]byte{}, err
	}
	var digestBytes32 [32]byte
	copy(digestBytes32[:], solsha3.SoliditySHA3(digest0))
	return digestBytes32, nil
}

func BytesFromHexString(hexNumber string) ([]byte, error) {
	data, err := hex.DecodeString(strings.TrimPrefix(hexNumber, "0x"))
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func RecoverCodeSelectSigAddr(ps APIReferralCodeSelectionPayload) (common.Address, error) {
	digestBytes32, err := GetCodeSelectionDigest(ps)
	if err != nil {
		return common.Address{}, err
	}
	addr, err := RecoverEvmAddress(string(digestBytes32[:]), ps.Signature)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}

func RecoverEvmAddress(data string, signature string) (common.Address, error) {
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
		err = errors.New("Could not get a public get from the message signature")
	}
	if err != nil {
		return common.Address{}, err
	}
	addr := crypto.PubkeyToAddress(*sigPublicKeyECDSA)
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

func AbiEncodeBytes32(types []string, values ...interface{}) ([]byte, error) {
	if len(types) != len(values) {
		return []byte{}, fmt.Errorf("number of types and values do not match")
	}
	byteSlice, err := AbiEncode(types, values...)
	if err != nil {
		return []byte{}, err
	}
	return byteSlice, nil
}

// AbiEncode encodes the provided types (e.g., string, uint256, int32) and
// corresponding values into a hex-string for EVM
func AbiEncode(types []string, values ...interface{}) ([]byte, error) {
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
