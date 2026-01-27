package provider

func init() {
	Register("openai", NewOpenAICompatSpec("openai", "https://api.openai.com/v1"))
	Register("moonshot", NewOpenAICompatSpec("moonshot", "https://api.moonshot.cn/v1"))
	Register("deepseek", NewOpenAICompatSpec("deepseek", "https://api.deepseek.com/v1"))
}
