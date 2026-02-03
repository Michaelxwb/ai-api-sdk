package openai

import "github.com/Michaelxwb/ai-api-sdk/provider/base"

func init() {
	base.Register("openai", NewOpenAICompatSpec("openai", "https://api.openai.com/v1"))
	base.Register("moonshot", NewOpenAICompatSpec("moonshot", "https://api.moonshot.cn/v1"))
	base.Register("deepseek", NewOpenAICompatSpec("deepseek", "https://api.deepseek.com/v1"))
}
