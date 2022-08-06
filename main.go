package main

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	redditclient "github.com/trilithus/redditrepostnofans/redditclient"
	"github.com/trilithus/redditrepostnofans/storage"
)

const (
	passwordFilename    = "password.json"
	subredditName       = "bdsmgw"
	repostSubredditName = "u_bdsmgw_repost_bot"
	maxConcurrentJobs   = 2
	ttlForPaidSiteUsers = time.Hour * 48
)

var (
	matchPaidSites = regexp.MustCompile(".*(onlyfans|fansly).*")
)

func main() {
	ctx := context.Background()

	postStorage := storage.NewStorage[any]("data_posts.json")
	contentStorage := storage.NewStorage[any]("data_content.json")
	onlyFansUsersStorage := storage.NewStorage[time.Time]("data_onlyfansusers.json")

	redditClient := redditclient.NewRedditClient(passwordFilename)
	if err := redditClient.Login(ctx); err != nil {
		log.Fatalf("could not log in reddit client, %s", err.Error())
	}

	iterator := redditclient.FetchNewPostsIterator{}
	for {
		posts, err := redditClient.FetchNewPosts(ctx, subredditName, &iterator)
		if err != nil {
			if !err.CanRetry() {
				log.Fatalf(err.Error())
			} else {
				log.Printf("Client returned an error but we're allowed to retry: %s", err.Error())
				continue
			}
		}

		jobQueue := make(chan int, maxConcurrentJobs)
		wg := sync.WaitGroup{}
		for _, post := range posts {
			if timeStamp, hasUser := onlyFansUsersStorage.Retrieve(post.AuthorID); hasUser {
				if time.Now().Before(timeStamp.Add(ttlForPaidSiteUsers)) {
					continue
				}
			}
			if _, postHasBeenCrossposted := postStorage.Retrieve(post.FullID); postHasBeenCrossposted {
				continue
			}
			if _, contentHasBeenCrossposted := contentStorage.Retrieve(post.URL); contentHasBeenCrossposted {
				continue
			}

			wg.Add(1)
			jobQueue <- 1
			go func(post redditclient.RedditPost) {
				defer func() { <-jobQueue }()
				defer wg.Done()

				var postHistory []redditclient.RedditPost
				backoff.Retry(func() error {
					var err redditclient.ClientError
					postHistory, err = redditClient.FetchProfilePostHistory(ctx, post.AuthorName)
					if err != nil {
						if err.CanRetry() {
							return err
						} else {
							return &backoff.PermanentError{Err: err}
						}
					}
					return nil
				}, backoff.NewExponentialBackOff())

				var commentHistory []redditclient.RedditPost
				backoff.Retry(func() error {
					var err redditclient.ClientError
					commentHistory, err = redditClient.FetchProfileCommentHistory(ctx, post.AuthorName)
					if err != nil {
						if err.CanRetry() {
							return err
						} else {
							return &backoff.PermanentError{Err: err}
						}
					}
					return nil
				}, backoff.NewExponentialBackOff())

				isPaidSitePoster := false
				for _, post := range postHistory {
					lowerTitle := strings.ToLower(post.Title)
					lowerSubReddit := strings.ToLower(post.SubReddit)

					//skip only fans
					if matchPaidSites.MatchString(lowerTitle) || matchPaidSites.MatchString(lowerSubReddit) || isPaidSitePoster {
						isPaidSitePoster = true
						break
					}
				}
				for _, comment := range commentHistory {
					lowerTitle := strings.ToLower(comment.Title)
					lowerSubReddit := strings.ToLower(comment.SubReddit)

					//skip only fans
					if matchPaidSites.MatchString(lowerTitle) || matchPaidSites.MatchString(lowerSubReddit) || isPaidSitePoster {
						isPaidSitePoster = true
						break
					}
				}

				if isPaidSitePoster {
					now := time.Now()
					onlyFansUsersStorage.Store(post.AuthorID, now)
					return
				} else {
					log.Printf("[%s]\t%s\n", post.FullID, post.Title)
					crossPost, err := redditClient.CrossPost(ctx, post, repostSubredditName)
					if err != nil {
						log.Fatal(err.Error())
						return
					}
					postStorage.Store(post.FullID, nil)
					contentStorage.Store(post.URL, nil)
					if crossPost != nil {
						time.Sleep(time.Second * 5)
						err = redditClient.ApprovePost(ctx, *crossPost)
						if err != nil {
							log.Fatal(err.Error())
						}
					}
				}
			}(post)
		}
		wg.Wait()

		if len(posts) == 0 {
			time.Sleep(time.Second * 90)
		} else {
			log.Print("\n\n -- end of batch -- \n\n")
		}
	}

}
