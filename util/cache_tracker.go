package util

import (
	"encoding/json"
	"sync"
	"time"

	anthropic "aurora/typings/anthropic"
)

type cacheBlock struct {
	fp     string
	tokens int
}

type cacheTracker struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

var globalCacheTracker = &cacheTracker{
	seen: make(map[string]time.Time),
	ttl:  5 * time.Minute,
}

func cacheFingerprint(role, blockType, text string) string {
	return role + "|" + blockType + "|" + text
}

func (t *cacheTracker) RecordAnthropic(req anthropic.MessagesRequest) (creationTokens, readTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for fp, seenAt := range t.seen {
		if now.Sub(seenAt) > t.ttl {
			delete(t.seen, fp)
		}
	}

	blocks := collectCacheBlocks(req)
	if len(blocks) == 0 {
		return 0, 0
	}

	for _, b := range blocks {
		if _, ok := t.seen[b.fp]; ok {
			readTokens += b.tokens
		} else {
			creationTokens += b.tokens
			t.seen[b.fp] = now
		}
	}
	return creationTokens, readTokens
}

func collectCacheBlocks(req anthropic.MessagesRequest) []cacheBlock {
	var blocks []cacheBlock

	// system blocks
	if len(req.System) > 0 {
		if req.System[0] == '[' {
			var rawBlocks []map[string]interface{}
			if err := json.Unmarshal(req.System, &rawBlocks); err == nil {
				for _, m := range rawBlocks {
					if _, hasCache := m["cache_control"]; hasCache {
						text, _ := m["text"].(string)
						fp := cacheFingerprint("system", "text", text)
						tokens := CountToken(text)
						if tokens < 1 && len(text) > 0 {
							tokens = 1
						}
						blocks = append(blocks, cacheBlock{fp, tokens})
					}
				}
			}
		}
	}

	// message blocks
	for _, msg := range req.Messages {
		if len(msg.Content) == 0 || msg.Content[0] != '[' {
			continue
		}
		var rawBlocks []map[string]interface{}
		if err := json.Unmarshal(msg.Content, &rawBlocks); err != nil {
			continue
		}
		for _, m := range rawBlocks {
			if _, hasCache := m["cache_control"]; hasCache {
				text, _ := m["text"].(string)
				typ, _ := m["type"].(string)
				fp := cacheFingerprint(msg.Role, typ, text)
				tokens := CountToken(text)
				if tokens < 1 && len(text) > 0 {
					tokens = 1
				}
				blocks = append(blocks, cacheBlock{fp, tokens})
			}
		}
	}

	return blocks
}

// RecordAnthropicCache parses real cache_control blocks in Anthropic requests.
func RecordAnthropicCache(req anthropic.MessagesRequest) (creationTokens, readTokens int) {
	return globalCacheTracker.RecordAnthropic(req)
}
