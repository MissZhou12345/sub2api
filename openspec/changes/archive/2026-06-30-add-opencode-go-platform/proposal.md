# Change: Add OpenCode Go as an independent platform

## Why
OpenCode Go packages expose models such as GLM-5.2 through an OpenAI-compatible upstream, but users want to consume those models from Claude Code CLI through Anthropic Messages semantics. Modeling OpenCode Go as an OpenAI group hides the actual routing intent and makes configuration confusing.

## What Changes
- Add a first-class `opencode_go` platform for accounts, groups, quotas, channels, and admin UI.
- Allow `opencode_go` API-key accounts to receive Claude Code `/v1/messages` traffic and forward it to OpenCode Go.
- Convert Anthropic Messages requests/responses to and from OpenAI Chat Completions for models such as `glm-5.2`.
- Keep optional Anthropic-native OpenCode Go forwarding for provider models that require `/v1/messages`.
- Remove the OpenAI-account `extra.opencode_go` compatibility path from the active behavior and specs.

## Impact
- Affected specs: `accounts`, `gateway`
- Affected code: backend platform constants, account validation/default mapping, gateway forwarding, scheduler/platform lists, admin account/group/channel UI, model whitelist/default mappings, tests
