package bot

import (
	"connect-text-bot/bot/connect/client"

	"github.com/google/uuid"
)

type (
	fConnect map[uuid.UUID]Bot

	Bot struct {
		connect *client.Client
	}
)

var botsConnect = make(fConnect)
