package referral

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"referral-system/src/utils"
	"strings"
	"time"
)

type ScoreLeaderBoard struct {
	Rank  int `json:"rank"`
	Score int `json:"score"`
	UserId
}

type UserId struct {
	Addr string `json:"addr"`
	Id   string `json:"id"`
}

type FeeCut struct {
	Addr    string
	Cut2Dec int
}

// SignUpSocialUser adds a user to the database, if already there: id is updated for address
// Verification is done before this function
func (rs *SocialSystem) SignUpSocialUser(twitterId, addr string) error {
	addr = strings.ToLower(addr)
	// first check if the address exists
	var id string
	err := rs.Db.QueryRow("SELECT id FROM soc_addr_to_id WHERE addr = $1", addr).Scan(&id)
	if err == nil {
		// address exists
		slog.Info("Address " + addr + " logged in (already registered with id " + id + ")")
		return nil
	} else if err != sql.ErrNoRows {
		return errors.New("SignUpSocialUser (select):" + err.Error())
	}
	// address does not exist yet
	slog.Info("Address " + addr + " logged in (new address with Twitter id " + id + ")")
	query := `INSERT INTO soc_addr_to_id (addr, id) VALUES ($1, $2)`
	_, err = rs.Db.Exec(query, addr, twitterId)
	if err != nil {
		slog.Error("SignUpSocialUser:" + err.Error())
		return err
	}
	// build ranking
	return rs.UserInteractionCountToDb(twitterId)
}

// UserInteractionCountToDb counts the user interactions for the
// given id, and adds (id, id_interacted, count) to the database
func (rs *SocialSystem) UserInteractionCountToDb(id string) error {
	t1 := time.Now()
	res, err := rs.Xsdk.Analyzer.CreateUserInteractionGraph(id)
	if err != nil {
		return err
	}
	t2 := time.Now()
	msg := fmt.Sprintf("User interaction created in %v\n", t2.Sub(t1).String())
	slog.Info(msg)
	ids, count := res.Ranked()
	query := `INSERT INTO counter
			  (id, id_interacted, count) VALUES ($1, $2, $3)
			  ON CONFLICT (id, id_interacted)
			  DO UPDATE SET count=$3`
	for k := 0; k < len(ids); k++ {
		_, err := rs.Db.Exec(query, id, ids[k], count[k])
		if err != nil {
			slog.Error("Could not insert addr, id:" + err.Error())
		}
	}
	return nil
}

// GetMyReferrals searches the user with the given address or id
func (rs *SocialSystem) GetUser(addrOrId string) (UserId, error) {
	var query string
	if strings.HasPrefix(addrOrId, "0x") {
		addrOrId = strings.ToLower(addrOrId)
		query = `SELECT addr, id FROM 
		addr_to_id ati  
		WHERE ati.addr =$1`
	} else {
		query = `SELECT addr, id FROM 
		addr_to_id ati  
		WHERE ati.id =$1`
	}
	var user UserId
	err := rs.Db.QueryRow(query, addrOrId).Scan(&user.Addr, &user.Id)
	if err != sql.ErrNoRows && err != nil {
		slog.Info("Failed to query user:" + err.Error())
		return UserId{}, errors.New("Failed")
	}
	return user, nil
}

// GetMyReferrals searches all users for which the given address or id
// is in the top 3 and also searches the fee that these addresses pay to the given address/id
func (rs *SocialSystem) GetMyReferrals(addrOrId string) ([]utils.APIResponseMyReferrals, error) {
	var query string
	if strings.HasPrefix(addrOrId, "0x") {
		addrOrId = strings.ToLower(addrOrId)
		query = `SELECT ti.id, ti.addr, ti.rnk FROM 
		top3_interactions ti 
		WHERE ti.addr_interacted =$1`
	} else {
		query = `SELECT ti.id, ti.addr, ti.rnk FROM 
		top3_interactions ti 
		WHERE ti.id_interacted =$1`
	}
	rows, err := rs.Db.Query(query, addrOrId)
	if err != nil {
		return nil, errors.New("could not get top 3:" + err.Error())
	}
	defer rows.Close()
	referrals := make([]utils.APIResponseMyReferrals, 0)
	for rows.Next() {
		var id, addr string
		var rank int
		rows.Scan(&id, &addr, &rank)
		// determine fee
		var ref utils.APIResponseMyReferrals
		if rank == 0 {
			continue
		}
		ref.PassOnPerc = rs.Config.SocialCutPerc[rank-1]
		ref.Referral = id + "|" + addr
		referrals = append(referrals, ref)
	}

	return referrals, nil
}

