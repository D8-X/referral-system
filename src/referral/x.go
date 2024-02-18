package referral

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SignUpSocialUser adds a user to the database, if already there: id is updated for address
func (a *App) SignUpSocialUser(twitterId, addr string) error {
	addr = strings.ToLower(addr)
	// first check if the address exists
	var id string
	err := a.Db.QueryRow("SELECT id FROM soc_addr_to_id WHERE addr = $1", addr).Scan(&id)
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
	_, err = a.Db.Exec(query, addr, twitterId)
	if err != nil {
		slog.Error("SignUpSocialUser:" + err.Error())
		return err
	}
	// build ranking
	return a.UserInteractionCountToDb(twitterId)
}

// UserInteractionCountToDb counts the user interactions for the
// given id, and adds (id, id_interacted, count) to the database
func (a *App) UserInteractionCountToDb(id string) error {
	t1 := time.Now()
	res, err := a.Xsdk.Analyzer.CreateUserInteractionGraph(id)
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
		_, err := a.Db.Exec(query, id, ids[k], count[k])
		if err != nil {
			slog.Error("Could not insert addr, id:" + err.Error())
		}
	}
	return nil
}

// GetTop3Interactions finds the 3 EVM-Addresses with top score
// Twitter interactions with the given EVM-Address
func (a *App) GetTop3Interactions(addr string) ([]string, error) {
	addr = strings.ToLower(addr)
	query := `SELECT addr_interacted FROM
		top3_interactions
		where addr=$1`
	rows, err := a.Db.Query(query, addr)
	if err != nil {
		return nil, errors.New("Could not get top n:" + err.Error())
	}
	defer rows.Close()
	addrRes := make([]string, 0, 3)
	for rows.Next() {
		var addr string
		rows.Scan(&addr)
		addrRes = append(addrRes, addr)
	}
	return addrRes, nil
}

// GetGlobalLeaders finds the 'num' global leaders according to the following global score
// Global score for id A = number of other accounts from which which A received any
// number of likes/retweets/comments received
func (a *App) GetGlobalLeaders(num int) ([]string, error) {
	query := `SELECT ati.addr
		FROM glbl_rank AS gr
		JOIN addr_to_id AS ati ON ati.id = gr.id
		ORDER BY gr.interaction_count DESC
		LIMIT $1;`
	rows, err := a.Db.Query(query, num)
	if err != nil {
		return nil, errors.New("Could not get top n:" + err.Error())
	}
	defer rows.Close()
	topAddr := make([]string, 0, 3)
	for rows.Next() {
		var addr string
		rows.Scan(&addr)
		topAddr = append(topAddr, addr)
	}
	return topAddr, nil
}
