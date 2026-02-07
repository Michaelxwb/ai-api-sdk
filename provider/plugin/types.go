package plugin

// MessageType 消息类型
type MessageType string

const (
	MsgStartLocating  MessageType = "start_locating"
	MsgLocatingResult MessageType = "locating_result"
	MsgSendMessage    MessageType = "send_message"
	MsgReplyChunk     MessageType = "reply_chunk"
	MsgReplyDone      MessageType = "reply_done"
	MsgError          MessageType = "error"
)

// Message 基础消息结构
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// ElementLocator 元素定位信息
type ElementLocator struct {
	Selector   string            `json:"selector"`
	XPath      string            `json:"xpath,omitempty"`
	Attributes map[string]string `json:"attributes"`
}

// ElementLocators 完整定位信息
type ElementLocators struct {
	Input       ElementLocator  `json:"input"`
	SendButton  ElementLocator  `json:"sendButton"`
	ReplyArea   ElementLocator  `json:"replyArea"`
	CreateChat  *ElementLocator `json:"createChat,omitempty"`
	PlatformURL string          `json:"platformUrl"`
}
