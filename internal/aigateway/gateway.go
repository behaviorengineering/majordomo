package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

// Gateway is an embedded Bifrost client plus an OpenAI-compatible loopback.
type Gateway struct {
	account *Account
	client  *bifrost.Bifrost
	server  *http.Server
	ln      net.Listener
	baseURL string // e.g. http://127.0.0.1:port/v1

	mu      sync.Mutex
	started bool
}

var (
	globalMu sync.Mutex
	globalGW *Gateway
)

// Ensure starts the process-wide gateway (once) and returns it.
func Ensure() (*Gateway, error) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalGW != nil && globalGW.started {
		return globalGW, nil
	}
	account, err := NewAccountFromEnv()
	if err != nil {
		return nil, err
	}
	gw, err := Start(context.Background(), account)
	if err != nil {
		return nil, err
	}
	globalGW = gw
	return gw, nil
}

// ShutdownGlobal stops the process-wide gateway if running.
func ShutdownGlobal() {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalGW != nil {
		globalGW.Shutdown()
		globalGW = nil
	}
}

// ResetForTests clears the process-wide gateway (tests only).
func ResetForTests() {
	ShutdownGlobal()
}

// Start initializes Bifrost and a loopback OpenAI HTTP server.
func Start(ctx context.Context, account *Account) (*Gateway, error) {
	if account == nil || !account.HasProviders() {
		return nil, fmt.Errorf("aigateway: account required")
	}
	client, err := bifrost.Init(ctx, schemas.BifrostConfig{
		Account:         account,
		InitialPoolSize: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("aigateway: bifrost init: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Shutdown()
		return nil, fmt.Errorf("aigateway: listen: %w", err)
	}
	gw := &Gateway{
		account: account,
		client:  client,
		ln:      ln,
		baseURL: fmt.Sprintf("http://%s/v1", ln.Addr().String()),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", gw.handleChatCompletions)
	mux.HandleFunc("/openai/v1/chat/completions", gw.handleChatCompletions)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	gw.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = gw.server.Serve(ln) }()
	gw.started = true
	return gw, nil
}

// BaseURL is the OpenAI-compatible base (…/v1) for DSPy/strop.
func (g *Gateway) BaseURL() string {
	if g == nil {
		return ""
	}
	return g.baseURL
}

// Origin is scheme://host without path (for OpenCode baseURL variants).
func (g *Gateway) Origin() string {
	if g == nil {
		return ""
	}
	return strings.TrimSuffix(g.baseURL, "/v1")
}

// Shutdown stops the loopback server and Bifrost.
func (g *Gateway) Shutdown() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.started {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if g.server != nil {
		_ = g.server.Shutdown(ctx)
	}
	if g.client != nil {
		g.client.Shutdown()
	}
	g.started = false
}

// ChildEnv returns env pairs so an OpenCode child uses the loopback only.
// Real provider keys are stripped.
func (g *Gateway) ChildEnv(parent []string) []string {
	if g == nil {
		return parent
	}
	strip := map[string]struct{}{
		envAnthropic: {},
		envOpenAI:    {},
		envGemini:    {},
		envGoogle:    {},
		envGoogleAI:  {},
	}
	out := make([]string, 0, len(parent)+4)
	for _, e := range parent {
		key, _, _ := strings.Cut(e, "=")
		if _, bad := strip[key]; bad {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"OPENAI_API_KEY="+DummyAPIKey,
		"OPENAI_BASE_URL="+g.baseURL,
		"OPENCODE_PROVIDER_API_KEY="+DummyAPIKey,
	)
	return out
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (g *Gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	var req openAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Stream {
		writeOpenAIError(w, http.StatusBadRequest, "streaming not supported on embedded gateway")
		return
	}

	primary, fallbacks := resolveRoute(g.account, req.Model)
	if primary.Provider == "" {
		writeOpenAIError(w, http.StatusBadRequest, "no providers configured")
		return
	}

	messages, err := toBifrostMessages(req.Messages)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, err.Error())
		return
	}

	bfReq := &schemas.BifrostChatRequest{
		Provider:  primary.Provider,
		Model:     primary.Model,
		Input:     messages,
		Fallbacks: fallbacks,
	}
	if req.Temperature != nil || req.MaxTokens != nil {
		bfReq.Params = &schemas.ChatParameters{}
		if req.Temperature != nil {
			bfReq.Params.Temperature = req.Temperature
		}
		if req.MaxTokens != nil {
			bfReq.Params.MaxCompletionTokens = req.MaxTokens
		}
	}

	bfCtx := schemas.NewBifrostContext(r.Context(), schemas.NoDeadline)
	resp, bfErr := g.client.ChatCompletionRequest(bfCtx, bfReq)
	if bfErr != nil {
		msg := "provider error"
		status := http.StatusBadGateway
		if bfErr.Error != nil && bfErr.Error.Message != "" {
			msg = bfErr.Error.Message
		}
		if bfErr.StatusCode != nil {
			status = *bfErr.StatusCode
		}
		writeOpenAIError(w, status, msg)
		return
	}
	writeOpenAIChatResponse(w, resp)
}

func toBifrostMessages(in []openAIMessage) ([]schemas.ChatMessage, error) {
	out := make([]schemas.ChatMessage, 0, len(in))
	for _, m := range in {
		role := schemas.ChatMessageRole(strings.ToLower(strings.TrimSpace(m.Role)))
		if role == "" {
			role = schemas.ChatMessageRoleUser
		}
		text, err := messageContentString(m.Content)
		if err != nil {
			return nil, err
		}
		out = append(out, schemas.ChatMessage{
			Role: role,
			Content: &schemas.ChatMessageContent{
				ContentStr: schemas.Ptr(text),
			},
		})
	}
	return out, nil
}

func messageContentString(content any) (string, error) {
	switch v := content.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []any:
		var b strings.Builder
		for _, part := range v {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := m["type"].(string); t == "text" {
				if s, _ := m["text"].(string); s != "" {
					b.WriteString(s)
				}
			}
		}
		return b.String(), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("unsupported message content")
		}
		return string(raw), nil
	}
}

