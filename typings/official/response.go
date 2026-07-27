package official

import "encoding/json"

type ChatCompletionChunk struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
	// Usage is only populated on the final chunk (stream_options.include_usage).
	Usage *usage `json:"usage,omitempty"`
	// Timing is only populated on the final usage chunk.
	Timing *Timing `json:"timing,omitempty"`
	// ReasoningEffort echoes the resolved thinking-effort level (non-standard extra field).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

func (chunk *ChatCompletionChunk) String() string {
	resp, _ := json.Marshal(chunk)
	return string(resp)
}

type Choices struct {
	Delta        Delta       `json:"delta"`
	Index        int         `json:"index"`
	FinishReason interface{} `json:"finish_reason"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

func NewChatCompletionChunk(text string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-4o-mini",
		Choices: []Choices{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}
}

func NewChatCompletionChunkWithModel(text string, model string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}
}

func StopChunkWithModel(reason string, model string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{
			{
				Index:        0,
				FinishReason: reason,
			},
		},
	}
}

func StopChunk(reason string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-4o-mini",
		Choices: []Choices{
			{
				Index:        0,
				FinishReason: reason,
			},
		},
	}
}

// UsageChunk emits the final chunk carrying usage + timing (OpenAI stream_options.include_usage compatible).
func UsageChunk(model string, promptTokens, completionTokens, cachedTokens int, ttftMs, totalMs int64, effort string) ChatCompletionChunk {
	tt := promptTokens + completionTokens
	var details *promptTokensDetails
	if cachedTokens > 0 {
		details = &promptTokensDetails{CachedTokens: cachedTokens}
	}
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{},
		Usage: &usage{
			PromptTokens:        promptTokens,
			CompletionTokens:    completionTokens,
			TotalTokens:         tt,
			PromptTokensDetails: details,
		},
		Timing: &Timing{
			TTFTMs:  ttftMs,
			TotalMs: totalMs,
		},
		ReasoningEffort: effort,
	}
}

type ChatCompletion struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Usage   usage    `json:"usage"`
	Choices []Choice `json:"choices"`
	// Timing carries TTFT and total duration (non-standard extra field).
	Timing *Timing `json:"timing,omitempty"`
	// ReasoningEffort echoes the resolved thinking-effort level (non-standard extra field).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}
type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Choice struct {
	Index        int         `json:"index"`
	Message      Msg         `json:"message"`
	FinishReason interface{} `json:"finish_reason"`
}
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// PromptTokensDetails holds cache breakdown (OpenAI-compatible).
	PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// Timing is attached to the final usage chunk (non-standard but harmless extra fields).
type Timing struct {
	TTFTMs  int64 `json:"ttft_ms,omitempty"`  // time to first token (ms)
	TotalMs int64 `json:"total_ms,omitempty"` // total request time (ms)
}

type ResponseAPI struct {
	ID                 string                 `json:"id"`
	Object             string                 `json:"object"`
	CreatedAt          int64                  `json:"created_at"`
	Status             string                 `json:"status"`
	Model              string                 `json:"model"`
	Output             []ResponseOutput       `json:"output"`
	OutputText         string                 `json:"output_text"`
	Usage              ResponseUsage          `json:"usage"`
	ParallelToolCalls  bool                   `json:"parallel_tool_calls"`
	PreviousResponseID interface{}            `json:"previous_response_id"`
	Error              interface{}            `json:"error"`
	IncompleteDetails  interface{}            `json:"incomplete_details"`
	Metadata           map[string]interface{} `json:"metadata"`
	// Timing carries TTFT and total duration (non-standard extra field).
	Timing *Timing `json:"timing,omitempty"`
	// ReasoningEffort echoes the resolved thinking-effort level (non-standard extra field).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type ResponseOutput struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Status  string                  `json:"status"`
	Role    string                  `json:"role"`
	Content []ResponseOutputContent `json:"content"`
}

type ResponseOutputContent struct {
	Type        string        `json:"type"`
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"`
}

type ResponseUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	InputTokensDetails  any `json:"input_tokens_details,omitempty"`
	OutputTokensDetails any `json:"output_tokens_details,omitempty"`
}

type ResponseStreamEvent struct {
	Type         string          `json:"type"`
	Sequence     int             `json:"sequence_number,omitempty"`
	Response     *ResponseAPI    `json:"response,omitempty"`
	Item         *ResponseOutput `json:"item,omitempty"`
	ItemID       string          `json:"item_id,omitempty"`
	Part         any             `json:"part,omitempty"`
	OutputIndex  int             `json:"output_index,omitempty"`
	ContentIndex int             `json:"content_index,omitempty"`
	Delta        string          `json:"delta,omitempty"`
	Text         string          `json:"text,omitempty"`
}

func (event ResponseStreamEvent) String() string {
	resp, _ := json.Marshal(event)
	return string(resp)
}

func NewResponseAPIWithModel(text string, model string) ResponseAPI {
	return NewResponseAPIFull(text, model, 0, 0, 0, 0, 0, "")
}

// NewResponseAPIFull builds a non-stream ResponseAPI with usage, cache breakdown and timing.
func NewResponseAPIFull(text, model string, inputTokens, outputTokens, cachedTokens, ttftMs, totalMs int64, effort string) ResponseAPI {
	var inputDetails any
	if cachedTokens > 0 {
		inputDetails = map[string]interface{}{"cached_tokens": cachedTokens}
	}
	return ResponseAPI{
		ID:        "resp_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:    "response",
		CreatedAt: 0,
		Status:    "completed",
		Model:     model,
		Output: []ResponseOutput{
			NewResponseOutput(text),
		},
		OutputText: text,
		Usage: ResponseUsage{
			InputTokens:        int(inputTokens),
			OutputTokens:       int(outputTokens),
			TotalTokens:        int(inputTokens + outputTokens),
			InputTokensDetails: inputDetails,
		},
		ParallelToolCalls: true,
		Metadata:          map[string]interface{}{},
		Timing: &Timing{
			TTFTMs:  ttftMs,
			TotalMs: totalMs,
		},
		ReasoningEffort: effort,
	}
}

func NewResponseOutput(text string) ResponseOutput {
	return ResponseOutput{
		ID:     "msg_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Type:   "message",
		Status: "completed",
		Role:   "assistant",
		Content: []ResponseOutputContent{
			{
				Type:        "output_text",
				Text:        text,
				Annotations: []interface{}{},
			},
		},
	}
}

func NewChatCompletionWithModel(text string, model string) ChatCompletion {
	return NewChatCompletionFull(text, model, 0, 0, 0, 0, 0, "")
}

// NewChatCompletionFull builds a non-stream ChatCompletion with usage, cache breakdown and timing.
func NewChatCompletionFull(text, model string, promptTokens, completionTokens, cachedTokens, ttftMs, totalMs int64, effort string) ChatCompletion {
	var details *promptTokensDetails
	if cachedTokens > 0 {
		details = &promptTokensDetails{CachedTokens: int(cachedTokens)}
	}
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   model,
		Usage: usage{
			PromptTokens:        int(promptTokens),
			CompletionTokens:    int(completionTokens),
			TotalTokens:         int(promptTokens + completionTokens),
			PromptTokensDetails: details,
		},
		Timing: &Timing{
			TTFTMs:  ttftMs,
			TotalMs: totalMs,
		},
		ReasoningEffort: effort,
		Choices: []Choice{
			{
				Message: Msg{
					Content: text,
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
}

func NewChatCompletion(full_test string, input_tokens, output_tokens int) ChatCompletion {
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   "gpt-4o-mini",
		Usage: usage{
			PromptTokens:     input_tokens,
			CompletionTokens: output_tokens,
			TotalTokens:      input_tokens + output_tokens,
		},
		Choices: []Choice{
			{
				Message: Msg{
					Content: full_test,
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
}
