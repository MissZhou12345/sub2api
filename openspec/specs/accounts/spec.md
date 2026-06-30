# accounts Specification

## Purpose
TBD - created by archiving change add-opencode-go-proxy. Update Purpose after archive.
## Requirements
### Requirement: OpenCode Go Account Configuration
An OpenCode Go upstream SHALL be configurable without a schema migration by using an OpenAI API-key account with OpenCode Go compatibility metadata.

#### Scenario: Minimal account configuration
- **WHEN** an admin creates an account with `platform=openai`, `type=apikey`, `credentials.api_key`, `credentials.base_url`, and `extra.opencode_go=true`
- **THEN** the gateway treats the account as an OpenCode Go-compatible upstream while preserving existing OpenAI account scheduling and billing behavior

#### Scenario: Custom Anthropic endpoint
- **WHEN** `extra.opencode_go_anthropic_base_url` is configured
- **THEN** the gateway uses it for Anthropic-native OpenCode Go model forwarding instead of deriving the endpoint from `credentials.base_url`

