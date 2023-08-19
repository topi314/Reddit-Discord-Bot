package redditbot

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/disgoorg/json"
	"github.com/disgoorg/log"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/oauth2"
)

func NewReddit(cfg RedditConfig) (*Reddit, error) {
	reddit := &Reddit{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL:  "https://www.reddit.com/api/v1/access_token",
				AuthStyle: oauth2.AuthStyleInHeader,
			},
		},
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}

	if _, err := reddit.getToken(); err != nil {
		return nil, err
	}

	return reddit, nil
}

type rateLimit struct {
	used      int
	remaining int
	reset     time.Time
}

type Reddit struct {
	config *oauth2.Config
	client *http.Client

	rateLimit rateLimit
	token     *oauth2.Token
	mu        sync.Mutex
}

func (r *Reddit) getToken() (*oauth2.Token, error) {
	if r.token.Valid() {
		return r.token, nil
	}

	token, err := r.config.Exchange(context.Background(), "", oauth2.SetAuthURLParam("grant_type", "client_credentials"))
	if err != nil {
		return nil, fmt.Errorf("error exchanging token: %w", err)
	}

	r.token = token

	return token, nil
}

func (r *Reddit) do(rq *http.Request, important bool) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("rate limit: used: %d, remaining: %d, reset: %s\n", r.rateLimit.used, r.rateLimit.remaining, r.rateLimit.reset.Format(time.RFC3339))

	now := time.Now()
	limit := 10
	if important {
		limit = 1
	}
	var sleep time.Duration
	if r.rateLimit.remaining <= limit && now.Before(r.rateLimit.reset) {
		sleep = r.rateLimit.reset.Sub(now)
		time.Sleep(sleep)
	}

	token, err := r.getToken()
	if err != nil {
		return nil, err
	}

	rq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	rq.Header.Set("User-Agent", "discord:com.github.topi314.reddit-discord-bot:1.0.0 (by /u/TobiDragneel)")

	rs, err := r.client.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("error doing request: %w", err)
	}

	headers := rs.Header
	used, err := strconv.ParseFloat(headers.Get("X-Ratelimit-Used"), 64)
	if err != nil {
		log.Error("error parsing x-ratelimit-used:", err.Error())
		used = float64(r.rateLimit.used + 1)
	}
	remaining, err := strconv.ParseFloat(headers.Get("X-Ratelimit-Remaining"), 64)
	if err != nil {
		log.Error("error parsing x-ratelimit-remaining:", err.Error())
		remaining = float64(r.rateLimit.remaining - 1)
	}
	reset, err := strconv.ParseFloat(headers.Get("X-Ratelimit-Reset"), 64)
	if err != nil {
		log.Error("error parsing x-ratelimit-reset:", err.Error())
		reset = float64(r.rateLimit.reset.Unix())
	}

	r.rateLimit = rateLimit{
		used:      int(used),
		remaining: int(remaining),
		reset:     now.Add(time.Second * time.Duration(reset)),
	}

	redditRequests.With(prometheus.Labels{
		"path":      rq.URL.Path,
		"method":    rq.Method,
		"status":    strconv.Itoa(rs.StatusCode),
		"important": strconv.FormatBool(important),
		"sleep":     strconv.FormatInt(int64(sleep), 10),
		"used":      strconv.Itoa(r.rateLimit.used),
		"remaining": strconv.Itoa(r.rateLimit.remaining),
		"reset":     strconv.FormatInt(r.rateLimit.reset.Unix(), 10),
	}).Inc()

	return rs, nil
}

func (r *Reddit) GetPostsUntil(subreddit string, fetchType string, until time.Time, maxPages int) ([]RedditPost, error) {
	var (
		posts []RedditPost
		after string
		page  = 1
	)
	for {
		newPosts, err := r.getPosts(subreddit, fetchType, after)
		if err != nil {
			return nil, err
		}

		for i := range newPosts {
			createdAt := time.Unix(int64(newPosts[i].CreatedUtc), 0)
			if createdAt.Before(until) || createdAt.Equal(until) {
				return posts, nil
			}
			posts = append(posts, newPosts[i])
		}

		if len(newPosts) == 0 {
			return posts, nil
		}

		after = newPosts[len(newPosts)-1].Name

		page++
		if page > maxPages {
			return posts, nil
		}
	}
}

func (r *Reddit) getPosts(subreddit string, fetchType string, after string) ([]RedditPost, error) {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/%s.json?raw_json=1&sr_detail=true&limit=100", subreddit, fetchType)
	if after != "" {
		url += fmt.Sprintf("&after=%s", after)
	}
	log.Debug("getting posts for url: ", url)
	rq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	rs, err := r.do(rq, false)
	if err != nil {
		return nil, err
	}
	defer rs.Body.Close()

	if rs.StatusCode == http.StatusNotFound {
		return nil, ErrSubredditNotFound
	} else if rs.StatusCode == http.StatusForbidden {
		return nil, ErrSubredditForbidden
	}

	var response RedditResponse[RedditListing[RedditPost]]
	if err = json.NewDecoder(rs.Body).Decode(&response); err != nil {
		return nil, err
	}

	posts := make([]RedditPost, 0, len(response.Data.Children))
	for i := range response.Data.Children {
		posts = append(posts, response.Data.Children[i].Data)
	}

	return posts, nil
}

func (r *Reddit) CheckSubreddit(subreddit string) error {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/about.json?raw_json=1", subreddit)
	rq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	rs, err := r.do(rq, true)
	if err != nil {
		return err
	}
	defer rs.Body.Close()

	if rs.StatusCode == http.StatusNotFound {
		return ErrSubredditNotFound
	} else if rs.StatusCode == http.StatusForbidden {
		return ErrSubredditForbidden
	}

	var response RedditResponse[struct{}]
	if err = json.NewDecoder(rs.Body).Decode(&response); err != nil {
		return err
	}

	if response.Kind != "t5" {
		return ErrSubredditNotFound
	}

	return nil
}

type RedditResponse[T any] struct {
	Kind string `json:"kind"`
	Data T      `json:"data"`
}

type RedditListing[T any] struct {
	Before   string `json:"before"`
	Children []struct {
		Data T `json:"data"`
	} `json:"children"`
}

type RedditPost struct {
	Selftext              string          `json:"selftext"`
	AuthorFullname        string          `json:"author_fullname"`
	Title                 string          `json:"title"`
	SubredditNamePrefixed string          `json:"subreddit_name_prefixed"`
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Author                string          `json:"author"`
	URL                   string          `json:"url"`
	Permalink             string          `json:"permalink"`
	CreatedUtc            float64         `json:"created_utc"`
	SrDetail              SubredditDetail `json:"sr_detail"`
}

type SubredditDetail struct {
	CommunityIcon string `json:"community_icon"`
}
