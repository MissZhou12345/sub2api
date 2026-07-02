## ADDED Requirements
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
