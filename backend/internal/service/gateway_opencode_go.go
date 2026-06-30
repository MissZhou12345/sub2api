package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func buildOpenCodeGoAnthropicMessagesURL(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	if trimmed == "" {
		return openCodeGoDefaultAnthropicMessagesURL
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(lower, "/v1/messages"):
		return trimmed
	case strings.HasSuffix(lower, "/messages"):
		return trimmed
	case strings.HasSuffix(lower, "/chat/completions"):
		return trimmed[:len(trimmed)-len("/chat/completions")] + "/messages"
	default:
		return buildOpenAIEndpointURL(trimmed, "/v1/messages")
	}
}

func stringSliceFromRaw(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func (s *GatewayService) forwardOpenCodeGoMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
	startTime time.Time,
) (*ForwardResult, error) {
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(parsed.Body.Bytes(), &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse opencode_go anthropic request: %w", err)
	}
	originalModel := anthropicReq.Model
	upstreamModel := account.GetMappedModel(originalModel)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = originalModel
	}
	anthropicReq.Model = upstreamModel

	logger.L().Debug("opencode_go messages: model mapping applied",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", anthropicReq.Stream),
		zap.Bool("anthropic_native", account.IsOpenCodeGoAnthropicNativeModel(upstreamModel)),
	)

	if account.IsOpenCodeGoAnthropicNativeModel(upstreamModel) {
		return s.forwardOpenCodeGoAnthropicNative(ctx, c, account, &anthropicReq, originalModel, upstreamModel, startTime)
	}
	return s.forwardOpenCodeGoChatCompletions(ctx, c, account, &anthropicReq, originalModel, upstreamModel, startTime)
}

func (s *GatewayService) forwardOpenCodeGoAnthropicNative(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicReq *apicompat.AnthropicRequest,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	upstreamBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal opencode_go anthropic request: %w", err)
	}

	apiKey := account.GetOpenCodeGoAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenCodeGoAnthropicBaseURL()
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid opencode_go anthropic_base_url: %w", err)
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, buildOpenCodeGoAnthropicMessagesURL(validatedURL), bytes.NewReader(upstreamBody))
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build opencode_go upstream request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("x-api-key", apiKey)
	if anthropicReq.Stream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	resp, err := s.doOpenCodeGoRequest(ctx, c, account, upstreamReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleOpenCodeGoHTTPError(c, account, resp)
	}
	if anthropicReq.Stream {
		return s.streamOpenCodeGoAnthropicNative(c, resp, originalModel, upstreamModel, startTime)
	}
	return s.bufferOpenCodeGoAnthropicNative(c, resp, originalModel, upstreamModel, startTime)
}

func (s *GatewayService) forwardOpenCodeGoChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicReq *apicompat.AnthropicRequest,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	responsesReq, err := apicompat.AnthropicToResponses(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert opencode_go anthropic to responses: %w", err)
	}
	responsesReq.Model = upstreamModel
	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("convert opencode_go responses to chat completions: %w", err)
	}
	chatReq.Model = upstreamModel
	chatReq.Stream = anthropicReq.Stream

	upstreamBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal opencode_go chat request: %w", err)
	}
	if chatReq.Stream {
		upstreamBody, err = ensureOpenAIChatStreamUsage(upstreamBody)
		if err != nil {
			return nil, fmt.Errorf("enable opencode_go stream usage: %w", err)
		}
	}

	apiKey := account.GetOpenCodeGoAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenCodeGoBaseURL()
	if baseURL == "" {
		baseURL = openCodeGoDefaultChatCompletionsURL
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid opencode_go base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build opencode_go upstream request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	if chatReq.Stream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	resp, err := s.doOpenCodeGoRequest(ctx, c, account, upstreamReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.handleOpenCodeGoHTTPError(c, account, resp)
	}
	if chatReq.Stream {
		return s.streamOpenCodeGoChatAsAnthropic(c, resp, originalModel, upstreamModel, startTime)
	}
	return s.bufferOpenCodeGoChatAsAnthropic(c, resp, originalModel, upstreamModel, startTime)
}

func (s *GatewayService) doOpenCodeGoRequest(ctx context.Context, c *gin.Context, account *Account, req *http.Request) (*http.Response, error) {
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, fmt.Errorf("opencode_go upstream request failed: %s", safeErr)
	}
	if resp == nil || resp.Body == nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, errors.New("opencode_go upstream returned empty response")
	}
	return resp, nil
}

