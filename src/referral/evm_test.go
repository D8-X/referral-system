package referral

import (
	"log"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
)

func TestGetTransactionReceipt(t *testing.T) {
	nodeURL := "https://rpc.public.zkevm-test.net"

	txHashSucc := "0xa4aa3bdc545e9d5a1ef8b00798dbc51d02401a257cbb4b15c0e637db76a65eb4"
	txHashFail := "0xbeb7bf7a32305172722d1469bd8fc050bbee2db2a012690e4280285584560fcf"
	txHashInvented := "0xCAC7bf7a32305172722d1469bd8fc050bbee2db2a012690e4280285584560fcf"
	// Create a new RPC client
	client, err := ethclient.Dial(nodeURL)
	if err != nil {
		log.Fatal(err)
		return
	}
	r1 := QueryTxStatus(client, txHashSucc)
	if r1 != TxConfirmed {
		t.Errorf("tx 'success' failed")
	}
	r2 := QueryTxStatus(client, txHashFail)
	if r2 != TxFailed {
		t.Errorf("tx 'fail' failed")
	}
	r3 := QueryTxStatus(client, txHashInvented)
	if r3 != TxNotFound {
		t.Errorf("tx 'not found' failed")
	}

}
