package redditclient

import (
	"context"
	"fmt"
)

func (c *client) ApprovePost(ctx context.Context, sourcePost RedditPost) ClientError {
	c.checkThrottle()

	response, err := c.redditClient.Moderation.Approve(ctx, sourcePost.FullID)
	if err != nil {
		return clientError{error: fmt.Errorf("something went wrong with the reddit client, %s", err.Error())}
	}
	if err := c.handleResponse(response); err != nil {
		return err
	}
	return nil
}
