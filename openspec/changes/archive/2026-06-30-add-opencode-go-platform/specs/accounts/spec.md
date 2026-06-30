## MODIFIED Requirements
### Requirement: OpenCode Go Account Configuration
An OpenCode Go upstream SHALL be configurable as a first-class API-key account using `platform=opencode_go`, not as an OpenAI account with compatibility metadata.

#### Scenario: Minimal account configuration
- **WHEN** an admin creates an account with `platform=opencode_go`, `type=apikey`, `credentials.api_key`, and optionally `credentials.base_url`
- **THEN** the gateway treats the account as an OpenCode Go upstream and uses `https://opencode.ai/zen/go/v1` as the default base URL when none is provided

#### Scenario: GLM default model mapping
- **WHEN** an OpenCode Go account is created without a custom `credentials.model_mapping`
- **THEN** common Claude Code model names are mapped to `glm-5.2` by default

#### Scenario: Custom Anthropic endpoint
- **WHEN** `credentials.anthropic_base_url` is configured
- **THEN** the gateway uses it for Anthropic-native OpenCode Go model forwarding instead of deriving the endpoint from `credentials.base_url`

#### Scenario: OpenAI compatibility metadata ignored
- **WHEN** an account is created with `platform=openai` and `extra.opencode_go=true`
- **THEN** the account remains an OpenAI account and is not treated as an OpenCode Go account
