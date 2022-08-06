package redditclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

type (
	Client interface {
		Login(ctx context.Context) ClientError
		FetchNewPosts(ctx context.Context, subredditName string, iterator *FetchNewPostsIterator) ([]RedditPost, ClientError)
		FetchProfilePostHistory(ctx context.Context, authorName string) ([]RedditPost, ClientError)
		FetchProfileCommentHistory(ctx context.Context, authorName string) ([]RedditPost, ClientError)
		CrossPost(ctx context.Context, sourcePost RedditPost, destinationSubReddit string) (*RedditPost, ClientError)
		ApprovePost(ctx context.Context, sourcePost RedditPost) ClientError
	}
	ClientError interface {
		error
		CanRetry() bool
	}

	clientError struct {
		error
		canRetry bool
	}

	client struct {
		clientLock     sync.Mutex
		redditClient   *reddit.Client
		lastRate       reddit.Rate
		credentialFile string
		credentials    struct {
			AppID     string `json:"appid"`
			AppSecret string `json:"appsecret"`
			Username  string `json:"username"`
			Password  string `json:"password"`
		}
	}

	RedditPost struct {
		ID         string
		AuthorID   string
		AuthorName string
		FullID     string
		Title      string
		SubReddit  string
		URL        string
	}
)

func (e clientError) CanRetry() bool {
	return e.canRetry
}

func NewRedditClient(credentialFilename string) Client {
	return &client{
		credentialFile: credentialFilename,
		lastRate: reddit.Rate{
			Remaining: 999,
		},
	}
}

func (c *client) handleResponse(response *reddit.Response) ClientError {
	c.clientLock.Lock()
	defer c.clientLock.Unlock()

	if response != nil {
		if response.Rate.Reset.After(c.lastRate.Reset) {
			c.lastRate = response.Rate
		}

		if response != nil && response.StatusCode == 429 {
			defer response.Body.Close()
			bodyString, _ := ioutil.ReadAll(response.Body)
			return &clientError{
				error:    fmt.Errorf("despite trying to rate limit ourselves, reddit is throttling us: %s", bodyString),
				canRetry: true,
			}
		}
	}

	return nil
}
func (c *client) checkThrottle() {
	c.clientLock.Lock()
	defer c.clientLock.Unlock()

	now := time.Now()
	if now.Before(c.lastRate.Reset) {
		if c.lastRate.Remaining == 0 {
			//block until we're allowed to do calls again
			time.Sleep(c.lastRate.Reset.Sub(now))
			time.Sleep(time.Second * 1)
		}
	}
}

func (c *client) Login(ctx context.Context) ClientError {
	passwordBytes, err := ioutil.ReadFile(c.credentialFile)
	if err != nil {
		return clientError{error: fmt.Errorf("could not access password file '%s', %w", c.credentialFile, err)}
	}

	if err := json.Unmarshal(passwordBytes, &c.credentials); err != nil {
		return clientError{error: fmt.Errorf("could not parse password file '%s', %w", c.credentialFile, err)}
	}

	credentials := reddit.Credentials{
		ID:       c.credentials.AppID,
		Secret:   c.credentials.AppSecret,
		Username: c.credentials.Username,
		Password: c.credentials.Password,
	}
	c.redditClient, err = reddit.NewClient(credentials)
	if err != nil {
		return clientError{error: fmt.Errorf("something wrong with reddit client, %s", err)}
	}
	return nil
}

type FetchNewPostsIterator struct {
	LastID  string
	FirstID string
}

func redditPostFromPostAPI(data reddit.Post) RedditPost {
	return RedditPost{
		FullID:     data.FullID,
		ID:         data.ID,
		Title:      data.Title,
		AuthorID:   data.AuthorID,
		AuthorName: data.Author,
		URL:        data.URL,
		SubReddit:  data.SubredditName,
	}
}

func redditPostFromCommentAPI(data reddit.Comment) RedditPost {
	return RedditPost{
		FullID:     data.FullID,
		ID:         data.ID,
		Title:      data.Body,
		AuthorID:   data.AuthorID,
		AuthorName: data.Author,
		SubReddit:  data.SubredditName,
	}
}