func writeOpenAIChatResponse(w http.ResponseWriter, resp *schemas.BifrostChatResponse) {
	type choiceMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type choice struct {
		Index        int       `json:"index"`
		Message      choiceMsg `json:"message"`
		FinishReason string    `json:"finish_reason"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type out struct {
		ID      string   `json:"id"`
		Object  string   `json:"object"`
		Created int      `json:"created"`
		Model   string   `json:"model"`
		Choices []choice `json:"choices"`
		Usage   *usage   `json:"usage,omitempty"`
	}

	o := out{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.Created,
		Model:   resp.Model,
	}
	if o.ID == "" {
		o.ID = "chatcmpl-majordomo"
	}
	if o.Created == 0 {
		o.Created = int(time.Now().Unix())
	}
	for i, c := range resp.Choices {
		msg := choiceMsg{Role: "assistant"}
		finish := "stop"
		if c.FinishReason != nil {
			finish = string(*c.FinishReason)
		}
		if c.ChatNonStreamResponseChoice != nil && c.ChatNonStreamResponseChoice.Message != nil {
			m := c.ChatNonStreamResponseChoice.Message
			if m.Role != "" {
				msg.Role = string(m.Role)
			}
			if m.Content != nil && m.Content.ContentStr != nil {
				msg.Content = *m.Content.ContentStr
			}
		}
		o.Choices = append(o.Choices, choice{
			Index:        i,
			Message:      msg,
			FinishReason: finish,
		})
	}
	if resp.Usage != nil {
		o.Usage = &usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(o)
}

func writeOpenAIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "aigateway_error",
		},
	})
}
