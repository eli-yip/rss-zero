package macked

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotIface interface {
	SendText(chatID string, text string) (err error)
}

type Bot struct{ *bot.Bot }

func NewBot(token string) (BotIface, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, err
	}
	go b.Start(context.Background())
	return &Bot{b}, nil
}

func (b *Bot) SendText(chatID string, text string) (err error) {
	_, err = b.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:             chatID,
		Text:               bot.EscapeMarkdownUnescaped(text),
		ParseMode:          models.ParseModeMarkdown,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
	})
	return err
}
