# gateway Specification

## Purpose
TBD - created by archiving change add-opencode-go-proxy. Update Purpose after archive.
## Requirements
### Requirement: OpenCode Go Claude Code Proxy
The gateway SHALL allow Claude Code CLI Anthropic Messages requests to be served by an OpenCode Go API-key account when the account enables OpenCode Go compatibility mode.

#### Scenario: Chat Completions OpenCode Go model
- **WHEN** a Claude Code client sends `POST /v1/messages` through an OpenAI group that selects an OpenCode Go account for a non-Anthropic-native OpenCode Go model
- **THEN** the gateway converts the request to OpenAI Chat Completions, forwards it to the account's OpenCode Go Chat Completions endpoint, and returns an Anthropic Messages response to the client

#### Scenario: Anthropic-native OpenCode Go model
- **WHEN** a Claude Code client sends `POST /v1/messages` through an OpenAI group that selects an OpenCode Go account for an Anthropic-native OpenCode Go model
- **THEN** the gateway forwards an Anthropic Messages request to the account's OpenCode Go Anthropic Messages endpoint and preserves Anthropic Messages response semantics

#### Scenario: Streaming
- **WHEN** the Claude Code request sets `stream` to `true`
- **THEN** the gateway streams Anthropic-compatible SSE events to the client while extracting usage for normal sub2api accounting when the upstream supplies usage

