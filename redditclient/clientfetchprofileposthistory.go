package redditclient

import (
	"context"
	"fmt"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

func (c *client) FetchProfilePostHistory(ctx context.Context, authorName string) ([]RedditPost, ClientError) {
	c.checkThrottle()
	opts := reddit.ListUserOverviewOptions{
		Sort: "new",
		ListOptions: reddit.ListOptions{
			Limit: 100,
		},
	}

	posts, response, err := c.redditClient.User.PostsOf(ctx, authorName, &opts)
	if err != nil {
		return nil, clientError{error: fmt.Errorf("something went wrong with the reddit client, %s", err.Error())}
	}
	if err := c.handleResponse(response); err != nil {
		return nil, err
	}

	returnedPosts := make([]RedditPost, 0, len(posts))
	for _, post := range posts {
		returnedPosts = append(returnedPosts, redditPostFromPostAPI(*post))
	}
	return returnedPosts, nil
}
