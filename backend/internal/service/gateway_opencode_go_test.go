package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenCodeGoAnthropicMessagesURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "empty", base: "", want: "https://opencode.ai/zen/go/v1/messages"},
		{name: "chat completions", base: "https://opencode.ai/zen/go/v1/chat/completions", want: "https://opencode.ai/zen/go/v1/messages"},
		{name: "v1", base: "https://opencode.ai/zen/go/v1", want: "https://opencode.ai/zen/go/v1/messages"},
		{name: "already messages", base: "https://opencode.ai/zen/go/v1/messages", want: "https://opencode.ai/zen/go/v1/messages"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, buildOpenCodeGoAnthropicMessagesURL(tc.base))
		})
	}
}

func TestGatewayServiceForwardOpenCodeGoChatCompletions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":32,"stream":false,"messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_opencode_chat"}},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"id":"chatcmpl_test",
			"object":"chat.completion",
			"model":"glm-5.2",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello from opencode"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}
		}`))),
	}}

	svc := &GatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID:          101,
		Name:        "opencode-go",
		Platform:    PlatformOpenCodeGo,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "oc-test",
			"base_url": "https://opencode.ai/zen/go/v1",
			"model_mapping": map[string]any{
				"claude-sonnet-4-6": "glm-5.2",
			},
		},
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "claude-sonnet-4-6", result.Model)
	require.Equal(t, "glm-5.2", result.UpstreamModel)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)

	require.Equal(t, "/zen/go/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer oc-test", upstream.lastReq.Header.Get("Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "glm-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "claude-sonnet-4-6", gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, "hello from opencode", gjson.Get(rec.Body.String(), "content.0.text").String())
}

func TestOpenAIExtraOpenCodeGoNoLongerActivatesOpenCodeGo(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"opencode_go": true},
	}

	require.False(t, account.IsOpenCodeGo())
	require.False(t, account.IsOpenCodeGoAPIKey())
}
