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
	}
	remaining, err := strconv.ParseFloat(headers.Get("X-Ratelimit-Remaining"), 64)
	if err != nil {
		log.Error("error parsing x-ratelimit-remaining:", err.Error())
	}
	reset, err := strconv.ParseFloat(headers.Get("X-Ratelimit-Reset"), 64)
	if err != nil {
		log.Error("error parsing x-ratelimit-reset:", err.Error())
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

func (r *Reddit) CheckSubreddit(subreddit string) error {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/about.json", subreddit)
	rq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	rs, err := r.do(rq, true)
	if err != nil {
		return err
	}
	defer rs.Body.Close()

	if rs.StatusCode == http.StatusNotFound || rs.StatusCode == http.StatusBadRequest {
		return ErrSubredditNotFound
	} else if rs.StatusCode == http.StatusForbidden {
		return ErrSubredditForbidden
	}

	var response RedditResponse
	if err = json.NewDecoder(rs.Body).Decode(&response); err != nil {
		return err
	}

	if len(response.Data.Children) == 0 {
		return ErrSubredditNotFound
	}

	return nil
}

func (r *Reddit) GetPosts(client *Reddit, subreddit string, fetchType string, lastPost string) ([]RedditPost, string, error) {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/%s.json?raw_json=1&limit=100", subreddit, fetchType)
	if lastPost != "" {
		url += fmt.Sprintf("&before=%s", lastPost)
	}
	log.Debug("getting posts for url:", url)
	rq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	rs, err := client.do(rq, false)
	if err != nil {
		return nil, "", err
	}
	defer rs.Body.Close()

	if rs.StatusCode == http.StatusNotFound {
		return nil, "", ErrSubredditNotFound
	} else if rs.StatusCode == http.StatusForbidden {
		return nil, "", ErrSubredditForbidden
	}

	var response RedditResponse
	if err = json.NewDecoder(rs.Body).Decode(&response); err != nil {
		return nil, "", err
	}

	before := response.Data.Before
	posts := make([]RedditPost, 0, len(response.Data.Children))
	for i := range response.Data.Children {
		posts = append(posts, response.Data.Children[i].Data)
	}

	if response.Data.Before != "" {
		var morePosts []RedditPost
		morePosts, before, err = r.GetPosts(client, subreddit, fetchType, before)
		if err != nil {
			return nil, "", err
		}
		posts = append(posts, morePosts...)
	}

	if len(posts) > 0 && before == "" {
		before = posts[0].Name
	}

	return posts, before, nil
}

type RedditResponse struct {
	Data struct {
		Before   string `json:"before"`
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditPost struct {
	Selftext              string  `json:"selftext"`
	AuthorFullname        string  `json:"author_fullname"`
	Title                 string  `json:"title"`
	SubredditNamePrefixed string  `json:"subreddit_name_prefixed"`
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Author                string  `json:"author"`
	URL                   string  `json:"url"`
	Permalink             string  `json:"permalink"`
	CreatedUtc            float64 `json:"created_utc"`
}
