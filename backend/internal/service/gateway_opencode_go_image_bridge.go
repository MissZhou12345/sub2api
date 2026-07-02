package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const openCodeGoImageBridgeDefaultPrompt = "You are an image OCR and description bridge for a coding assistant. Read every image carefully. Return concise plain text in Chinese when the user's text is Chinese; otherwise use the user's language. Include OCR text, visible UI/code details, diagrams, tables, errors, and any details needed for the final text-only model. Do not answer the user's task directly; only describe the image content."

type openCodeGoImageBridgeResult struct {
	Request apicompat.AnthropicRequest
	Usage   ClaudeUsage
	Applied bool
}

type openCodeGoImageRef struct {
	Index     int
	MediaType string
	DataURI   string
}

func (s *GatewayService) applyOpenCodeGoImageTextBridge(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	req *apicompat.AnthropicRequest,
	upstreamModel string,
) (*openCodeGoImageBridgeResult, error) {
	result := &openCodeGoImageBridgeResult{Request: *req}
	if req == nil || !account.IsOpenCodeGoImageTextBridgeTargetModel(upstreamModel) {
		return result, nil
	}

	images := collectOpenCodeGoAnthropicImages(req.Messages)
	if len(images) == 0 {
		return result, nil
	}
	if !account.IsOpenCodeGoImageTextBridgeEnabled() {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "Image input is not supported by the selected OpenCode Go model. Enable image_text_bridge_enabled to convert images to text before forwarding.")
		return nil, fmt.Errorf("opencode_go image bridge disabled for model %s", upstreamModel)
	}
	maxImages := account.GetOpenCodeGoImageTextBridgeMaxImages()
	if len(images) > maxImages {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Too many images for OpenCode Go image bridge: got %d, max %d", len(images), maxImages))
		return nil, fmt.Errorf("opencode_go image bridge image limit exceeded: got %d max %d", len(images), maxImages)
	}

	visionModel := account.GetOpenCodeGoImageTextBridgeModel()
	if strings.TrimSpace(visionModel) == "" {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "OpenCode Go image bridge requires image_text_bridge_model")
		return nil, errors.New("opencode_go image bridge missing model")
	}

	description, usage, err := s.callOpenCodeGoImageTextBridge(ctx, c, account, req, images, visionModel)
	if err != nil {
		if c.Writer.Written() {
			return nil, err
		}
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "OpenCode Go image bridge failed")
		return nil, err
	}

	transformed, err := replaceOpenCodeGoImagesWithText(req, description)
	if err != nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Failed to build text-only image bridge request")
		return nil, err
	}
	result.Request = transformed
	result.Usage = usage
	result.Applied = true
	return result, nil
}

