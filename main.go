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
	passwordFilename     = "password.json"
	subredditName        = "bdsmgw"
	repostSubredditName  = "u_bdsmgw_repost_bot"
	maxConcurrentJobs    = 2
	ttlForPaidSiteUsers  = time.Hour * 48
	ttlForProcessedPosts = time.Hour * 48
)

var (
	matchPaidSites = regexp.MustCompile(".*(onlyfans|fansly).*")
)

func main() {
	ctx := context.Background()

	postStorage := storage.NewStorage[any]("data_posts.json")
	contentStorage := storage.NewStorage[any]("data_content.json")
	onlyFansUsersStorage := storage.NewStorage[time.Time]("data_onlyfansusers.json")
	processedStorage := storage.NewStorage[time.Time]("data_processed.json")

	redditClient := redditclient.NewRedditClient(passwordFilename)
	if err := redditClient.Login(ctx); err != nil {
		log.Fatalf("could not log in reddit client, %s", err.Error())
	}

	for {
		iterator := redditclient.FetchNewPostsIterator{}
		posts := make([]redditclient.RedditPost, 0, 1000)
		for {
			fetchedPosts, err := redditClient.FetchNewPosts(ctx, subredditName, &iterator)
			if err != nil {
				if !err.CanRetry() {
					log.Fatalf(err.Error())
				} else {
					log.Printf("Client returned an error but we're allowed to retry: %s", err.Error())
					break
				}
			}
			if len(fetchedPosts) == 0 {
				break
			}
			posts = append(posts, fetchedPosts...)
		}

		processedPostCount := 0
		repostCount := 0

		jobQueue := make(chan int, maxConcurrentJobs)
		wg := sync.WaitGroup{}
		for _, post := range posts {
			if _, hasProcessed := processedStorage.Retrieve(post.FullID); hasProcessed {
				continue
			}
			processedStorage.Store(post.FullID, time.Now())

			processedPostCount++
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
					lowerURL := strings.ToLower(post.URL)

					//skip only fans
					if matchPaidSites.MatchString(lowerTitle) ||
						matchPaidSites.MatchString(lowerSubReddit) ||
						matchPaidSites.MatchString(lowerURL) || isPaidSitePoster {
						isPaidSitePoster = true
						break
					}
				}
				for _, comment := range commentHistory {
					lowerTitle := strings.ToLower(comment.Title)
					lowerSubReddit := strings.ToLower(comment.SubReddit)
					lowerURL := strings.ToLower(post.URL)

					//skip only fans
					if matchPaidSites.MatchString(lowerTitle) ||
						matchPaidSites.MatchString(lowerSubReddit) ||
						matchPaidSites.MatchString(lowerURL) || isPaidSitePoster {
						isPaidSitePoster = true
						break
					}
				}

				if isPaidSitePoster {
					now := time.Now()
					onlyFansUsersStorage.Store(post.AuthorID, now)
					return
				} else {
					repostCount++
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

		processedPostIDs := processedStorage.GetKeys()
		now := time.Now()
		for _, postID := range processedPostIDs {
			creationTime, _ := processedStorage.Retrieve(postID)
			if now.After(creationTime.Add(ttlForProcessedPosts)) {
				processedStorage.Erase(postID)
			}
		}

		if processedPostCount == 0 {
			log.Println("-- no new posts --")
			time.Sleep(time.Second * 90)
			continue
		}

		if processedPostCount > 0 && repostCount == 0 {
			log.Printf("-- processed %d posts, no new reposts -- \n\n", processedPostCount)
			time.Sleep(time.Second * 300)
		} else {
			log.Printf("-- processed %d posts, %d (s) reposts -- \n\n", processedPostCount, repostCount)
			time.Sleep(time.Second * 30)
		}
	}

}
