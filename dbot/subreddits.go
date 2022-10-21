package dbot

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/snowflake/v2"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const EmbedColor = 0xff581a

type Subreddits struct {
	bot        *Bot
	mu         sync.RWMutex
	subreddits map[string]*Subreddit
}

func (s *Subreddits) SubscribeToSubreddit(name string, client webhook.Client) {
	s.mu.RLock()
	subreddit, ok := s.subreddits[name]
	s.mu.RUnlock()
	if !ok {
		subreddit = NewSubreddit(s.bot)
		s.mu.Lock()
		s.subreddits[name] = subreddit
		s.mu.Unlock()
	}
	subreddit.AddClient(client)
	if !ok {
		go subreddit.Listen()
	}
}

func (s *Subreddits) UnsubscribeFromSubreddit(name string, webhookID snowflake.ID) error {
	s.mu.RLock()
	subreddit, ok := s.subreddits[name]
	s.mu.RUnlock()
	if ok {
		subreddit.RemoveClient(webhookID)
	}
}

func (s *Subreddits) DeleteSubreddit(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subreddits, name)
}

func NewSubreddit(bot *Bot) *Subreddit {
	return &Subreddit{
		bot:      bot,
		workChan: make(chan struct{}, 1),
	}
}

type Subreddit struct {
	bot      *Bot
	name     string
	mu       sync.Mutex
	clients  []webhook.Client
	workChan chan struct{}
}

func (s *Subreddit) AddClient(client webhook.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = append(s.clients, client)
}

func (s *Subreddit) RemoveClient(webhookID snowflake.ID) {
	if err := s.bot.DB.RemoveSubscription(webhookID, s.name); err != nil {
		s.bot.Logger.Errorf("error while removing subscription from db: %s", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, client := range s.clients {
		if client.ID() == webhookID {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
	if len(s.clients) == 0 {
		s.Kill()
	}
}

func (s *Subreddit) Listen() {
	s.bot.Logger.Debugf("listening to r/%s", s.name)
	defer s.bot.Logger.Debugf("stop listening to r/%s", s.name)
	defer close(s.workChan)
	var lastPostTime time.Time
	for {
		_, ok := <-s.workChan
		if !ok {
			return
		}
		posts, _, err := s.bot.Reddit.Subreddit.NewPosts(context.TODO(), s.name, nil)
		if err != nil {
			s.processError(err)
			continue
		}
		for _, post := range posts {
			if post.Created.After(lastPostTime) {
				s.processPost(*post)
				lastPostTime = post.Created.Time
			}
		}
	}
}

func (s *Subreddit) RemoveAndKill() {
	if err := s.bot.DB.RemoveAllSubscriptions(s.name); err != nil {
		s.bot.Logger.Errorf("error while removing all subscriptions from db: %s", err)
	}
	s.bot.Subreddits.DeleteSubreddit(s.name)

	close(s.workChan)
	s.mu.Lock()
	defer s.mu.Unlock()
	var wg sync.WaitGroup
	for i := range s.clients {
		wg.Add(1)
		client := s.clients[i]
		go func() {
			_, _ = client.CreateMessage(discord.WebhookMessageCreate{
				Embeds: []discord.Embed{discord.NewEmbedBuilder().
					SetDescriptionf("`r/%s` is no longer available or private. Deleting webhook...", s.name).
					SetColor(EmbedColor).
					Build(),
				},
			})
			_ = client.DeleteWebhook()
		}()
	}
	wg.Wait()
}

func (s *Subreddit) Kill() {
	close(s.workChan)
}

func (s *Subreddit) processPost(post reddit.Post) {
	title := cutString(post.Title, 256)
	description := cutString(post.Body, 4096)

	url := post.URL
	if !imageRegex.MatchString(url) {
		url = ""
	}

	message := discord.WebhookMessageCreate{
		Embeds: []discord.Embed{discord.NewEmbedBuilder().
			SetTitle(title).
			SetURL("https://www.reddit.com"+post.Permalink).
			SetColor(EmbedColor).
			SetAuthorName("New post on "+post.SubredditNamePrefixed).
			SetAuthorURL("https://www.reddit.com/"+post.SubredditNamePrefixed).
			SetDescription(description).
			SetImage(url).
			AddField("Author", post.Author, false).
			Build(),
		},
	}

	s.broadcastMessage(message)
}

func (s *Subreddit) processError(err error) {
	if rErr, ok := err.(*reddit.ErrorResponse); ok && rErr.Response.StatusCode == http.StatusNotFound {
		s.bot.Logger.Errorf("r/%s doesn't exist", s.name)
		s.RemoveAndKill()
		return
	} else if rErr, ok = err.(*reddit.ErrorResponse); ok && rErr.Response.StatusCode == http.StatusForbidden {
		s.bot.Logger.Errorf("r/%s is private", s.name)
		s.RemoveAndKill()
		return
	}
	s.bot.Logger.Errorf("error while getting posts from r/%s: %s", s.name, err)
}

func (s *Subreddit) broadcastMessage(message discord.WebhookMessageCreate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var wg sync.WaitGroup
	for i := range s.clients {
		wg.Add(1)
		client := s.clients[i]
		go func() {
			defer wg.Done()
			if _, err := client.CreateMessage(message); err != nil {
				s.bot.Logger.Error(err)
				s.handleWebhookError(err, client)
			}
		}()
	}
	wg.Wait()
}

func (s *Subreddit) handleWebhookError(err error, client webhook.Client) {
	if rErr, ok := err.(*rest.Error); ok && rErr.Response.StatusCode == http.StatusNotFound {
		go s.RemoveClient(client.ID())
		return
	}
	s.bot.Logger.Errorf("error while sending post to webhook %s: %s", client.ID(), err)
}

func cutString(str string, maxLen int) string {
	runes := []rune(str)
	if len(runes) > maxLen {
		return string(runes[0:maxLen-1]) + "…"
	}
	return string(runes)
}
