package initialize

import (
	"aurora/internal/duckgo"
	anthropic "aurora/typings/anthropic"
	"aurora/util"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// anthropicStreamResult holds the output-side telemetry for an Anthropic request.
type anthropicStreamResult struct {
	text         string
	outputTokens int
	ttftMs       int64
	totalMs      int64
}

func (h *Handler) messagesHandler(c *gin.Context) {
	var req anthropic.MessagesRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	if req.Model == "" {
		req.Model = "claude-haiku-4-5"
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 1024
	}

	// thinking (extended thinking) → reasoning_effort.
	effort := anthropic.ThinkingToEffort(req.Thinking)
	if effort == "" {
		effort = os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
	}

	// Convert Anthropic messages → internal OpenAI-shaped request.
	apiReq := anthropic.ToOpenAPIRequest(req)
	apiReq.ReasoningEffort = effort

	// Input token counting + prompt-cache simulation.
	inputTokens := util.CountMessagesTokens(apiReq.Messages)
	cacheCreation, cacheRead := util.RecordAnthropicCache(req)
	promptHash := util.HashPrompt(messagesText(apiReq.Messages))
	if cacheCreation == 0 && cacheRead == 0 {
		// Fallback to prompt hash simulation if request has no explicit cache_control blocks
		cacheCreation, cacheRead = util.RecordCache(promptHash, inputTokens)
	}

	translated, response, err := h.startDuckDuckGoRequest(apiReq)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": duckgo.ReadResponseError(response).Error(),
			"type":    "upstream_error",
			"code":    response.Status,
			"model":   translated.Model,
		}})
		return
	}

	// Cache breakdown is known before streaming; set before first flush.
	setCacheHeaders(c, promptHash, cacheCreation, cacheRead)

	start := time.Now()
	result := handleAnthropicStream(c, response.Body, req.Model, req.Stream, start, inputTokens, cacheCreation, cacheRead, effort)

	// Timing headers (only delivered for non-stream; for stream the same values
	// live in the message_delta event).
	c.Header("X-TTFT-Ms", fmt.Sprintf("%d", result.ttftMs))
	c.Header("X-Total-Time-Ms", fmt.Sprintf("%d", result.totalMs))

	if c.Writer.Status() != 200 {
		return
	}

	if !req.Stream {
		c.JSON(200, buildAnthropicResponse(req.Model, result, inputTokens, cacheCreation, cacheRead, effort))
	}
}

// handleAnthropicStream reads DuckDuckGo's text SSE and emits Anthropic SSE events.
// For non-stream it accumulates the text and returns the result.
func handleAnthropicStream(c *gin.Context, body io.ReadCloser, model string, stream bool, start time.Time, inputTokens, cacheCreation, cacheRead int, effort string) anthropicStreamResult {
	defer body.Close()

	reader := bufio.NewReader(body)
	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
	}

	msgID := "msg_" + util.RandomHexadecimalString()

	var sb strings.Builder
	var firstTokenSet bool
	var ttftMs int64

	if stream {
		writeEvent(c, "message_start", anthropic.MessageStartEvent{
			Type: "message_start",
			Message: anthropic.AnthropicStartMessage{
				ID:      msgID,
				Type:    "message",
				Role:    "assistant",
				Model:   model,
				Content: []anthropic.ContentBlock{},
				Usage: anthropic.AnthropicUsage{
					InputTokens:              inputTokens,
					CacheCreationInputTokens: cacheCreation,
					CacheReadInputTokens:     cacheRead,
				},
			},
		})
		writeEvent(c, "content_block_start", anthropic.ContentBlockStartEvent{
			Type:         "content_block_start",
			Index:        0,
			ContentBlock: anthropic.ContentBlock{Type: "text", Text: ""},
		})
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return anthropicStreamResult{}
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:] // strip "data: "
		if strings.HasPrefix(line, "[DONE]") {
			continue
		}

		var delta struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &delta); err != nil {
			continue
		}
		if delta.Message == "" {
			continue
		}

		sb.WriteString(delta.Message)
		if !firstTokenSet {
			firstTokenSet = true
			ttftMs = time.Since(start).Milliseconds()
		}

		if stream {
			writeEvent(c, "content_block_delta", anthropic.ContentBlockDeltaEvent{
				Type:  "content_block_delta",
				Index: 0,
				Delta: anthropic.ContentDelta{Type: "text_delta", Text: delta.Message},
			})
		}
	}

	fullText := sb.String()
	outputTokens := util.CountToken(fullText)
	totalMs := time.Since(start).Milliseconds()

	if stream {
		writeEvent(c, "content_block_stop", anthropic.ContentBlockStopEvent{Type: "content_block_stop", Index: 0})
		writeEvent(c, "message_delta", anthropic.MessageDeltaEvent{
			Type:  "message_delta",
			Delta: anthropic.MessageDelta{StopReason: "end_turn"},
			Usage: anthropic.AnthropicUsage{OutputTokens: outputTokens},
		})
		writeEvent(c, "message_stop", anthropic.MessageStopEvent{Type: "message_stop"})
		c.Writer.Flush()
	}

	return anthropicStreamResult{
		text:         fullText,
		outputTokens: outputTokens,
		ttftMs:       ttftMs,
		totalMs:      totalMs,
	}
}

// writeEvent serializes an Anthropic SSE event and writes it to the client.
func writeEvent(c *gin.Context, eventType string, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.Writer.WriteString("event: " + eventType + "\n")
	c.Writer.WriteString("data: " + string(b) + "\n\n")
	c.Writer.Flush()
}

// buildAnthropicResponse builds the non-stream MessagesResponse.
func buildAnthropicResponse(model string, r anthropicStreamResult, inputTokens, cacheCreation, cacheRead int, effort string) anthropic.MessagesResponse {
	usage := anthropic.AnthropicUsage{
		InputTokens:              inputTokens,
		OutputTokens:             r.outputTokens,
		CacheCreationInputTokens: cacheCreation,
		CacheReadInputTokens:     cacheRead,
	}
	return anthropic.MessagesResponse{
		ID:         "msg_" + util.RandomHexadecimalString(),
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		Content:    []anthropic.ContentBlock{{Type: "text", Text: r.text}},
		StopReason: "end_turn",
		Usage:      usage,
	}
}