func (s *GatewayService) callOpenCodeGoImageTextBridge(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	req *apicompat.AnthropicRequest,
	images []openCodeGoImageRef,
	visionModel string,
) (string, ClaudeUsage, error) {
	userText := latestOpenCodeGoUserText(req.Messages)
	parts := make([]apicompat.ChatContentPart, 0, len(images)+2)
	prompt := account.GetCredential("image_text_bridge_prompt")
	if strings.TrimSpace(prompt) == "" {
		prompt = openCodeGoImageBridgeDefaultPrompt
	}
	textPrompt := prompt
	if strings.TrimSpace(userText) != "" {
		textPrompt += "\n\nUser request text:\n" + userText
	}
	parts = append(parts, apicompat.ChatContentPart{Type: "text", Text: textPrompt})
	for _, img := range images {
		parts = append(parts, apicompat.ChatContentPart{
			Type:     "image_url",
			ImageURL: &apicompat.ChatImageURL{URL: img.DataURI},
		})
	}
	content, err := json.Marshal(parts)
	if err != nil {
		return "", ClaudeUsage{}, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 || maxTokens > 2048 {
		maxTokens = 2048
	}
	chatReq := apicompat.ChatCompletionsRequest{
		Model: visionModel,
		Messages: []apicompat.ChatMessage{{
			Role:    "user",
			Content: content,
		}},
		MaxCompletionTokens: &maxTokens,
		Stream:              false,
	}
	upstreamBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", ClaudeUsage{}, err
	}

	apiKey := account.GetOpenCodeGoAPIKey()
	if apiKey == "" {
		return "", ClaudeUsage{}, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenCodeGoBaseURL()
	if baseURL == "" {
		baseURL = openCodeGoDefaultChatCompletionsURL
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", ClaudeUsage{}, fmt.Errorf("invalid opencode_go image bridge base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	releaseUpstreamCtx()
	if err != nil {
		return "", ClaudeUsage{}, fmt.Errorf("build opencode_go image bridge request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Accept", "application/json")

	resp, err := s.doOpenCodeGoRequest(ctx, c, account, upstreamReq)
	if err != nil {
		return "", ClaudeUsage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", ClaudeUsage{}, s.openCodeGoImageBridgeHTTPError(c, account, resp)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return "", ClaudeUsage{}, fmt.Errorf("read opencode_go image bridge response: %w", err)
	}
	var chatResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", ClaudeUsage{}, fmt.Errorf("parse opencode_go image bridge response: %w", err)
	}
	text := strings.TrimSpace(openCodeGoChatResponseText(chatResp))
	if text == "" {
		return "", claudeUsageFromOpenAIChatUsage(chatResp.Usage), errors.New("opencode_go image bridge returned empty text")
	}
	logger.L().Debug("opencode_go image bridge: preflight completed",
		zap.Int64("account_id", account.ID),
		zap.String("vision_model", visionModel),
		zap.Int("image_count", len(images)),
	)
	return text, claudeUsageFromOpenAIChatUsage(chatResp.Usage), nil
}

func (s *GatewayService) openCodeGoImageBridgeHTTPError(c *gin.Context, account *Account, resp *http.Response) error {
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
	return fmt.Errorf("opencode_go image bridge upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func collectOpenCodeGoAnthropicImages(messages []apicompat.AnthropicMessage) []openCodeGoImageRef {
	var images []openCodeGoImageRef
	for _, msg := range messages {
		var blocks []apicompat.AnthropicContentBlock
		if err := json.Unmarshal(msg.Content, &blocks); err != nil {
			continue
		}
		collectOpenCodeGoImageBlocks(blocks, &images)
	}
	return images
}

func collectOpenCodeGoImageBlocks(blocks []apicompat.AnthropicContentBlock, images *[]openCodeGoImageRef) {
	for _, block := range blocks {
		switch block.Type {
		case "image":
			if uri := openCodeGoAnthropicImageToDataURI(block.Source); uri != "" {
				*images = append(*images, openCodeGoImageRef{
					Index:     len(*images) + 1,
					MediaType: block.Source.MediaType,
					DataURI:   uri,
				})
			}
		case "tool_result":
			var inner []apicompat.AnthropicContentBlock
			if err := json.Unmarshal(block.Content, &inner); err == nil {
				collectOpenCodeGoImageBlocks(inner, images)
			}
		}
	}
}

func replaceOpenCodeGoImagesWithText(req *apicompat.AnthropicRequest, imageText string) (apicompat.AnthropicRequest, error) {
	out := *req
	out.Messages = make([]apicompat.AnthropicMessage, len(req.Messages))
	injected := false
	for i, msg := range req.Messages {
		outMsg := msg
		blocks, ok := parseOpenCodeGoAnthropicBlocks(msg.Content)
		if !ok {
			out.Messages[i] = outMsg
			continue
		}
		rewritten, changed := rewriteOpenCodeGoImageBlocks(blocks, imageText, &injected)
		if changed {
			content, err := json.Marshal(rewritten)
			if err != nil {
				return apicompat.AnthropicRequest{}, err
			}
			outMsg.Content = content
		}
		out.Messages[i] = outMsg
	}
	if !injected && strings.TrimSpace(imageText) != "" {
		content, err := json.Marshal([]apicompat.AnthropicContentBlock{{
			Type: "text",
			Text: openCodeGoImageBridgeTextBlock(imageText),
		}})
		if err != nil {
			return apicompat.AnthropicRequest{}, err
		}
		out.Messages = append(out.Messages, apicompat.AnthropicMessage{Role: "user", Content: content})
	}
	return out, nil
}

func rewriteOpenCodeGoImageBlocks(blocks []apicompat.AnthropicContentBlock, imageText string, injected *bool) ([]apicompat.AnthropicContentBlock, bool) {
	out := make([]apicompat.AnthropicContentBlock, 0, len(blocks))
	changed := false
	for _, block := range blocks {
		switch block.Type {
		case "image":
			changed = true
			if !*injected && strings.TrimSpace(imageText) != "" {
				out = append(out, apicompat.AnthropicContentBlock{
					Type: "text",
					Text: openCodeGoImageBridgeTextBlock(imageText),
				})
				*injected = true
			}
		case "tool_result":
			var inner []apicompat.AnthropicContentBlock
			if err := json.Unmarshal(block.Content, &inner); err == nil {
				rewrittenInner, innerChanged := rewriteOpenCodeGoImageBlocks(inner, imageText, injected)
				if innerChanged {
					innerContent, _ := json.Marshal(rewrittenInner)
					block.Content = innerContent
					changed = true
				}
			}
			out = append(out, block)
		default:
			out = append(out, block)
		}
	}
	return out, changed
}

func parseOpenCodeGoAnthropicBlocks(raw json.RawMessage) ([]apicompat.AnthropicContentBlock, bool) {
	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks, true
	}
	return nil, false
}

func openCodeGoAnthropicImageToDataURI(src *apicompat.AnthropicImageSource) string {
	if src == nil || strings.TrimSpace(src.Data) == "" {
		return ""
	}
	mediaType := strings.TrimSpace(src.MediaType)
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + src.Data
}

func openCodeGoImageBridgeTextBlock(imageText string) string {
	return "[Image context extracted by vision bridge]\n" + strings.TrimSpace(imageText)
}

func latestOpenCodeGoUserText(messages []apicompat.AnthropicMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		if text := openCodeGoMessageText(messages[i].Content); text != "" {
			return text
		}
	}
	return ""
}

func openCodeGoMessageText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n\n")
}

func openCodeGoChatResponseText(resp apicompat.ChatCompletionsResponse) string {
	var parts []string
	for _, choice := range resp.Choices {
		if text := openCodeGoChatMessageContentText(choice.Message.Content); text != "" {
			parts = append(parts, text)
		}
		if choice.Message.ReasoningContent != "" && len(parts) == 0 {
			parts = append(parts, choice.Message.ReasoningContent)
		}
	}
	return strings.Join(parts, "\n\n")
}

func openCodeGoChatMessageContentText(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var parts []apicompat.ChatContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var texts []string
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			texts = append(texts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(texts, "\n\n")
}

func mergeOpenCodeGoUsage(dst *ClaudeUsage, src ClaudeUsage) {
	if dst == nil {
		return
	}
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CacheCreationInputTokens += src.CacheCreationInputTokens
	dst.CacheReadInputTokens += src.CacheReadInputTokens
	dst.CacheCreation5mTokens += src.CacheCreation5mTokens
	dst.CacheCreation1hTokens += src.CacheCreation1hTokens
	dst.ImageOutputTokens += src.ImageOutputTokens
	dst.KiroCredits += src.KiroCredits
}
