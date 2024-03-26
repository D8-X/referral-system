package referral

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"math/rand"
	"referral-system/src/contracts"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TxStatus represents the status of a transaction
type TxStatus int

const (
	TxNotFound  TxStatus = iota // 0
	TxConfirmed                 // 1
	TxFailed                    // 2
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

// CreateMultipayInstance creates a contract instance of MultiPay
func (a *App) CreateMultipayInstance() error {
	var err error
	if a.RpcClient == nil {
		return errors.New("CreateMultipayInstance requires RpcClient")
	}
	evmAddr := common.HexToAddress(a.Settings.MultiPayContractAddr)
	c, err := contracts.NewMultiPay(evmAddr, a.RpcClient)
	if err != nil {
		return errors.New("Failed to instantiate Proxy contract: " + err.Error())
	}
	a.MultipayCtrct = c
	return nil
}

// CreateAuth creates the necessary object for write-blockchain transactions
func (exc *basePayExec) CreateAuth() (*bind.TransactOpts, error) {
	client := exc.Client
	if client == nil {
		return nil, errors.New("createAuth: rpc client is nil")
	}
	privateKey := exc.ExecPrivKey
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	chainIdB := new(big.Int).SetUint64(uint64(exc.ChainId))
	auth, err := bind.NewKeyedTransactorWithChainID(exc.ExecPrivKey, chainIdB)
	signerFn := func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
		return types.SignTx(tx, types.NewEIP155Signer(chainIdB), privateKey)
	}
	auth.Signer = signerFn
	// set default values
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(300000)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return nil, err
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	nonceB := new(big.Int).SetUint64(nonce)
	auth.Nonce = nonceB
	auth.GasPrice = gasPrice

	return auth, nil
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

// FilterPayments collects historical events and updates the database
func FilterPayments(ctrct *contracts.MultiPay, client *ethclient.Client, startBlock, endBlock uint64) ([]PaymentLog, error) {

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return []PaymentLog{}, errors.New("Failed to get block hgeader: " + err.Error())
	}
	nowBlock := header.Number.Uint64()

	var logs []PaymentLog
	var reportCount int
	var pathLen = float64(nowBlock - startBlock)
	// filter payments in batches of 32_768 (and decreasing) blocks to avoid RPC limit
	deltaBlock := uint64(32_768)
	for trial := 0; trial < 7; trial++ {
		err = nil
		if trial > 0 {
			msg := fmt.Sprintf("Retrying with num blocks=%d (%d/%d)...", deltaBlock, trial, 7)
			slog.Info(msg)
			time.Sleep(5 * time.Second)
		}
		for {
			endBlock := startBlock + deltaBlock
			if reportCount%100 == 0 {
				msg := fmt.Sprintf("Reading payments from onchain: %.0f%%", 100-100*float64(nowBlock-startBlock)/pathLen)
				slog.Info(msg)
			}
			// Create an event iterator for the MultiPayPayment events
			var endBlockPtr *uint64 = &endBlock
			if endBlock >= nowBlock {
				endBlockPtr = nil
			}
			opts := &bind.FilterOpts{
				Start:   startBlock,  // Starting block number
				End:     endBlockPtr, // Ending block (nil for latest)
				Context: context.Background(),
			}
			var multiPayPaymentIterator *contracts.MultiPayPaymentIterator
			multiPayPaymentIterator, err = ctrct.FilterPayment(opts, []common.Address{}, []uint32{}, []common.Address{})
			if err != nil {
				break
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

			processMultiPayEvents(client, multiPayPaymentIterator, &logs)
			if endBlock >= nowBlock {
				break
			}
			startBlock = endBlock + 1
			reportCount += 1
		}
		if err == nil {
			break
		}
		slog.Info("Failed to create event iterator: " + err.Error())
		deltaBlock = deltaBlock / 2
	}
	if err != nil {
		return logs, err
	}
	slog.Info("Reading payments completed.")
	return logs, nil
}

// processMultiPayEvents loops through blockchain events from the multipay contract and collects the data in
// the logs slice
func processMultiPayEvents(client *ethclient.Client, multiPayPaymentIterator *contracts.MultiPayPaymentIterator, logs *[]PaymentLog) {
	blockTimestamps := make(map[uint64]uint64)
	countDefaultCode := 0
	for {
		if !multiPayPaymentIterator.Next() {
			break // No more events to process
		}
		var pay PaymentLog
		event := multiPayPaymentIterator.Event

		// decode pool Id from message, and timestamp from event id
		var s []string = decodePaymentInfo(event.Message)
		if s == nil {
			slog.Info("- event message not in expected format")
			continue
		}
		var err error
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
		if pay.Code == "DEFAULT" {
			countDefaultCode += 1
		} else {
			slog.Info("Event Data for code " + pay.Code)
		}
		*logs = append(*logs, pay)
	}
	if countDefaultCode > 0 {
		slog.Info("Event Data for code DEFAULT (" + strconv.Itoa(countDefaultCode) + " times)")
	}
	// find unassigend block timestamps
	for _, pay := range *logs {
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
}

// get the block timestamp for a block with a given number
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

// QueryTxStatus queries the rpc for the transaction status
func QueryTxStatus(client *ethclient.Client, txHash string) TxStatus {
	receipt, err := getTransactionReceipt(client, txHash)
	if err != nil {
		slog.Error("Could not obtain transaction " + txHash + " error:" + err.Error())
		return TxNotFound
	}
	if receipt.Status == 1 {
		return TxConfirmed
	}
	// Transaction failed
	return TxFailed
}

func getTransactionReceipt(client *ethclient.Client, txHash string) (*types.Receipt, error) {
	// Create the context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h := common.HexToHash(txHash)
	// Call eth_getTransactionReceipt RPC method
	receipt, err := client.TransactionReceipt(ctx, h)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}
