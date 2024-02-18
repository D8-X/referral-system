package referral

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"referral-system/src/utils"
	"strconv"

	tc "github.com/D8-X/twitter-counter/src/twitter"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

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
	return nil
}

func (rs *SocialSystem) OpenPay(app *App, traderAddr string) (utils.APIResponseOpenEarnings, error) {
	return utils.APIResponseOpenEarnings{}, nil
}

func (rs *SocialSystem) PreProcessPayments(rpc *ethclient.Client) error {
	return nil
}

func (rs *SocialSystem) ProcessPayments(app *App, rows *sql.Rows, scale map[uint32]float64, batchTs string) {
}

func (rs *SocialSystem) processCodePaymentRow(app *App, row AggregatedFeesRow, chain []DbReferralChainOfChild, batchTs string, scaling float64) error {
	return nil
}
