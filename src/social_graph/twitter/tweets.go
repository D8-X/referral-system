package twitter

import (
	"fmt"
	"io"
	"net/http"
	"referral-system/src/social_graph/env"

	"github.com/spf13/viper"
)

type TweetsFetcher interface {
	FetchTweets() ([]any, error)

	FetchUserTweets(twitterUserId int) ([]any, error)
}

func NewTweetsFetcher(c *http.Client) TweetsFetcher {
	return &httpTweetsFetcher{
		c: c,
	}
}

var _ TweetsFetcher = (*httpTweetsFetcher)(nil)

type httpTweetsFetcher struct {
	c *http.Client
}

func (h *httpTweetsFetcher) FetchUserTweets(twitterUserId int) ([]any, error) {

	return nil, nil
}
func (h *httpTweetsFetcher) FetchTweets() ([]any, error) {
	// specific tweet with converstaations and author id
	// tweetId := "1739753701894410247"
	// endpoint := "https://api.twitter.com/2/tweets?ids=" + tweetId + "&expansions=author_id&tweet.fields=conversation_id"

	// Specific user's tweets (the one who commented on d8x's tweet)
	userId := "1717292055083220992"
	// endpoint := "https://api.twitter.com/2/users/" + userId + "/tweets"

	// Tweet liking users
	// endpoint := "https://api.twitter.com/2/tweets/" + tweetId + "/liking_users"

	// User likes
	endpoint := "https://api.twitter.com/2/users/" + userId + "/liked_tweets?expansions=author_id"

	// conversationId := "1739753701894410247"
	// endpoint := "https://api.twitter.com/2/tweets/search/recent?query=conversation_id:" + conversationId + "&tweet.fields=in_reply_to_user_id,author_id,created_at,conversation_id"

	// Get user id from username
	// endpont := https://api.twitter.com/2/users/by?usernames=twitterdev

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+viper.GetString(env.TWITTER_AUTH_BEARER))

	resp, err := h.c.Do(req)
	if err != nil {
		return nil, err
	}

	respContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("respContent: %s\n", respContent)

	return nil, nil
}
