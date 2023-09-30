package referral

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"referral-system/src/contracts"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func (a *App) CreateRpcClient() error {
	rnd := rand.Intn(len(a.Rpc))
	var rpc *ethclient.Client
	var err error
	for trial := 0; ; trial++ {
		rpc, err = ethclient.Dial(a.Rpc[rnd])
		if err != nil {
			if trial == 5 {
				return err
			}
			slog.Info("Rpc error" + err.Error() + " retrying " + strconv.Itoa(5-trial))
			time.Sleep(time.Duration(2) * time.Second)
		} else {
			break
		}
	}
	a.RpcClient = rpc
	return nil
}

func (a *App) CreateMultipayInstance() error {
	var err error
	if a.RpcClient == nil {
		return errors.New("CreateMultipayInstance requires RpcClient")
	}
	evmAddr := common.HexToAddress(a.Settings.MultiPayContractAddr)
	for trial := 0; ; trial++ {
		a.MultipayCtrct, err = contracts.NewMultiPay(evmAddr, a.RpcClient)
		if err != nil {
			if trial == 5 {
				return err
			}
			slog.Info("Failed to instantiate Multipay " + err.Error() + " retrying " + strconv.Itoa(5-trial))
			time.Sleep(time.Duration(2) * time.Second)
		} else {
			break
		}
	}
	return nil
}

func FilterPayments(ctrct *contracts.MultiPay) error {
	// Create an event iterator for the MultiPayPayment events
	opts := &bind.FilterOpts{
		Start:   0,   // Starting block number
		End:     nil, // Ending block (nil for latest)
		Context: context.Background(),
	}
	multiPayPaymentIterator, err := ctrct.FilterPayment(opts, []common.Address{}, []uint32{}, []common.Address{})
	if err != nil {
		return errors.New("Failed to create event iterator: " + err.Error())
	}
	// Iterate through the events and gather paymentlog:
	/*
		PaymentLog
			BatchTimestamp string
			Code           string
			PoolId         uint32
			TokenAddr      string
			BrokerAddr     string
			PayeeAddr      []string
			AmountDecN     []string
	*/
	for {
		if !multiPayPaymentIterator.Next() {
			break // No more events to process
		}
		var log PaymentLog
		event := multiPayPaymentIterator.Event

		// decode pool Id from message, and timestamp from event id
		s := strings.Split(event.Message, ".")
		if len(s) != 3 {
			slog.Info("- event message in different format")
			continue
		}
		log.BatchTimestamp, err = strconv.Atoi(s[0])
		if err != nil {
			slog.Info("- event message batch timestamp not in expected format")
			continue
		}
		id, err := strconv.Atoi(s[2])
		if err != nil {
			slog.Info("- event message pool id not in expected format")
			continue
		}
		log.PoolId = uint32(id)
		log.Code = s[1]
		log.BrokerAddr = event.From.String()
		log.PayeeAddr = event.Payees
		log.AmountDecN = event.Amounts
		slog.Info("Event Data for code " + log.Code)
	}
	return nil
}
