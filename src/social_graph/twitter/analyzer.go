package twitter

import "log/slog"

// UserInteractions defines the interaction graph for a single user
type UserInteractions struct {
	UserTwitterId string

	// Replies of other users to the tweets of this user. Map of user_id (other
	// users) -> number of replies to current user's tweets
	RepliesToTweets map[string]uint
}

type Analyzer struct {
	Client Client
}

// CreateUserInteractionGraph runs a full interaction check for a given user id.
func (a *Analyzer) CreateUserInteractionGraph(userTwitterId string) {
	// Start with collecting the user's tweets up to specific tweets limit or
	// date
	collectedUserTweetIds := []string{}
	opts := []ApiRequestOption{}
	for {
		tweets, err := a.Client.FetchUserTweets(userTwitterId,
			append(opts, OptApplyMaxResults("5"))...,
		)
		if err != nil {
			slog.Error("failed to fetch user tweets", err)
			// TODO do not kill the process, but handle the possible rate
			// limiting error
		}

		// TODO Process which tweets are replies, which are simple tweets which are
		// retweets
		for _, tweet := range tweets.Data {
			collectedUserTweetIds = append(collectedUserTweetIds, tweet.TweetId)
		}
	}

	// For each collected user tweet (non reply and non retweet)- find the
	// conversations (replies) to these tweets

	// For each collected tweet

	// Collect the likers of user tweets

	// Collect the liked tweets of user
}
