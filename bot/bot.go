package bot

import (
	"github.com/TopiSenpai/Reddit-Discord-Bot/reddit"
	"github.com/disgoorg/log"
)

type Bot struct {
	Logger log.Logger

	Reddit *reddit.Client
}
