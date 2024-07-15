package contracts

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

func TestFindBlockBefore(t *testing.T) {
	var rpc *ethclient.Client
	url := "https://arbitrum.llamarpc.com" //"https://rpc.public.zkevm-test.net"
	rpc, err := ethclient.Dial(url)
	if err != nil {
		t.Fail()
	}
	ts := uint64(time.Now().Unix() - 15*86400)
	num, ts1, err := FindBlockWithTs(rpc, ts)
	if err != nil {
		t.Fail()
		return
	}
	fmt.Println("Block:", num)
	fmt.Println("Time Searched:", ts)
	fmt.Println("Time Block   :", ts1)
	fmt.Println("Time diff:", int(ts)-int(ts1))

}
