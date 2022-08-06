package redditclient

import (
	"context"
	"fmt"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

func (c *client) FetchNewPosts(ctx context.Context, subredditName string, iterator *FetchNewPostsIterator) ([]RedditPost, ClientError) {
	c.checkThrottle()
	listOptions := reddit.ListOptions{
		Limit: 100,
	}
	if iterator != nil {
		//Bit counter intuitive, but we're listing new posts where 'before' means newer.
		listOptions.Before = iterator.LastID
	}

	postSlice, response, err := c.redditClient.Subreddit.NewPosts(ctx, subredditName, &listOptions)
	if err != nil {
		return nil, clientError{error: fmt.Errorf("something went wrong with the reddit client, %s", err.Error())}
	}
	if err := c.handleResponse(response); err != nil {
		return nil, err
	}

	returnedPosts := make([]RedditPost, 0, 100)
	var lastDate *time.Time
	var firstDate *time.Time
	var firstID string
	var lastID string
	for _, post := range postSlice {
		returnedPosts = append(returnedPosts,
			redditPostFromPostAPI(*post),
		)
		if firstDate == nil || post.Created.Before(*firstDate) {
			firstID = post.FullID
			firstDate = new(time.Time)
			*firstDate = post.Created.Time
		}

		if lastDate == nil || post.Created.After(*lastDate) {
			lastID = post.FullID
			lastDate = new(time.Time)
			*lastDate = post.Created.Time
		}
	}
	if (lastDate != nil && firstDate != nil) && iterator != nil {
		iterator.LastID = lastID
		iterator.FirstID = firstID
	}
	return returnedPosts, nil
}
