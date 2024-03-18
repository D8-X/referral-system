package referral

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/big"
	"math/rand"
	"net/http"
	"referral-system/env"
	"referral-system/src/contracts"
	"strconv"
	"strings"
	"time"

	"github.com/D8-X/d8x-futures-go-sdk/pkg/d8x_futures"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

/*
	PayExec implements an interface for a broker.
*/

type PayExec interface {
	// assign private key and remote broker address
	Init(viper *viper.Viper, multiPayAddr string) error
	GetBrokerAddr() common.Address
	TransactPayment(tokenAddr common.Address, total *big.Int, amounts []*big.Int, payees []common.Address, id int64, msg, code string, rpc *ethclient.Client) (string, error)
	GetExecutorAddrHex() string
	SetClient(client *ethclient.Client)
	NewTokenBucket(tokens int, refillRate float64)
}

type PaySummaryAux struct {
	Payer         string `json:"payer"`
	Executor      string `json:"executor"`
	Token         string `json:"token"`
	Timestamp     uint32 `json:"timestamp"`
	Id            uint32 `json:"id"`
	TotalAmount   string `json:"totalAmount"`
	ChainId       int64  `json:"chainId"`
	MultiPayCtrct string `json:"multiPayCtrct"`
}

type BrokerPaySignatureReqAux struct {
	Payment           PaySummaryAux `json:"payment"`
	ExecutorSignature string        `json:"signature"`
}

type basePayExec struct {
	BrokerAddr        common.Address
	ExecPrivKey       *ecdsa.PrivateKey //executor private key
	MultipayCtrct     *contracts.MultiPay
	MultipayCtrctAddr string
	ChainId           int64
	Client            *ethclient.Client
	RPCTokenBucket    TokenBucket
}

type RemotePayExec struct {
	basePayExec
	RemoteBrkrUrl string
}

func (exc *basePayExec) GetBrokerAddr() common.Address {
	return exc.BrokerAddr
}

func (exc *basePayExec) SetClient(client *ethclient.Client) {
	exc.Client = client
}

func (exc *basePayExec) NewTokenBucket(capacity int, refillRate float64) {
	exc.RPCTokenBucket = TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (exc *RemotePayExec) Init(viper *viper.Viper, multiPayAddr string) error {
	exc.ChainId = viper.GetInt64(env.CHAIN_ID)
	exc.MultipayCtrctAddr = multiPayAddr
	addr := viper.GetString(env.REMOTE_BROKER_HTTP)
	if addr == "" {
		return errors.New("No remote broker URL defined")
	}
	addr, _ = strings.CutSuffix(addr, "/")
	exc.RemoteBrkrUrl = addr
	pk, err := crypto.HexToECDSA(viper.GetString(env.BROKER_KEY))
	if err != nil {
		return err
	}
	exc.ExecPrivKey = pk
	// remote broker address
	client := resty.New()
	// Make the GET request
	resp, err := client.R().
		EnableTrace().
		Get(addr + "/broker-address")

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if resp.StatusCode() != 200 {
		return errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode()))
	}
	type BrokerResp struct {
		BrokerAddr string `json:"brokerAddr"`
	}
	var apiResponse BrokerResp
	respBody := resp.String()
	err = json.Unmarshal([]byte(respBody), &apiResponse)
	if err != nil {
		return err
	}
	exc.BrokerAddr = common.HexToAddress(apiResponse.BrokerAddr)
	return nil
}

func logPaymentIntent(tokenAddr common.Address, amounts []*big.Int, payees []common.Address, id int64, msg, code string) {
	slog.Info("transact " + code + ";" + msg + ", last trade " + strconv.FormatInt(id, 10))
	for k := 0; k < len(payees); k++ {
		slog.Info(" -- Payee " + payees[k].String())
		slog.Info("    Amount (decN)" + amounts[k].String())
		slog.Info("    Token addr" + tokenAddr.Hex())
	}
}

func (exc *RemotePayExec) GetExecutorAddrHex() string {
	execAddr, _ := privateKeyToAddress(exc.ExecPrivKey)
	return execAddr
}

func (exc *RemotePayExec) TransactPayment(tokenAddr common.Address, total *big.Int, amounts []*big.Int, payees []common.Address, id int64, msg, code string, rpc *ethclient.Client) (string, error) {
	logPaymentIntent(tokenAddr, amounts, payees, id, msg, code)
	if len(amounts) != len(payees) {
		return "", errors.New("#amounts must be equal to #payees")
	}
	// get signature
	ts := time.Now().Unix()
	execAddr, _ := privateKeyToAddress(exc.ExecPrivKey)
	payment := d8x_futures.PaySummary{
		Payer:         exc.BrokerAddr,
		Executor:      common.HexToAddress(execAddr),
		Token:         tokenAddr,
		Timestamp:     uint32(ts),
		Id:            uint32(id),
		TotalAmount:   total,
		ChainId:       exc.ChainId,
		MultiPayCtrct: common.HexToAddress(exc.MultipayCtrctAddr),
	}

	sig, err := exc.remoteGetSignature(payment, rpc)
	if err != nil {
		return "", err
	}
	slog.Info("Got signature for payment execution:" + sig)
	// check signature address
	sigTrim := strings.TrimPrefix(sig, "0x")
	signer, err := d8x_futures.RecoverPaymentSignatureAddr(common.Hex2Bytes(sigTrim[:]), &payment)
	if err != nil {
		return "", errors.New("Could not recover signature " + err.Error())
	}
	if payment.Payer.String() != signer.String() {
		slog.Error("Payment payer " + payment.Payer.String() + "not equal to signer " + signer.String())
		return "", errors.New("payer address must be signer address")
	}
	slog.Info("Signature ok")
	txHash, err := exc.Pay(payment, sig, amounts, payees, msg)
	if err != nil {
		slog.Error("Unable to pay:" + err.Error())
		return "", err
	}
	slog.Info("Payment submitted tx hash = " + txHash)
	return txHash, nil
}

