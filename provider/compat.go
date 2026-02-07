package provider

import (
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// Re-export base types
type ChatRequest = base.ChatRequest
type ChatResponse = base.ChatResponse
type Message = base.Message
type ProviderSpec = base.ProviderSpec
type ProviderTransportSpec = base.ProviderTransportSpec
type BuildOptions = base.BuildOptions

// Re-export streaming types
type StreamChunk = streaming.StreamChunk
type ProviderStreamSpec = streaming.ProviderStreamSpec

// Re-export registry functions
var Register = base.Register
var Get = base.Get
var List = base.List
