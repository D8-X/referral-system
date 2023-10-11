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
	"net/http"
	"referral-system/env"
	"strconv"
	"strings"
	"time"

	d8x_futures "github.com/D8-X/d8x-futures-go-sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

/*
	PayExec implements an interface for either a local broker or a remote
	broker.
*/

type PayExec interface {
	// assign private key and remote broker address
	Init(viper *viper.Viper, multiPayAddr string) error
	GetBrokerAddr() common.Address
	TransactPayment(tokenAddr common.Address, total *big.Int, amounts []*big.Int, payees []common.Address, id int64, msg string) error
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
	MultipayCtrctAddr string
	ChainId           int64
}

type RemotePayExec struct {
	basePayExec
	RemoteBrkrUrl string
}

type LocalPayExec struct {
	basePayExec
}

func (exc *basePayExec) GetBrokerAddr() common.Address {
	return exc.BrokerAddr
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

func (exc *LocalPayExec) Init(viper *viper.Viper, multiPayAddr string) error {
	exc.ChainId = viper.GetInt64(env.CHAIN_ID)
	exc.MultipayCtrctAddr = multiPayAddr
	pk, err := crypto.HexToECDSA(viper.GetString(env.BROKER_KEY))
	if err != nil {
		return errors.New("BROKER_KEY not correctly defined: " + err.Error())
	}
	exc.ExecPrivKey = pk
	addrStr, err := privateKeyToAddress(pk)
	if err != nil {
		return err
	}
	exc.BrokerAddr = common.HexToAddress(addrStr)
	return nil
}

func logPaymentIntent(tokenAddr common.Address, amounts []*big.Int, payees []common.Address, id int64, msg string) {
	slog.Info("transact " + msg + ", batch " + strconv.FormatInt(id, 10))
	for k := 0; k < len(payees); k++ {
		slog.Info(" -- Payee " + payees[k].String())
		slog.Info("    Amount (decN)" + amounts[k].String())
	}
}

func (exc *LocalPayExec) TransactPayment(tokenAddr common.Address, total *big.Int, amounts []*big.Int, payees []common.Address, id int64, msg string) error {
	logPaymentIntent(tokenAddr, amounts, payees, id, msg)
	return nil
}

func (exc *RemotePayExec) TransactPayment(tokenAddr common.Address, total *big.Int, amounts []*big.Int, payees []common.Address, id int64, msg string) error {
	logPaymentIntent(tokenAddr, amounts, payees, id, msg)
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

	sig, err := exc.remoteGetSignature(payment)
	if err != nil {
		return err
	}
	slog.Info("Got signature for payment execution:" + sig)
	return nil
}

func (exc *RemotePayExec) remoteGetSignature(paydata d8x_futures.PaySummary) (string, error) {
	var execWallet d8x_futures.Wallet
	pk := fmt.Sprintf("%x", exc.ExecPrivKey.D)
	err := execWallet.NewWallet(pk, paydata.ChainId, nil)
	if err != nil {
		return "", errors.New("error creating wallet:" + err.Error())
	}
	_, sg, err := d8x_futures.CreatePaymentBrokerSignature(paydata, execWallet)
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

func privateKeyToAddress(k *ecdsa.PrivateKey) (string, error) {
	publicKey := k.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", errors.New("error casting public key to ECDSA")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address, nil
}
