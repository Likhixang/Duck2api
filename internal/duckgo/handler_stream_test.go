package duckgo

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	duckgotypes "aurora/typings/duckgo"
	"github.com/gin-gonic/gin"
)

// fakeCloser wraps a bytes.Buffer so it implements io.ReadCloser for http.Response.Body.
type fakeCloser struct {
	*bytes.Buffer
}

func (f fakeCloser) Close() error { return nil }

// mockWriter implements gin.ResponseWriter for CreateTestContext.
type mockWriter struct {
	*bytes.Buffer
	status int
	header http.Header
}

func newMockWriter() *mockWriter {
	return &mockWriter{Buffer: &bytes.Buffer{}, header: http.Header{}}
}

func (m *mockWriter) Header() http.Header   { return m.header }
func (m *mockWriter) WriteHeader(code int)   { m.status = code }
func (m *mockWriter) WriteHeaderNow()        {}
func (m *mockWriter) Status() int            { return m.status }
func (m *mockWriter) Size() int              { return m.Buffer.Len() }
func (m *mockWriter) Written() bool          { return m.Buffer.Len() > 0 }
func (m *mockWriter) Write(data []byte) (int, error) {
	return m.Buffer.Write(data)
}
func (m *mockWriter) WriteString(s string) (int, error) {
	return m.Buffer.WriteString(s)
}
func (m *mockWriter) Flush()                 {}
func (m *mockWriter) Hijack()                {}
func (m *mockWriter) Pusher() http.Pusher    { return nil }
func (m *mockWriter) CloseNotify() <-chan bool {
	return nil
}

// TestHandlerStreamUsage verifies the SSE output: content chunks, the [DONE] stop
// chunk, and the final usage+timing chunk emitted after it.
func TestHandlerStreamUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := newMockWriter()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{}

	sse := "data: {\"id\":\"m1\",\"action\":\"success\",\"created\":1,\"model\":\"gpt-5.4-nano\",\"role\":\"assistant\",\"message\":\"Hello\"}\n" +
		"data: {\"id\":\"m1\",\"action\":\"success\",\"created\":2,\"model\":\"gpt-5.4-nano\",\"role\":\"assistant\",\"message\":\" world\"}\n" +
		"data: [DONE]\n"

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       fakeCloser{Buffer: bytes.NewBufferString(sse)},
	}

	var oldReq duckgotypes.ApiRequest
	oldReq.Model = "gpt-5.4-nano"

	stats := HandlerStats{
		Start:        time.Now().Add(-500 * time.Millisecond),
		PromptTokens: 10,
		CachedTokens: 4,
		Effort:       "high",
	}
	result := Handler(c, resp, oldReq, true, stats)

	if result.Text != "Hello world" {
		t.Fatalf("unexpected text: %q", result.Text)
	}
	if result.OutputTokens <= 0 {
		t.Fatalf("expected output tokens > 0, got %d", result.OutputTokens)
	}
	if result.TTFTMs <= 0 {
		t.Fatalf("expected ttft_ms > 0, got %d", result.TTFTMs)
	}

	out := w.String()
	// Content chunks present.
	if !strings.Contains(out, `"content":"Hello"`) {
		t.Fatalf("missing first content chunk in:\n%s", out)
	}
	// Stop chunk with finish_reason present.
	if !strings.Contains(out, `"finish_reason":"stop"`) {
		t.Fatalf("missing stop chunk in:\n%s", out)
	}
	// Final usage chunk present and carries usage + timing + effort.
	if !strings.Contains(out, `"usage"`) {
		t.Fatalf("missing usage chunk in:\n%s", out)
	}
	if !strings.Contains(out, `"prompt_tokens":10`) {
		t.Fatalf("missing prompt_tokens in usage chunk:\n%s", out)
	}
	if !strings.Contains(out, `"cached_tokens":4`) {
		t.Fatalf("missing cached_tokens in usage chunk:\n%s", out)
	}
	if !strings.Contains(out, `"ttft_ms"`) {
		t.Fatalf("missing ttft_ms in usage chunk:\n%s", out)
	}
	if !strings.Contains(out, `"reasoning_effort":"high"`) {
		t.Fatalf("missing reasoning_effort in usage chunk:\n%s", out)
	}
	// The stop chunk should come BEFORE the usage chunk.
	stopIdx := strings.Index(out, `"finish_reason":"stop"`)
	usageIdx := strings.Index(out, `"usage"`)
	if stopIdx == -1 || usageIdx == -1 || stopIdx > usageIdx {
		t.Fatalf("stop chunk should precede usage chunk, stopIdx=%d usageIdx=%d\n%s", stopIdx, usageIdx, out)
	}
}