// GetTop3Interactions finds the 3 EVM-Addresses with top score
// Twitter interactions with the given EVM-Address or Twitter Id
func (rs *SocialSystem) GetTop3Interactions(addrOrId string) ([]ScoreLeaderBoard, error) {
	var query string
	if strings.HasPrefix(addrOrId, "0x") {
		addrOrId = strings.ToLower(addrOrId)
		query = `SELECT id_interacted, addr_interacted, count, rank
		top3_interactions
		WHERE addr=$1
		order by count desc`
	} else {
		query = `SELECT id_interacted, addr_interacted, count, rank
		top3_interactions
		WHERE id=$1
		order by count desc`
	}

	rows, err := rs.Db.Query(query, addrOrId)
	if err != nil {
		return nil, errors.New("Could not get top 3:" + err.Error())
	}
	defer rows.Close()
	top3 := make([]ScoreLeaderBoard, 0, 3)
	for rows.Next() {
		var entry ScoreLeaderBoard
		rows.Scan(&entry.Addr, &entry.Id, &entry.Score, &entry.Rank)
		top3 = append(top3, entry)
	}
	return top3, nil
}

// GetGlobalLeaders finds the 'num' global leaders according to the following global score
// Global score for id A = number of other accounts from which which A received any
// number of likes/retweets/comments received
func (rs *SocialSystem) GetGlobalLeaders(num int) ([]ScoreLeaderBoard, error) {
	query := `SELECT ati.addr, ati.id, gr.interaction_count
		FROM glbl_rank AS gr
		JOIN addr_to_id AS ati ON ati.id = gr.id
		ORDER BY gr.interaction_count DESC
		LIMIT $1;`
	rows, err := rs.Db.Query(query, num)
	if err != nil {
		return nil, errors.New("Could not get top n:" + err.Error())
	}
	defer rows.Close()
	board := make([]ScoreLeaderBoard, 0, num)
	rank := 1
	for rows.Next() {
		var gb ScoreLeaderBoard
		rows.Scan(&gb.Addr, &gb.Id, &gb.Score)
		gb.Rank = rank
		board = append(board, gb)
		rank++
	}
	return board, nil
}

// GetMyRebate determines the rebate for the given trader, value in percent,
// e.g. 2.5% is 2.5
func (rs *SocialSystem) GetMyRebate(addrOrId string) (float64, error) {
	topI, err := rs.GetTop3Interactions(addrOrId)
	if err != nil {
		return 0, err
	}
	if len(topI) == 0 {
		//trader has no connections
		return rs.Config.AnonTrdrCutPerc, nil
	}
	return rs.Config.KnownTrdrCutPerc, nil
}

// GetFeeCutsForTrader determines what percentage to send from the given trader to which participants.
// order: trader, broker, participant1-3
// That is, calculate the cut of the fees paid by the trader
// that is to be re-distributed according to the social graph and fee
// rebate settings.
// The numbers are in 100 percent. E.g., 12.5%=1250
func (rs *SocialSystem) GetFeeCutsForTrader(addrTrader string) ([]FeeCut, error) {
	cuts := make([]FeeCut, 0, 3)
	topI, err := rs.GetTop3Interactions(addrTrader)
	if err != nil {
		return nil, err
	}
	// two decimals: we multiply the percentage with 100 and use ints
	// pay top 3 (or less) influencers
	var cut, cutDistr int
	if len(topI) == 0 {
		//trader has no connections
		cut = int(rs.Config.AnonTrdrCutPerc * 100)
	} else {
		cut = int(rs.Config.KnownTrdrCutPerc * 100)
	}
	cutDistr += cut
	// 1. Trader
	cuts = append(cuts, FeeCut{Addr: addrTrader, Cut2Dec: cut})
	// 2. Broker (cut not calculated yet)
	cuts = append(cuts, FeeCut{})
	// 3. top 3
	for k, user := range topI {
		cut = int(rs.Config.SocialCutPerc[k] * 100)
		cuts = append(cuts, FeeCut{Addr: user.Addr, Cut2Dec: cut})
		cutDistr += cut
	}
	// pay rest to top influencers
	rem := 4 - len(cuts)
	if rem > 0 {
		v2, err := rs.GetGlobalLeaders(3)
		if err != nil {
			return nil, err
		}
		for k := 0; k < rem; k++ {
			j := len(topI) + k
			cut = int(rs.Config.SocialCutPerc[j] * 100)
			cuts = append(cuts, FeeCut{Addr: v2[k].Addr, Cut2Dec: cut})
			cutDistr += cut
		}
	}
	if cutDistr > 10000 {
		return nil, errors.New("cut exceeds 100%, revisit config")
	}
	// broker gets residual
	cuts[1] = FeeCut{Addr: rs.BrokerAddr, Cut2Dec: 10000 - cutDistr}
	return cuts, nil
}
