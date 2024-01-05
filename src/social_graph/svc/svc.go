package svc

import (
	"log/slog"
	"os"
	"referral-system/src/social_graph/env"
	"referral-system/src/social_graph/twitter"

	"github.com/spf13/viper"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	})))
}

func RunTwitterSocialGraphService() {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		slog.Warn(".env file not found")
	}

	viper.AutomaticEnv()

	required := []string{
		env.TWITTER_AUTH_BEARER,
	}

	for _, e := range required {
		if !viper.IsSet(e) {
			slog.Error("missing required environment variable", slog.String("env", e))
			os.Exit(1)
		}
	}

	// Build the Twitter client
	var client twitter.Client = twitter.NewAuthBearerClient(viper.GetString(env.TWITTER_AUTH_BEARER))

	tweetId := "1742122106882552133"
	client.FetchTweetDetails(tweetId) // twitter.OptApplyMaxResults("5"),

	// a := &twitter.Analyzer{Client: client}
	// a.CreateUserInteractionGraph(userId)
	// if err != nil {
	// slog.Error("failed to fetch user tweets", err)
	// return
	// }

	// fmt.Printf("Tweets: %+v\n", tweets)
}
