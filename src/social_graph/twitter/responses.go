package twitter

// UserTweetsResponse represents the data from user tweets endpoint. It will include
// the tweets data as well as meta data, pagination, etc.
type UserTweetsResponse struct {
	Data []Tweet `json:"data"`
	Meta Meta    `json:"meta"`
}

type Meta struct {
	NewestID string `json:"newest_id"`
	// NextToken is passed as pagination_token query parameter for the
	// next pagination page
	NextToken     string `json:"next_token"`
	OldestID      string `json:"oldest_id"`
	PreviousToken string `json:"previous_token"`
	ResultCount   int    `json:"result_count"`
}

type Tweet struct {
	AuthorUserId string `json:"author_id"`
	CreatedAt    string `json:"created_at"`
	TweetId      string `json:"id"`
	TweetText    string `json:"text"`

	// Original tweet id if this is a reply. See
	// https://developer.twitter.com/en/docs/twitter-api/tweets/timelines/api-reference/get-users-id-tweets
	ConversationTweetId string `json:"conversation_id"`

	// User id of the conversation tweet (this tweet is a reply to a tweet of
	// this InReplyToUserId user id). Will be empty when tweet is not a reply
	InReplyToUserId string `json:"in_reply_to_user_id"`
}

func (t Tweet) IsReply() bool {
	return t.ConversationTweetId != ""
}

type TweetDetailsResponse struct {
}
