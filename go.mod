module github.com/trilithus/redditrepostnofans

go 1.19

require (
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/vartanbeno/go-reddit/v2 v2.0.1
)

replace github.com/vartanbeno/go-reddit/v2 => ../redditrepostbot.modules/go-reddit

require (
	github.com/golang/protobuf v1.2.0 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	golang.org/x/net v0.0.0-20190108225652-1e06a53dbb7e // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	google.golang.org/appengine v1.4.0 // indirect
)
