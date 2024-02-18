package referral

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v4"
	"github.com/square/go-jose/v3"
)

type SocialWallet struct {
	PublicKey string `json:"public_key"` // compressed public key derived based on the specified curve
	Type      string `json:"type"`       //"web3auth_key" incase of social logins
	Curve     string `json:"curve"`      //"secp256k1" (default) or "ed25519" You can specify which curve you want use for the encoded public key in the login parameters
}
type APISocialIdToken struct {
	Iat               int            `json:"iat"`               //issued at Unix sec
	Aud               string         `json:"aud"`               //audience
	Iss               string         `json:"iss"`               //issuer
	Email             string         `json:"email"`             //optional
	Name              string         `json:"name"`              //optional
	ProfileImage      string         `json:"profileImage"`      //optional
	Verifier          string         `json:"verifier"`          //Web3auth's verifier used while user login
	VerifierId        string         `json:"verifierId"`        //Unique user id given by OAuth login provider
	AggregateVerifier string         `json:"aggregateVerifier"` //Name of the verifier if you are using a single id verifier (aggregateVerifier) (optional)
	Exp               int            `json:"exp"`               //The "exp" (expiration time) claim identifies the expiration time on or after which the JWT MUST NOT be accepted for processing.
	Wallets           []SocialWallet `json:"wallets"`
}

// Define a struct to represent the JSON Web Key (JWK) Set
type JWKSet struct {
	Keys []json.RawMessage `json:"keys"`
}

// RegisterSocialUser checks the validity of the user login information (web3auth)
// and if a new valid twitter user stores the id/address mapping in the DB and
// makes the Twitter ranking
func (rs *SocialSystem) RegisterSocialUser(tokenString, walletAddr string) error {
	addr, payload, err := verifyWeb3Auth(tokenString, walletAddr)
	if err != nil {
		return err
	}
	// "twitter|1602945858105917441"
	id := strings.Split(payload.VerifierId, "|")
	if id[0] != "twitter" {
		slog.Info("Non-Twitter user signed in")
		return nil
	}
	twitterId := id[1]
	return rs.SignUpSocialUser(twitterId, addr.Hex())
}

// verifyWeb3Auth verifies the JWT token from web3auth and returns the EVM-address and the payload if
// valid
func verifyWeb3Auth(tokenString string, walletAddr string) (*common.Address, *APISocialIdToken, error) {

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return web3AuthKeyFunc(token)
	})

	if !token.Valid {
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, nil, errors.New("Token malformed:" + err.Error())
		} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			// Token is either expired or not active yet
			return nil, nil, errors.New("token expired")
		} else {
			return nil, nil, errors.New("Could not handle token:" + err.Error())
		}
	}
	jsonData := token.Claims.(jwt.MapClaims)
	// Convert JSON data to byte slice
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, nil, errors.New("Error marshalling JSON:" + err.Error())
	}
	var payload APISocialIdToken
	// Unmarshal JSON data into the struct variable
	err = json.Unmarshal(jsonBytes, &payload)
	if err != nil {
		return nil, nil, errors.New("Error unmarshalling JSON:" + err.Error())
	}

	pk := payload.Wallets[0].PublicKey
	publicKeyBytes, err := hex.DecodeString(pk)
	if err != nil {
		return nil, nil, errors.New("Public key invalid:" + err.Error())
	}
	pubkey1, err := crypto.DecompressPubkey(publicKeyBytes)
	if err != nil {
		return nil, nil, errors.New("Public key not decompressable:" + err.Error())
	}
	addr := crypto.PubkeyToAddress(*pubkey1)
	if !strings.EqualFold(walletAddr, addr.Hex()) {
		return nil, nil, errors.New("address does not match")
	}

	return &addr, &payload, nil
}

// web3AuthKeyFunc returns the key for the given jwt.Token
func web3AuthKeyFunc(token *jwt.Token) (interface{}, error) {
	// URL for the JWK Set
	jwksURL := "https://api-auth.web3auth.io/jwks"
	// Fetch the JWK Set from the remote URL
	resp, err := http.Get(jwksURL)
	if err != nil {
		slog.Error("Error fetching JWK Set: " + err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body: " + err.Error())
		return nil, err
	}
	// Parse the JWK Set
	var jwkSet JWKSet
	err = json.Unmarshal(body, &jwkSet)
	if err != nil {
		slog.Error("Error parsing JWK Set: " + err.Error())
		return nil, err
	}
	// Get the first key (it's the one we need)
	var jwk jose.JSONWebKey
	err = json.Unmarshal(jwkSet.Keys[0], &jwk)
	if err != nil {
		slog.Error("Error parsing JWK: " + err.Error())
		return nil, err
	}
	return jwk.Key, nil
}
