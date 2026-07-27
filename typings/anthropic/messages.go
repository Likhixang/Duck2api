// Package anthropic provides types and converters for the Anthropic Messages API
// (the native protocol used by Claude Code). DuckDuckGo's upstream only returns
// plain text, so every Anthropic-specific concept (content blocks, tool_use,
// thinking, system prompts) is translated to/from the internal OpenAI-shaped
// representation that the rest of the proxy already speaks.
package anthropic

import (
	"encoding/json"
)

// ---------- Request ----------

// MessagesRequest is the Anthropic Messages API request body.
type MessagesRequest struct {
	Model         string          `json:"model"`
	Messages      []Message       `json:"messages"`
	System        json.RawMessage `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Tools         []Tool          `json:"tools,omitempty"`
	ToolChoice    json.RawMessage `json:"tool_choice,omitempty"`
	Thinking      json.RawMessage `json:"thinking,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          int             `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// Message is a single Anthropic message. Content may be a plain string or an
// array of content blocks (text, image, tool_use, tool_result, thinking).
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ContentBlock is one block inside a message's content array.
type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *ImageSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`          // tool_use
	Name      string          `json:"name,omitempty"`        // tool_use
	Input     json.RawMessage `json:"input,omitempty"`       // tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result
	Content   json.RawMessage `json:"content,omitempty"`     // tool_result
	IsError   bool            `json:"is_error,omitempty"`
	Thinking  string          `json:"thinking,omitempty"` // thinking block
	Signature string          `json:"signature,omitempty"`
}

// ImageSource describes a base64 image in Anthropic format.
type ImageSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Tool is an Anthropic tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ---------- Non-stream response ----------

// MessagesResponse is the Anthropic Messages API non-stream response.
type MessagesResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"` // "message"
	Role         string          `json:"role"` // "assistant"
	Model        string          `json:"model"`
	Content      []ContentBlock  `json:"content"`
	StopReason   string          `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        AnthropicUsage  `json:"usage"`
}

// AnthropicUsage is the usage object in Anthropic responses.
type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ---------- Streaming events ----------

// MessageStartEvent is the first SSE event (message_start).
type MessageStartEvent struct {
	Type    string               `json:"type"`
	Message AnthropicStartMessage `json:"message"`
}

// AnthropicStartMessage is the message payload inside message_start.
type AnthropicStartMessage struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []ContentBlock `json:"content"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        AnthropicUsage `json:"usage"`
}

// ContentBlockStartEvent announces a new content block.
type ContentBlockStartEvent struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// ContentBlockDeltaEvent carries an incremental delta for a content block.
type ContentBlockDeltaEvent struct {
	Type  string      `json:"type"`
	Index int         `json:"index"`
	Delta ContentDelta `json:"delta"`
}

// ContentDelta is the delta payload (text_delta, thinking_delta, input_json_delta).
type ContentDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	InputJSON   string `json:"input_json,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

// ContentBlockStopEvent marks the end of a content block.
type ContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// MessageDeltaEvent carries the stop reason and output usage.
type MessageDeltaEvent struct {
	Type  string       `json:"type"`
	Delta MessageDelta `json:"delta"`
	Usage AnthropicUsage `json:"usage"`
}

// MessageDelta is the delta payload inside message_delta.
type MessageDelta struct {
	StopReason   string  `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

// MessageStopEvent is the final SSE event.
type MessageStopEvent struct {
	Type string `json:"type"`
}

// PingEvent is an optional keepalive.
type PingEvent struct {
	Type string `json:"type"`
}
