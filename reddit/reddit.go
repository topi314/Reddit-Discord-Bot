package reddit

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/rest/route"
	"github.com/disgoorg/log"
)

func New(logger log.Logger, client rest.Client, config Config) *Client {
	return &Client{
		logger: logger,
		client: client,
		config: config,
	}
}

type Client struct {
	logger log.Logger
	client rest.Client
	config Config

	token      string
	expiration time.Time
	tokenMu    sync.Mutex
}

func (c *Client) GetSubreddit(name string) {
	compiledRoute, err := GetSubreddit.Compile(nil, name)
	if err != nil {
		return
	}
	var
	c.Do(compiledRoute, nil, )
}

func (c *Client) Do(route *route.CompiledAPIRoute, rqBody any, rsBody any, opts ...rest.RequestOpt) error {
	token := c.GetToken()
	opts = append(opts, rest.WithHeader("User-Agent", fmt.Sprintf("<%s>:<%s>:<%s> (by /u/%s)", disgo.OS, c.config.AppID, c.config.Version, c.config.Username)))
	opts = append(opts, rest.WithHeader("Authorization", fmt.Sprintf("bearer %s", token)))
	return c.client.Do(route, rqBody, rsBody, opts...)
}

func (c *Client) GetToken() string {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	return c.getToken()
}

func (c *Client) getToken() string {
	if c.token == "" || time.Now().After(c.expiration) {
		auth, err := c.GetRefreshToken()
		if err != nil {
			c.logger.Error("failed to refresh reddit bearer token, retrying in 5s: ", err)
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			<-timer.C
			return c.GetToken()
		}
		c.token = auth.AccessToken
		c.expiration = time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)
	}
	return c.token
}

func (c *Client) GetRefreshToken() (auth Authorization, err error) {
	compiledRoute, err := GetAccessToken.Compile(nil)
	if err != nil {
		return
	}
	body := url.Values{
		"grant_type": {"refresh_token"},
	}

	basic := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.config.ClientID, c.config.ClientSecret)))
	err = c.client.Do(compiledRoute, body, &auth, rest.WithHeader("Authorization", "Basic "+basic))
	return
}

type Authorization struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
