package openai

import "github.com/Michaelxwb/ai-api-sdk/provider/base"

func init() {
	base.Register("openai", NewOpenAICompatSpec("openai", "https://api.openai.com/v1"))
	base.Register("moonshot", NewOpenAICompatSpec("moonshot", "https://api.moonshot.cn/v1"))
	base.Register("deepseek", NewOpenAICompatSpec("deepseek", "https://api.deepseek.com/v1"))
	base.Register("dashscope", NewOpenAICompatSpec("dashscope", "https://dashscope.aliyuncs.com/compatible-mode/v1"))
	base.Register("volcengine", NewOpenAICompatSpec("volcengine", "https://ark.cn-beijing.volces.com/api/v3"))
	base.Register("qianfan", NewOpenAICompatSpec("qianfan", "https://qianfan.baidubce.com/v2"))
}