func (s *GatewayService) handleOpenCodeGoHTTPError(c *gin.Context, account *Account, resp *http.Response) (*ForwardResult, error) {
	respBody, _ := s.readUpstreamErrorBody(resp)
	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	if upstreamMsg == "" {
		upstreamMsg = http.StatusText(resp.StatusCode)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "failover",
		Message:            upstreamMsg,
	})
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: respBody, ResponseHeaders: resp.Header.Clone()}
	}
	writeAnthropicError(c, mapUpstreamStatusCode(resp.StatusCode), "api_error", upstreamMsg)
	return nil, fmt.Errorf("opencode_go upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func (s *GatewayService) bufferOpenCodeGoChatAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeAnthropicError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read opencode_go upstream body: %w", err)
	}

	var chatResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Failed to parse upstream response")
		return nil, fmt.Errorf("parse opencode_go chat response: %w", err)
	}
	responsesResp := apicompat.ChatCompletionsResponseToResponses(&chatResp, upstreamModel)
	anthropicResp := apicompat.ResponsesToAnthropic(responsesResp, originalModel)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, anthropicResp)

	return &ForwardResult{
		RequestID:     requestID,
		Usage:         claudeUsageFromOpenAIChatUsage(chatResp.Usage),
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *GatewayService) streamOpenCodeGoChatAsAnthropic(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	chatState := apicompat.NewChatCompletionsToResponsesStreamState(upstreamModel)
	anthropicState := apicompat.NewResponsesEventToAnthropicState()
	anthropicState.Model = originalModel
	var usage ClaudeUsage
	var firstTokenMs *int

	writeAnthropicEvents := func(events []apicompat.ResponsesStreamEvent) error {
		for _, resEvt := range events {
			if resEvt.Usage != nil {
				usage = claudeUsageFromOpenAIUsage(copyOpenAIUsageFromResponsesUsage(resEvt.Usage))
			}
			if resEvt.Response != nil && resEvt.Response.Usage != nil {
				usage = claudeUsageFromOpenAIUsage(copyOpenAIUsageFromResponsesUsage(resEvt.Response.Usage))
			}
			anthEvents := apicompat.ResponsesEventToAnthropicEvents(&resEvt, anthropicState)
			for _, anthEvt := range anthEvents {
				sse, err := apicompat.ResponsesAnthropicEventToSSE(anthEvt)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprint(c.Writer, sse); err != nil {
					return err
				}
			}
			if len(anthEvents) > 0 {
				c.Writer.Flush()
			}
		}
		return nil
	}

	for scanner.Scan() {
		payload, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.L().Warn("opencode_go chat stream: failed to parse chunk",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			continue
		}
		if firstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) {
			elapsed := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &elapsed
		}
		if chunk.Usage != nil {
			usage = claudeUsageFromOpenAIChatUsage(chunk.Usage)
		}
		if err := writeAnthropicEvents(apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, chatState)); err != nil {
			break
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("read opencode_go chat stream: %w", err)
	}
	if err := writeAnthropicEvents(apicompat.FinalizeChatCompletionsResponsesStream(chatState)); err != nil {
		return nil, err
	}
	for _, anthEvt := range apicompat.FinalizeResponsesAnthropicStream(anthropicState) {
		sse, err := apicompat.ResponsesAnthropicEventToSSE(anthEvt)
		if err != nil {
			return nil, err
		}
		if _, err := fmt.Fprint(c.Writer, sse); err != nil {
			break
		}
	}
	c.Writer.Flush()

	return &ForwardResult{
		RequestID:     requestID,
		Usage:         usage,
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        true,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

func (s *GatewayService) bufferOpenCodeGoAnthropicNative(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeAnthropicError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read opencode_go anthropic body: %w", err)
	}
	var anthropicResp apicompat.AnthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err == nil {
		anthropicResp.Model = originalModel
		respBody, _ = json.Marshal(anthropicResp)
	}
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", respBody)

	return &ForwardResult{
		RequestID:     requestID,
		Usage:         claudeUsageFromAnthropicUsage(anthropicResp.Usage),
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *GatewayService) streamOpenCodeGoAnthropicNative(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var eventName string
	var usage ClaudeUsage
	var firstTokenMs *int
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			if _, err := fmt.Fprintln(c.Writer, line); err != nil {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var event apicompat.AnthropicStreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err == nil {
				if event.Type == "message_start" && event.Message != nil {
					event.Message.Model = originalModel
					usage = claudeUsageFromAnthropicUsage(event.Message.Usage)
				}
				if event.Type == "message_delta" && event.Usage != nil {
					usage = claudeUsageFromAnthropicUsage(*event.Usage)
				}
				if firstTokenMs == nil && event.Type == "content_block_delta" {
					elapsed := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &elapsed
				}
				if event.Type != "" {
					payloadBytes, _ := json.Marshal(event)
					payload = string(payloadBytes)
				}
			}
			if _, err := fmt.Fprintln(c.Writer, "data: "+payload); err != nil {
				break
			}
			continue
		}
		if _, err := fmt.Fprintln(c.Writer, line); err != nil {
			break
		}
		if line == "" {
			c.Writer.Flush()
			eventName = ""
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("read opencode_go anthropic stream: %w", err)
	}
	if eventName != "" {
		c.Writer.Flush()
	}
	return &ForwardResult{
		RequestID:     requestID,
		Usage:         usage,
		Model:         originalModel,
		UpstreamModel: upstreamModel,
		Stream:        true,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

func claudeUsageFromAnthropicUsage(usage apicompat.AnthropicUsage) ClaudeUsage {
	return ClaudeUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
}

func claudeUsageFromOpenAIChatUsage(usage *apicompat.ChatUsage) ClaudeUsage {
	if usage == nil {
		return ClaudeUsage{}
	}
	out := ClaudeUsage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
	if usage.PromptTokensDetails != nil {
		out.CacheReadInputTokens = usage.PromptTokensDetails.CachedTokens
	}
	return out
}

func claudeUsageFromOpenAIUsage(usage OpenAIUsage) ClaudeUsage {
	return ClaudeUsage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
}
