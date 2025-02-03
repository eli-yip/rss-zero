package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRSSHubFeedURL(t *testing.T) {
	t.Run("supported feed type", func(t *testing.T) {
		testCases := []struct {
			feedType string
			username string
			expect   string
		}{
			{"weibo", "username", "https://rsshub.example.com/weibo/user/username/readable:true&authorNameBold=true&showEmojiForRetweet=true"},
			{"telegram", "username", "https://rsshub.example.com/telegram/channel/username/showLinkPreview=0&showViaBot=0&showReplyTo=0&showFwdFrom=0&showFwdFromAuthor=0&showInlineButtons=0&showMediaTagInTitle=1&showMediaTagAsEmoji=1&includeFwd=0&includeReply=1&includeServiceMsg=0&includeUnsupportedMsg=0"},
		}

		assert := assert.New(t)
		for _, tc := range testCases {
			feedURL, err := generateRSSHubFeedURL("https://rsshub.example.com", tc.feedType, tc.username)
			assert.Nil(err)
			assert.Equal(tc.expect, feedURL)
		}
	})

	t.Run("unsupported feed type", func(t *testing.T) {
		_, err := generateRSSHubFeedURL("https://rsshub.example.com", "unsupported", "username")
		assert.NotNil(t, err)
	})
}
