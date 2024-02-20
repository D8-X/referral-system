package referral

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"referral-system/src/utils"
	"strconv"
	"strings"

	tc "github.com/D8-X/twitter-counter/src/twitter"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const SOCIAL_SYS_TYPE = "SocialSystem"

type SocialSystem struct {
	Config     SocialSysConf
	Xsdk       *XSdk
	Db         *sql.DB
	BrokerAddr string
}

type SocialSysConf struct {
	ChainId                int            `json:"chainId"`
	PaymentMaxLookBackDays int            `json:"paymentMaxLookBackDays"`
	PayCronSchedule        string         `json:"paymentScheduleCron"`
	MultiPayContractAddr   string         `json:"multiPayContractAddr"`
	SocialCutPerc          []float64      `json:"socialCutPerc"`
	AnonTrdrCutPerc        float64        `json:"anonTraderCutPerc"`
	KnownTrdrCutPerc       float64        `json:"knownTraderCutPerc"`
	BrokerPayoutAddr       common.Address `json:"brokerPayoutAddr"`
}

type XSdk struct {
	Client   tc.Client
	Analyzer *tc.Analyzer
}

func (c *SocialSystem) GetType() string {
	return SOCIAL_SYS_TYPE
}

func NewSocialSystem(twitterAuthBearer string) *SocialSystem {
	// connect X
	var sdk XSdk
	sdk.Client = tc.NewAuthBearerClient(twitterAuthBearer)
	sdk.Analyzer = tc.NewProductionAnalyzer(sdk.Client)

	s := SocialSystem{}
	s.Xsdk = &sdk
	return &s
}

func (rs *SocialSystem) SetDb(db *sql.DB) {
	rs.Db = db
}

func (rs *SocialSystem) SetBrokerAddr(addr string) {
	rs.BrokerAddr = addr
}

func (rs *SocialSystem) LoadConfig(fileName string, chainId int) error {
	var configs []SocialSysConf
	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &configs)
	if err != nil {
		return err
	}
	// pick correct conf by chain id
	var conf SocialSysConf = SocialSysConf{}
	for k := 0; k < len(configs); k++ {
		if configs[k].ChainId == chainId {
			conf = configs[k]
			break
		}
	}
	if conf.ChainId != chainId {
		return errors.New("No setting found for chain id " + strconv.Itoa(chainId))
	}
	rs.Config = conf
	return nil
}

func (rs *SocialSystem) GetCronSchedule() string {
	return rs.Config.PayCronSchedule
}

func (rs *SocialSystem) GetMultiPayAddr() string {
	return rs.Config.MultiPayContractAddr
}

func (rs *SocialSystem) GetPaymentLookBackDays() int {
	return rs.Config.PaymentMaxLookBackDays
}
func (rs *SocialSystem) GetBrokerAddr() string {
	return rs.BrokerAddr
}
func (rs *SocialSystem) SettingsToDb() error {
	// not needed for social referral system
	return nil
}

// OpenPay determines how much the given trader gets paid back
// from his trading activity
func (rs *SocialSystem) OpenPay(rows *sql.Rows, app *App, traderAddr string) (utils.APIResponseOpenEarnings, error) {
	var payments []utils.OpenPay
	var res utils.APIResponseOpenEarnings
	res.Code = "social"
	// trader rebate
	fees, err := rs.GetFeeCutsForTrader(traderAddr)
	if err != nil {
		return utils.APIResponseOpenEarnings{}, err
	}
	traderCut := float64(fees[0].Cut2Dec) / 100
	for rows.Next() {
		var el AggrFeesOpenPay
		rows.Scan(&el.PoolId, &el.Code, &el.BrokerFeeCc,
			&el.LastTradeConsidered, &el.TokenName, &el.TokenDecimals)
		fee := new(big.Int)
		fee.SetString(el.BrokerFeeCc, 10)
		amount := utils.ABDKToFloat(fee)
		amount = amount * traderCut
		var op = utils.OpenPay{
			PoolId:    el.PoolId,
			Amount:    amount,
			TokenName: el.TokenName,
		}
		payments = append(payments, op)
	}
	res.OpenPay = payments
	return res, nil
}

func (rs *SocialSystem) PreProcessPayments(rpc *ethclient.Client) error {
	// not needed for social referral system
	return nil
}

func (rs *SocialSystem) ProcessPayments(app *App, rows *sql.Rows, scale map[uint32]float64, batchTs string) {
	for rows.Next() {
		var el AggregatedFeesRow
		var fee string
		rows.Scan(&el.PoolId, &el.TraderAddr, &el.Code, &fee,
			&el.LastTradeConsidered, &el.TokenAddr, &el.TokenDecimals)
		el.BrokerFeeABDKCC = new(big.Int)
		el.BrokerFeeABDKCC.SetString(fee, 10)
		// order: trader, broker, participant1-3
		feeCut, err := rs.GetFeeCutsForTrader(el.TraderAddr)
		if err != nil {
			slog.Error("ProcessPayments trader " + el.TraderAddr + " GetFeeCutsForTrader:" + err.Error())
			continue
		}
		err = rs.processPayment(app, el, feeCut, batchTs, scale[el.PoolId])
		if err != nil {
			slog.Error("aborting payments..." + err.Error())
			break
		}
	}
}

func (rs *SocialSystem) processPayment(app *App, row AggregatedFeesRow, feeCut []FeeCut, batchTs string, scaling float64) error {
	totalDecN := utils.ABDKToDecN(row.BrokerFeeABDKCC, row.TokenDecimals)
	// scale
	if scaling < 1 {
		totalDecN = utils.DecNTimesFloat(totalDecN, scaling, 18)
		msg := fmt.Sprintf("Scaling payment amount by %.2f", scaling)
		slog.Info(msg)
	}
	payees := make([]common.Address, len(feeCut))
	amounts := make([]*big.Int, len(feeCut))
	precision := 6
	distributed := new(big.Int).Set(amounts[0])
	for k, f := range feeCut {
		amounts[k] = utils.DecNTimesFloat(totalDecN, float64(f.Cut2Dec)/100, precision)
		payees[k] = common.HexToAddress(f.Addr)
		distributed.Add(distributed, amounts[k])
	}
	// parent amount goes to broker payout address
	payees[1] = rs.Config.BrokerPayoutAddr
	// rounding down typically leads to totalDecN<distributed
	if distributed.Cmp(totalDecN) < 0 {
		totalDecN = distributed
	}
	// encode message: batchTs.<code>.<poolId>.<encodingversion>
	msg := EncodePaymentInfo(batchTs, row.Code, int(row.PoolId))
	// id = lastTradeConsideredTs in seconds
	id := row.LastTradeConsidered.Unix()
	app.PaymentExecutor.SetClient(app.RpcClient)
	txHash, err := app.PaymentExecutor.TransactPayment(common.HexToAddress(row.TokenAddr), totalDecN, amounts, payees, id, msg, row.Code, app.RpcClient)
	if err != nil {
		slog.Error(err.Error())
		if strings.Contains(err.Error(), "insufficient funds") {
			return err
		} else {
			return nil
		}
	}
	app.DbWriteTx(row.TraderAddr, row.Code, amounts, payees, batchTs, row.PoolId, txHash)
	return nil
}
