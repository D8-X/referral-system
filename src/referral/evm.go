package referral

import (
	"context"
	"errors"
	"log/slog"
	"math/big"
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

func (a *App) CreateErc20Instance(tokenAddr string) (*contracts.Erc20, error) {
	tknAddr := common.HexToAddress(tokenAddr)
	instance, err := contracts.NewErc20(tknAddr, a.RpcClient)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (a *App) QueryTokenBalance(tknCtrct *contracts.Erc20, tknOwnerAddr string) (*big.Int, error) {
	ownerAddr := common.HexToAddress(tknOwnerAddr)
	bal, err := tknCtrct.BalanceOf(&bind.CallOpts{}, ownerAddr)
	return bal, err
}

func FilterPayments(ctrct *contracts.MultiPay, client *ethclient.Client, startBlock uint64) ([]PaymentLog, error) {
	// Create an event iterator for the MultiPayPayment events
	opts := &bind.FilterOpts{
		Start:   startBlock, // Starting block number
		End:     nil,        // Ending block (nil for latest)
		Context: context.Background(),
	}
	multiPayPaymentIterator, err := ctrct.FilterPayment(opts, []common.Address{}, []uint32{}, []common.Address{})
	if err != nil {
		return []PaymentLog{}, errors.New("Failed to create event iterator: " + err.Error())
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
	blockTimestamps := make(map[uint64]uint64)
	var logs []PaymentLog
	for {
		if !multiPayPaymentIterator.Next() {
			break // No more events to process
		}
		var pay PaymentLog
		event := multiPayPaymentIterator.Event

		// decode pool Id from message, and timestamp from event id
		s := strings.Split(event.Message, ".")
		if len(s) != 3 {
			slog.Info("- event message in different format")
			continue
		}
		pay.BatchTimestamp, err = strconv.Atoi(s[0])
		if err != nil {
			slog.Info("- event message batch timestamp not in expected format")
			continue
		}
		id, err := strconv.Atoi(s[2])
		if err != nil {
			slog.Info("- event message pool id not in expected format")
			continue
		}
		pay.PoolId = uint32(id)
		pay.Code = s[1]
		pay.BrokerAddr = event.From.String()
		//Trader must be the first address
		pay.PayeeAddr = event.Payees
		pay.AmountDecN = event.Amounts
		pay.TxHash = event.Raw.TxHash.String()
		pay.BlockNumber = uint64(multiPayPaymentIterator.Event.Raw.BlockNumber)
		if blockTimestamps[pay.BlockNumber] == 0 {
			// retrieve timestamp
			t := getBlockTimestamp(pay.BlockNumber, client)
			// zero on error
			blockTimestamps[pay.BlockNumber] = t
		}
		pay.BlockTs = blockTimestamps[pay.BlockNumber]
		slog.Info("Event Data for code " + pay.Code)
		logs = append(logs, pay)
	}
	// find unassigend block timestamps
	for _, pay := range logs {
		if pay.BlockTs == 0 {
			ts := getBlockTimestamp(pay.BlockNumber, client)
			if ts != 0 {
				blockTimestamps[pay.BlockNumber] = ts
			} else {
				// we still could not retrieve the timestamp,
				// now we proxy the timestamp with the batch timestamp + 5mins
				// this should avoid that we pay out too much
				blockTimestamps[pay.BlockNumber] = uint64(pay.BatchTimestamp) + 5*60
			}
		}
	}
	return logs, nil
}

func getBlockTimestamp(blockNum uint64, client *ethclient.Client) uint64 {
	var b big.Int
	b.SetUint64(blockNum)
	block, err := client.BlockByNumber(context.Background(), &b)
	if err == nil {
		return block.Time()
	} else {
		return 0
	}
}
