package twitter

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/go-resty/resty/v2"
)

// Twitter V2 API endpoint with trailing slash
const TwitterV2API = "https://api.twitter.com/2/"

type Client interface {
	// FetchUserTweets fetches timeline tweets which include tweets, retweets,
	// replies, quote tweets of given userId
	//
	// Limitations are: maximum 3200 in the past.
	//
	// See
	// https://developer.twitter.com/en/docs/twitter-api/tweets/timelines/introduction
	FetchUserTweets(userId string, options ...ApiRequestOption) (*UserTweetsResponse, error)

	FetchUserLikedTweets(userId string, options ...ApiRequestOption)

	FetchTweetDetails(tweetId string, options ...ApiRequestOption) (*TweetDetailsResponse, error)
}

func NewAuthBearerClient(authBearer string) *twitterHTTPClient {
	r := resty.New()
	r.SetHeader("Authorization", "Bearer "+authBearer)

	return &twitterHTTPClient{
		authBearer: authBearer,
		r:          r,
	}
}

var _ Client = (*twitterHTTPClient)(nil)

// twitterHTTPClient is the basic http client for Twitter V2 API authenticating
// with authBearer
type twitterHTTPClient struct {
	authBearer string
	r          *resty.Client
}

func (t *twitterHTTPClient) sendGet(endpoint string, options ...ApiRequestOption) ([]byte, error) {
	req := t.r.R()

	for _, opt := range options {
		opt.Apply(req)
	}

	resp, err := req.Get(endpoint)
	if err != nil {
		return nil, err
	}
	body := resp.Body()

	fullEndpoint := endpoint + "?" + req.QueryParam.Encode()
	slog.Info("sent a GET twitter API request",
		slog.String("endpoint", fullEndpoint),
	)

	if resp.StatusCode() != 200 {
		slog.Error(
			"response failed",
			slog.String("endpoint", fullEndpoint),
			slog.Int("status", resp.StatusCode()),
			slog.String("repsonse", string(body)),
		)
		return nil, fmt.Errorf("response failed: %d", resp.StatusCode())
	}

	return body, nil
}

// FetchUserTweets send a user tweets request and parses it. Collected
// iformation includes tweet text, tweet id, conversation id,
func (t *twitterHTTPClient) FetchUserTweets(userId string, options ...ApiRequestOption) (*UserTweetsResponse, error) {
	endpoint := TwitterV2API + "users/" + userId + "/tweets"
	body, err := t.sendGet(endpoint,
		append(
			options,
			OptApplyMaxResults("5"),
			// Append the conversation_id expansion to get the information if
			// tweet is a reply in conversation. For simple tweets the
			// conversation_id should be the same tweet id
			&OptApplyQueryParam{
				Key:   "tweet.fields",
				Value: "conversation_id,referenced_tweets",
			},
			// Append information about conversation tweet author
			&OptApplyQueryParam{
				Key:   "expansions",
				Value: "in_reply_to_user_id",
			},
		)...,
	)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Response body: %s\n", body)

	ret := &UserTweetsResponse{}
	if err := json.Unmarshal(body, ret); err != nil {
		return nil, fmt.Errorf("parsing user tweets response: %w", err)
	}

	return ret, nil
}

func (t twitterHTTPClient) FetchTweetDetails(tweetId string, options ...ApiRequestOption) (*TweetDetailsResponse, error) {

	endpoint := TwitterV2API + "tweets/" + tweetId
	body, err := t.sendGet(endpoint,
		options...,
	)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Response body: %s\n", body)

	// ret := &UserTweetsResponse{}
	// if err := json.Unmarshal(body, ret); err != nil {
	// 	return nil, fmt.Errorf("parsing user tweets response: %w", err)
	// }

	return nil, nil
}

func (t *twitterHTTPClient) FetchUserLikedTweets(userId string, options ...ApiRequestOption) {

}