// remoteGetSignature signs the payment data locally with the executor address and
// retrieves the remote-broker signature via REST API. Returns the signature
// as hex-string
func (exc *RemotePayExec) remoteGetSignature(paydata d8x_futures.PaySummary, rpc *ethclient.Client) (string, error) {
	pk := fmt.Sprintf("%x", exc.ExecPrivKey.D)

	execWallet, err := d8x_futures.NewWallet(pk, paydata.ChainId, rpc)
	if err != nil {
		return "", errors.New("error creating wallet:" + err.Error())
	}
	_, sg, err := d8x_futures.RawCreatePaymentBrokerSignature(&paydata, execWallet)
	slog.Info("Querying broker signature...")
	slog.Info("Token    = " + paydata.Token.String())
	slog.Info("Broker   = " + paydata.Payer.String())
	slog.Info("Multipay = " + paydata.MultiPayCtrct.String())
	slog.Info("Amount   = " + paydata.TotalAmount.String())

	var p = BrokerPaySignatureReqAux{
		Payment: PaySummaryAux{
			Payer:         paydata.Payer.String(),
			Executor:      paydata.Executor.String(),
			Token:         paydata.Token.String(),
			Timestamp:     paydata.Timestamp,
			Id:            paydata.Id,
			TotalAmount:   paydata.TotalAmount.String(),
			ChainId:       paydata.ChainId,
			MultiPayCtrct: paydata.MultiPayCtrct.String(),
		},
		ExecutorSignature: sg,
	}
	payload, err := json.Marshal(p)
	if err != nil {
		slog.Error("Error marshaling JSON:" + err.Error())
		return "", err
	}
	url := exc.RemoteBrkrUrl + "/sign-payment"
	// Send a POST request with the JSON payload
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		slog.Error("Error sending POST request:" + err.Error())
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading response body:" + err.Error())
		return "", err
	}
	slog.Info("Remote broker response obtained.")
	type Response struct {
		BrokerSignature string `json:"brokerSignature"`
		Error           string `json:"error"`
	}
	var responseData Response
	if err := json.Unmarshal(body, &responseData); err != nil {
		slog.Error("Error unmarshaling JSON:" + err.Error())
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("Error: Non-200 status code received, " + responseData.Error)
		return "", errors.New("Error: Non-200 status code received")
	}

	if responseData.Error != "" {
		slog.Error("Error response:" + responseData.Error)
		return "", errors.New(responseData.Error)
	}
	return responseData.BrokerSignature, nil
}

func (exc *RemotePayExec) Pay(payment d8x_futures.PaySummary, sig string, amounts []*big.Int, payees []common.Address, msg string) (string, error) {
	// check pre-condition
	t := new(big.Int).Set(amounts[0])
	for i := 1; i < len(amounts); i++ {
		t.Add(t, amounts[i])
	}
	if t.String() != payment.TotalAmount.String() {
		return "", errors.New("total amount must be sum of amounts")
	}
	exc.waitForToken("auth")
	auth, err := exc.CreateAuth()
	if err != nil {
		slog.Error("Pay: Could not create auth: " + err.Error())
		return "", err
	}
	if auth.From.String() != payment.Executor.String() {
		return "", errors.New("Payment executor must transaction sender")
	}
	mpay, err := contracts.NewMultiPay(common.HexToAddress(exc.MultipayCtrctAddr), exc.Client)
	if err != nil {
		return "", errors.New("Failed to instantiate Proxy contract: " + err.Error())
	}

	var s = contracts.MultiPayPaySummary{
		Payer:       payment.Payer,
		Executor:    payment.Executor,
		Token:       payment.Token,
		Timestamp:   payment.Timestamp,
		Id:          payment.Id,
		TotalAmount: payment.TotalAmount,
	}
	auth.GasLimit = 5000000
	sigTrim := strings.TrimPrefix(sig, "0x")
	exc.waitForToken("DelegatedPay")
	tx, err := mpay.DelegatedPay(auth, s, common.Hex2Bytes(sigTrim), amounts, payees, msg)

	if err != nil {
		return "", err
	}
	return tx.Hash().Hex(), nil
}

func (exc *RemotePayExec) waitForToken(topic string) {
	for {
		if exc.RPCTokenBucket.Take() {
			slog.Info(topic + ": rpc token obtained")
			return
		}
		slog.Info(topic + ": too many RPC requests, slowing down ")
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	}
}

func privateKeyToAddress(k *ecdsa.PrivateKey) (string, error) {
	publicKey := k.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", errors.New("error casting public key to ECDSA")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address, nil
}
