package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	officialtypes "aurora/typings/official"
)

// ThinkingToEffort maps an Anthropic thinking config to a reasoning_effort level.
// DuckDuckGo only supports none/low, but we keep the richer level here so the web
// can display the resolved intensity; the handler maps it to none/low for upstream.
func ThinkingToEffort(thinking json.RawMessage) string {
	if len(thinking) == 0 {
		return ""
	}
	var t struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens"`
	}
	if err := json.Unmarshal(thinking, &t); err != nil {
		return ""
	}
	if t.Type != "enabled" {
		return "none"
	}
	switch {
	case t.BudgetTokens >= 10000:
		return "high"
	case t.BudgetTokens >= 5000:
		return "medium"
	default:
		return "low"
	}
}

// systemText extracts the plain text from an Anthropic system prompt (string or
// array of text blocks).
func systemText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Text != "" {
				sb.WriteString(b.Text)
				sb.WriteString("\n")
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

// contentBlocks parses a message's content into Anthropic content blocks.
func contentBlocks(raw json.RawMessage) []ContentBlock {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if s == "" {
				return nil
			}
			return []ContentBlock{{Type: "text", Text: s}}
		}
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	return blocks
}

// ToOpenAPIRequest converts an Anthropic Messages request into the internal
// OpenAI-shaped APIRequest that the rest of the proxy understands.
func ToOpenAPIRequest(req MessagesRequest) officialtypes.APIRequest {
	out := officialtypes.APIRequest{
		Model:  req.Model,
		Stream: req.Stream,
	}

	// system prompt → leading system message
	if sys := systemText(req.System); sys != "" {
		out.Messages = append(out.Messages, officialtypes.ApiMessage{
			Role:    "system",
			Content: sys,
		})
	}

	for _, msg := range req.Messages {
		role := msg.Role
		blocks := contentBlocks(msg.Content)
		if len(blocks) == 0 {
			continue
		}
		converted := convertBlocks(role, blocks)
		out.Messages = append(out.Messages, converted...)
	}

	return out
}

// toImagePart builds an OpenAI image_url content part from an Anthropic image source.
func toImagePart(src *ImageSource) map[string]interface{} {
	return map[string]interface{}{
		"type": "image_url",
		"image_url": map[string]interface{}{
			"url": fmt.Sprintf("data:%s;base64,%s", src.MediaType, src.Data),
		},
	}
}

// convertBlocks turns Anthropic content blocks into one or more OpenAI messages.
// Because DuckDuckGo only carries text/images, tool_use/tool_result/thinking
// blocks are folded into text so conversation context is preserved.
func convertBlocks(role string, blocks []ContentBlock) []officialtypes.ApiMessage {
	var textParts []string
	var imageParts []map[string]interface{}

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				textParts = append(textParts, b.Text)
			}
		case "image":
			if b.Source != nil && b.Source.Data != "" {
				imageParts = append(imageParts, toImagePart(b.Source))
			}
		case "thinking":
			// No CoT upstream; drop the thinking block (effort is mapped at the
			// request level). We do not surface thinking text.
		case "tool_use":
			// Fold assistant tool calls into text so history stays coherent.
			inputStr := ""
			if len(b.Input) > 0 {
				inputStr = string(b.Input)
			}
			textParts = append(textParts, fmt.Sprintf("[Tool call: %s]\n%s", b.Name, inputStr))
		case "tool_result":
			// Fold user tool results into text.
			res := contentBlockText(b.Content)
			textParts = append(textParts, fmt.Sprintf("[Tool result for %s]\n%s", b.ToolUseID, res))
		}
	}

	content := buildContent(textParts, imageParts)
	if content == nil {
		return nil
	}
	return []officialtypes.ApiMessage{{Role: role, Content: content}}
}

// buildContent assembles the final Content value: plain string for text-only,
// array for mixed/multimodal.
func buildContent(textParts []string, imageParts []map[string]interface{}) interface{} {
	text := strings.Join(textParts, "\n")
	if len(imageParts) == 0 {
		if text == "" {
			return nil
		}
		return text
	}
	var parts []map[string]interface{}
	if text != "" {
		parts = append(parts, map[string]interface{}{"type": "text", "text": text})
	}
	parts = append(parts, imageParts...)
	return parts
}

// contentBlockText extracts text from a tool_result content (string or blocks).
func contentBlockText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Text != "" {
				sb.WriteString(b.Text)
				sb.WriteString("\n")
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return string(raw)
}
