package reddit

type Kind string

const (
	KindComment           Kind = "t1"
	KindUser              Kind = "t2"
	KindPost              Kind = "t3"
	KindMessage           Kind = "t4"
	KindSubreddit         Kind = "t5"
	KindTrophy            Kind = "t6"
	KindListing           Kind = "Listing"
	KindSubredditSettings Kind = "subreddit_settings"
	KindKarmaList         Kind = "KarmaList"
	KindTrophyList        Kind = "TrophyList"
	KindUserList          Kind = "UserList"
	KindMore              Kind = "more"
	KindLiveThread        Kind = "LiveUpdateEvent"
	KindLiveThreadUpdate  Kind = "LiveUpdate"
	KindModAction         Kind = "modaction"
	KindMulti             Kind = "LabeledMulti"
	KindMultiDescription  Kind = "LabeledMultiDescription"
	KindWikiPage          Kind = "wikipage"
	KindWikiPageListing   Kind = "wikipagelisting"
	KindWikiPageSettings  Kind = "wikipagesettings"
	KindStyleSheet        Kind = "stylesheet"
)

type PostHint string

const (
	PostHintImage       PostHint = "image"
	PostHintSelf        PostHint = "self"
	PostHintRichVideo   PostHint = "rich:video"
	PostHintHostedVideo PostHint = "hosted:video"
)

type Thing[D any] struct {
	Kind Kind `json:"kind"`
	Data D    `json:"data"`
}

type Listing[D any] struct {
	After    string     `json:"after"`
	Before   string     `json:"before"`
	Dist     int        `json:"dist"`
	Children []Thing[D] `json:"children"`
}

type SubredditAbout struct {
	Title               string `json:"title"`
	DisplayNamePrefixed string `json:"display_name_prefixed"`
	DisplayName         string `json:"display_name"`
	IconImg             string `json:"icon_img"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	URL                 string `json:"url"`
	ID                  string `json:"id"`
	CommunityIcon       string `json:"community_icon"`
}

type Post struct {
	ID         string     `json:"id,omitempty"`
	Name       string     `json:"name,omitempty"`
	CreatedUTC Timestamp  `json:"created_utc,omitempty"`
	Edited     *Timestamp `json:"edited,omitempty"`

	Permalink string `json:"permalink,omitempty"`
	URL       string `json:"url,omitempty"`

	Title    string `json:"title,omitempty"`
	Selftext string `json:"selftext,omitempty"`

	SubredditName         string `json:"subreddit,omitempty"`
	SubredditNamePrefixed string `json:"subreddit_name_prefixed,omitempty"`
	SubredditID           string `json:"subreddit_id,omitempty"`

	Author         string `json:"author,omitempty"`
	AuthorFullname string `json:"author_fullname,omitempty"`

	Spoiler bool `json:"spoiler"`
	Locked  bool `json:"locked"`
	NSFW    bool `json:"over_18"`

	PostHint            PostHint     `json:"post_hint"`
	Preview             *PostPreview `json:"preview,omitempty"`
	URLOverriddenByDest string       `json:"url_overridden_by_dest,omitempty"`
}

type PostPreview struct {
	Images struct {
		Source struct {
			Url string `json:"url"`
		} `json:"source"`
		Id string `json:"id"`
	} `json:"images"`
}
