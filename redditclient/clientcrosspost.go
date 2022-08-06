package redditclient

import (
	"context"
	"fmt"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

func (c *client) CrossPost(ctx context.Context, sourcePost RedditPost, destinationSubReddit string) (*RedditPost, ClientError) {
	c.checkThrottle()
	request := reddit.SubmitCrosspostRequest{
		Subreddit: destinationSubReddit,
		URL:       sourcePost.URL,
		FullName:  sourcePost.FullID,
		Title:     sourcePost.Title,
		Resubmit:  false,
	}

	submittedResult, response, err := c.redditClient.Post.SubmitCrosspost(ctx, request)
	if err != nil {
		return nil, clientError{error: fmt.Errorf("something went wrong with the reddit client, %s", err.Error())}
	}
	if err := c.handleResponse(response); err != nil {
		return nil, err
	}

	if submittedResult != nil {
		return &RedditPost{
			ID:     submittedResult.ID,
			URL:    submittedResult.URL,
			FullID: submittedResult.FullID,
		}, nil
	}

	return nil, nil
}
