package referral

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"net/http"
	"referral-system/env"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
)

/*
	PayExec implements an interface for either a local broker or a remote
	broker.
*/

type PayExec interface {
	// assign private key and remote broker address
	Init(viper *viper.Viper) error
	GetBrokerAddr() common.Address
	//ExecutePayment(p PaymentExecution) error
}

type basePayExec struct {
	BrokerAddr  common.Address
	ExecPrivKey *ecdsa.PrivateKey //executor private key
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

func (exc *RemotePayExec) Init(viper *viper.Viper) error {
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
	// remote broker address via http
	resp, err := http.Get(addr + "/broker-address")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("unexpected status code: " + strconv.Itoa(resp.StatusCode))
	}
	type BrokerResp struct {
		BrokerAddr string `json:"brokerAddr"`
	}
	var apiResponse BrokerResp
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return err
	}
	exc.BrokerAddr = common.HexToAddress(apiResponse.BrokerAddr)
	return nil
}

func (exc *LocalPayExec) Init(viper *viper.Viper) error {
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

func privateKeyToAddress(k *ecdsa.PrivateKey) (string, error) {
	publicKey := k.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", errors.New("error casting public key to ECDSA")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address, nil
}
