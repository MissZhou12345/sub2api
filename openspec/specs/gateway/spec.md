# gateway Specification

## Purpose
Defines gateway behavior for serving Claude Code Anthropic Messages requests through OpenCode Go accounts, including GLM-5.2 Chat Completions conversion.
## Requirements
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

### Requirement: OpenCode Go Image-to-Text Bridge
The gateway SHALL support an optional image-to-text bridge for OpenCode Go requests whose final mapped upstream model is text-only.

#### Scenario: Image request bridged to GLM
- **WHEN** a Claude Code client sends `POST /v1/messages` through an `opencode_go` group, the mapped upstream model is `glm-5.2`, the request contains one or more Anthropic image content blocks, and the selected account has the image-to-text bridge enabled with a configured vision model
- **THEN** the gateway first sends the image content to the configured vision model, replaces the image content with generated textual image context, forwards a pure-text request to `glm-5.2`, and returns an Anthropic Messages response to the client

#### Scenario: Bridge disabled for text-only model
- **WHEN** a Claude Code client sends an image-containing `POST /v1/messages` request to an `opencode_go` account whose mapped upstream model is configured as text-only and the image-to-text bridge is disabled
- **THEN** the gateway returns a clear Anthropic-compatible error instead of forwarding unsupported image content to the text-only model

#### Scenario: Vision preflight failure
- **WHEN** the image-to-text bridge is enabled but the configured vision model request fails
- **THEN** the gateway returns an Anthropic-compatible upstream error and SHALL NOT forward the original image-containing request to `glm-5.2`

#### Scenario: Text-only request unchanged
- **WHEN** a Claude Code client sends a text-only `POST /v1/messages` request through an `opencode_go` account
- **THEN** the gateway uses the existing OpenCode Go forwarding behavior without invoking the image-to-text bridge

#### Scenario: Streaming bridge preflight remains alive
- **WHEN** a Claude Code client sends a streaming image-containing `POST /v1/messages` request that invokes the OpenCode Go image-to-text bridge
- **THEN** the gateway sends Anthropic-compatible SSE keepalive comments while the vision preflight is running so upstream vision latency does not leave the client/proxy connection idle
