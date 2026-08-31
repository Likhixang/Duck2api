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

// TestReadImageResponse_PrioritizeTool verifies that when both a legacy parts
// image and a GenerateImage tool image are present, only the tool image is returned.
func TestReadImageResponse_PrioritizeTool(t *testing.T) {
	partsImage := `{"action":"success","model":"gpt-5.4-nano","role":"assistant","message":"Here is your image","parts":[{"type":"generated-image","result":"AAAA"}]}`
	toolImage := `{"action":"success","model":"gpt-5.4-nano","role":"assistant","toolName":"GenerateImage","data":{"b64Image":"BBBB","format":"png"}}`

	sse := "data: " + partsImage + "\n" +
		"data: " + toolImage + "\n" +
		"data: [DONE]\n"

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       fakeCloser{Buffer: bytes.NewBufferString(sse)},
	}

	result := ReadImageResponse(resp)

	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image (tool), got %d", len(result.Images))
	}
	if result.Images[0].Result != "BBBB" {
		t.Fatalf("expected tool image BBBB, got %q", result.Images[0].Result)
	}
	if result.Text != "Here is your image" {
		t.Fatalf("expected text 'Here is your image', got %q", result.Text)
	}
}

// TestReadImageResponse_FallbackToParts verifies that when no tool image is present,
// legacy parts images are still returned as fallback.
func TestReadImageResponse_FallbackToParts(t *testing.T) {
	partsImage := `{"action":"success","model":"gpt-5.4-nano","role":"assistant","parts":[{"type":"generated-image","result":"CCCC"}]}`

	sse := "data: " + partsImage + "\n" +
		"data: [DONE]\n"

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       fakeCloser{Buffer: bytes.NewBufferString(sse)},
	}

	result := ReadImageResponse(resp)

	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image (fallback), got %d", len(result.Images))
	}
	if result.Images[0].Result != "CCCC" {
		t.Fatalf("expected fallback image CCCC, got %q", result.Images[0].Result)
	}
}

// TestReadImageResponse_SkipPartial verifies that partial-status tool images
// (progressive previews) are skipped, keeping only the final success image.
func TestReadImageResponse_SkipPartial(t *testing.T) {
	partial := `{"action":"success","model":"gpt-5.4-nano","toolName":"GenerateImage","data":{"b64Image":"PPPP","format":"jpeg","status":"partial"}}`
	final := `{"action":"success","model":"gpt-5.4-nano","toolName":"GenerateImage","data":{"b64Image":"FFFF","format":"jpeg","status":"success"}}`

	sse := "data: " + partial + "\n" +
		"data: " + final + "\n" +
		"data: [DONE]\n"

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       fakeCloser{Buffer: bytes.NewBufferString(sse)},
	}

	result := ReadImageResponse(resp)

	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image (final success), got %d", len(result.Images))
	}
	if result.Images[0].Result != "FFFF" {
		t.Fatalf("expected final image FFFF, got %q", result.Images[0].Result)
	}
}

// TestReadImageResponse_MultipleToolImages verifies that multi-tool-image SSE
// returns all tool images without dropping any.
func TestReadImageResponse_MultipleToolImages(t *testing.T) {
	tool1 := `{"action":"success","model":"gpt-5.4-nano","toolName":"GenerateImage","data":{"b64Image":"DDDD","format":"png"}}`
	tool2 := `{"action":"success","model":"gpt-5.4-nano","toolName":"GenerateImage","data":{"b64Image":"EEEE","format":"jpg"}}`

	sse := "data: " + tool1 + "\n" +
		"data: " + tool2 + "\n" +
		"data: [DONE]\n"

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       fakeCloser{Buffer: bytes.NewBufferString(sse)},
	}

	result := ReadImageResponse(resp)

	if len(result.Images) != 2 {
		t.Fatalf("expected 2 tool images, got %d", len(result.Images))
	}
	if result.Images[0].Result != "DDDD" || result.Images[1].Result != "EEEE" {
		t.Fatalf("unexpected images: %+v", result.Images)
	}
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
