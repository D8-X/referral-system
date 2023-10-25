package contracts

import (
	"context"
	"errors"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/exp/slog"
)

func FindBlockWithTs(client *ethclient.Client, ts uint64) (uint64, uint64, error) {
	blockB, err := client.BlockByNumber(context.Background(), nil)
	if err != nil {
		return 0, 0, err
	}
	tsB := blockB.Time()
	numB := blockB.Number().Uint64()
	if ts >= tsB {
		return tsB, numB, nil
	}
	// guess and search so that ts is between tsA and tsB
	var numA, tsA, timeEst, numCalls uint64
	timeEst = 10
	numCalls = 0
	for {
		tDiff := tsB - ts
		timeBack := max(tDiff/timeEst, 1)
		if timeBack >= numB {
			return 0, 0, errors.New("Genesis Block reached timestamp search failed")
		}
		numA = numB - timeBack
		numABig := big.NewInt(int64(numA))
		blockA, err := client.BlockByNumber(context.Background(), numABig)
		numCalls++
		if err != nil {
			return 0, 0, errors.New("RPC issue in FindBlockFromTs:" + err.Error())
		}
		tsA = blockA.Time()
		numA = blockA.Number().Uint64()
		if tsA < ts {
			break
		}
		timeEst = max((tsB-tsA)/(numB-numA), 1)
		tsB = tsA
		numB = numA
	}
	blockNo, tsFound, numCalls2, err := binSearch(client, numA, tsA, numB, tsB, ts)
	numCalls = numCalls + numCalls2
	slog.Info("Num rpc calls FindBlockWithTs=" + strconv.Itoa(int(numCalls)))
	return blockNo, tsFound, err
}

func binSearch(client *ethclient.Client, numA uint64, tsA uint64, numB uint64, tsB uint64, ts uint64) (uint64, uint64, uint64, error) {

	var tsP, numP, numCalls uint64
	numCalls = 0
	for {
		numP = (numA + numB) / 2
		numPBig := big.NewInt(int64(numP))
		blockP, err := client.BlockByNumber(context.Background(), numPBig)
		numCalls++
		if err != nil {
			return 0, 0, numCalls, errors.New("RPC issue in FindBlockFromTs(search):" + err.Error())
		}
		tsP = blockP.Time()
		if tsP < ts {
			tsA = tsP
			numA = numP
		} else {
			tsB = tsP
			numB = numP
		}
		if numB <= numA+2 {
			break
		}

	}
	return numP, tsP, numCalls, nil
}

func getCurrentBlockTs(rpcClient *ethclient.Client) (int64, error) {
	header, err := rpcClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, errors.New("Failed to retrieve the latest block header: " + err.Error())
	}
	return int64(header.Time), nil
}
