package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"go.step.sm/crypto/jose"
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

// VerifyJWT JSON Web Tokens (JWT)
func VerifyJWT(token *APISocialIdToken, appPubKey string) error {
	// URL for the JWK Set
	jwksURL := "https://api-auth.web3auth.io/jwks"
	// Fetch the JWK Set from the remote URL
	resp, err := http.Get(jwksURL)
	if err != nil {
		slog.Error("Error fetching JWK Set: " + err.Error())
		return err
	}
	defer resp.Body.Close()
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body: " + err.Error())
		return err
	}
	// Parse the JWK Set
	var jwkSet JWKSet
	err = json.Unmarshal(body, &jwkSet)
	if err != nil {
		slog.Error("Error parsing JWK Set: " + err.Error())
		return err
	}
	// Get the first key (it's the one we need)
	var jwk jose.JSONWebKey
	err = json.Unmarshal(jwkSet.Keys[0], &jwk)
	if err != nil {
		slog.Error("Error parsing JWK: " + err.Error())
		return err
	}
	//slog.Info(jwk)
	return nil
}
