package referral

import (
	"database/sql"
	"fmt"
	"referral-system/env"
	"referral-system/src/utils"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
)

func TestSignUpSocialUser(t *testing.T) {
	twitterId := "822561529623093249"
	addr := "0x73284777D0Ae0f1e99C7D9b4112b2ED9a7DcDFD2"
	vpr, err := utils.LoadEnv([]string{env.TWITTER_AUTH_BEARER, env.CONFIG_PATH, env.DATABASE_DSN_HISTORY}, "../../.env")
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	soc := NewSocialSystem(vpr.GetString(env.TWITTER_AUTH_BEARER))
	db, err := sql.Open("postgres", vpr.GetString(env.DATABASE_DSN_HISTORY))
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	soc.SetDb(db)
	start := time.Now()
	err = soc.SignUpSocialUser(twitterId, addr)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	fmt.Printf("Execution time: %s\n", elapsed)
}
