# Change: Add OpenCode Go proxy support

## Why
Users with an OpenCode Go subscription need to use Claude Code CLI through sub2api. OpenCode Go exposes OpenAI Chat Completions and Anthropic Messages style endpoints, while sub2api's existing OpenAI `/v1/messages` compatibility path assumes an upstream Responses API.

## What Changes
- Add an OpenCode Go compatibility mode for OpenAI API-key accounts.
- Allow Claude Code `/v1/messages` requests to be forwarded to OpenCode Go using either the OpenCode Go Anthropic Messages endpoint or a Chat Completions conversion path.
- Keep the existing group, account scheduling, failover, and usage recording flow.

## Impact
- Affected specs: gateway, accounts
- Affected code: `backend/internal/service/account.go`, `backend/internal/service/openai_gateway_messages.go`, OpenAI gateway tests
