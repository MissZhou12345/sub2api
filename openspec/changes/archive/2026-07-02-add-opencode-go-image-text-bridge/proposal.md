# Change: Add OpenCode Go Image-to-Text Bridge

## Why
Claude Code CLI can send image content blocks, and sub2api already preserves those blocks through its Anthropic-to-Chat conversion path. OpenCode Go's default `glm-5.2` mapping rejects image input, so image requests currently fail before `glm-5.2` can answer.

## What Changes
- Add an optional OpenCode Go image-to-text bridge for `glm-5.2` requests that contain image content.
- When enabled, the gateway first sends image blocks to a configured vision-capable OpenCode Go model such as `kimi-k2.7-code` or `kimi-k2.6`.
- The gateway replaces image blocks with textual image descriptions/OCR output, then forwards the resulting pure-text conversation to the original `glm-5.2` path.
- Add account/group-level configuration for enabling the bridge, choosing the vision model, and limiting images per request.
- Return a clear Anthropic-compatible error when image input is present but the bridge is disabled or the vision preflight fails.

## Non-Goals
- The gateway will not call a Claude Code client's local MCP server directly. Local MCP servers live on the client side and are not reachable from sub2api unless separately exposed as a server-side service.
- The bridge will not make `glm-5.2` natively multimodal. It converts images into text before calling `glm-5.2`.

## Impact
- Affected specs: `gateway`, `accounts`
- Affected code: `backend/internal/service/gateway_opencode_go.go`, `backend/internal/pkg/apicompat`, OpenCode Go account credential parsing, tests
- Operational impact: image content may be sent to the configured vision model before the final `glm-5.2` request; this has privacy, latency, and cost implications.
