package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
)

func TestGPT(t *testing.T) {
	_ = config.InitForTestToml()
	assert := assert.New(t)
	ai := NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)
	assert.NotNil(ai)
	answer, err := ai.Conclude("What is the meaning of life?")
	assert.Nil(err)
	t.Log(answer)
}

func TestTranslateToZh(t *testing.T) {
	_ = config.InitForTestToml()
	assert := assert.New(t)
	ai := NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)
	assert.NotNil(ai)
	answer, err := ai.TranslateToZh("* SECURITY\r\n  * Compile with Go 1.23.7\r\n  * Bump x/oauth2 & x/crypto (#33704) (#33727)\r\n\r\n* PERFORMANCE\r\n  * Optimize user dashboard loading (#33686) (#33708)\r\n \r\n* BUGFIXES\r\n  * Fix navbar dropdown item align (#33782)\r\n  * Fix inconsistent closed issue list icon (#33722) (#33728)\r\n  * Fix for Maven Package Naming Convention Handling (#33678) (#33679)\r\n  * Improve Open-with URL encoding (#33666) (#33680)\r\n  * Deleting repository should unlink all related packages (#33653) (#33673)\r\n  * Fix omitempty bug (#33663) (#33670)\r\n  * Upgrade go-crypto from 1.1.4 to 1.1.6 (#33745) (#33754)\r\n  * Fix OCI image.version annotation for releases to use full semver (#33698) (#33701)\r\n  * Try to fix ACME path when renew (#33668) (#33693)\r\n  * Fix mCaptcha bug (#33659) (#33661)\r\n  * Git graph: don't show detached commits (#33645) (#33650)\r\n  * Use MatchPhraseQuery for bleve code search (#33628)\r\n  * Adjust appearence of commit status webhook (#33778) #33789\r\n  * Upgrade golang net from 0.35.0 -> 0.36.0 (#33795) #33796\r\n\r\nInstances on **[Gitea Cloud](https://cloud.gitea.com)** will be automatically upgraded to this version during the specified maintenance window.\r\n\r\n")
	assert.Nil(err)
	t.Log(answer)
}

func TestEmbedding(t *testing.T) {
	_ = config.InitForTestToml()
	assert := assert.New(t)

	ai := NewAIService(config.C.Openai.APIKey, config.C.Openai.BaseURL)
	assert.NotNil(ai)
	answer, err := ai.Embed("What is the meaning of life?")
	assert.Nil(err)
	t.Log(answer)
}
