package reddit

import "github.com/disgoorg/disgo/rest/route"

var (
	BasePath = "https://oauth.reddit.com/"

	GetAccessToken = route.NewCustomAPIRoute(route.POST, "https://www.reddit.com/api/v1/", "access_token")

	SearchSubredditNames = route.NewCustomAPIRoute(route.POST, BasePath, "search_reddit_names", "query", "include_over_18", "include_unadvertisable", "raw_json")

	GetSubreddit = route.NewCustomAPIRoute(route.GET, BasePath, "r/{subreddit}/about", "raw_json")

	NewSubredditPosts = route.NewCustomAPIRoute(route.GET, BasePath, "r/{subreddit}/new", "limit", "after", "raw_json")
)
