## MODIFIED Requirements
### Requirement: OpenCode Go Claude Code Proxy
The gateway SHALL allow Claude Code CLI Anthropic Messages requests to be served by an OpenCode Go API-key account selected from an `opencode_go` group.

#### Scenario: GLM-5.2 through Chat Completions
- **WHEN** a Claude Code client sends `POST /v1/messages` through an `opencode_go` group selecting an OpenCode Go account whose mapped upstream model is `glm-5.2`
- **THEN** the gateway converts the request to OpenAI Chat Completions, forwards it to the account's OpenCode Go Chat Completions endpoint, and returns an Anthropic Messages response to the client

#### Scenario: Anthropic-native OpenCode Go model
- **WHEN** a Claude Code client sends `POST /v1/messages` through an `opencode_go` group selecting an OpenCode Go account for an Anthropic-native OpenCode Go model
- **THEN** the gateway forwards an Anthropic Messages request to the account's OpenCode Go Anthropic Messages endpoint and preserves Anthropic Messages response semantics

#### Scenario: Streaming
- **WHEN** the Claude Code request sets `stream` to `true`
- **THEN** the gateway streams Anthropic-compatible SSE events to the client while extracting usage for normal sub2api accounting when the upstream supplies usage

#### Scenario: Token counting
- **WHEN** a Claude Code client sends `POST /v1/messages/count_tokens` through an `opencode_go` group
- **THEN** the gateway returns a local not-found error because OpenCode Go does not provide an Anthropic-compatible token counting endpoint
