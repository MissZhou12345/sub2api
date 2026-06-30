## Context
The previous implementation used `platform=openai`, `type=apikey`, and `extra.opencode_go=true`. That works mechanically, but it conflicts with the desired mental model: OpenCode Go should be configured and scheduled like its own provider, similar to Kiro, while the client remains Claude Code CLI.

## Goals
- `opencode_go` is a first-class platform value.
- Admins can create an OpenCode Go API-key account from the UI without hidden `extra` fields.
- Claude Code CLI can call the normal `/v1/messages` endpoint with an API key bound to an OpenCode Go group.
- The default practical target is `glm-5.2`, mapped from Claude model names and forwarded through OpenCode Go Chat Completions.
- Usage accounting continues to record client-facing model and upstream model.

## Non-Goals
- Implement OpenCode CLI behavior.
- Require users to configure Claude Code as an OpenAI-compatible client.
- Add database schema columns; platform remains a string field.
- Support non-Messages OpenAI endpoints for OpenCode Go groups unless explicitly added later.

## Decisions
- Use platform id `opencode_go` and display name `OpenCode Go`.
- Account type is `apikey`; credentials use existing `api_key`, `base_url`, and `model_mapping`.
- Default base URL is `https://opencode.ai/zen/go/v1`.
- Default model mapping maps common Claude Code model aliases to `glm-5.2`.
- `/v1/messages` remains handled by the normal `GatewayHandler`, with a new `GatewayService.Forward` branch for OpenCode Go.
- `/v1/messages/count_tokens` returns a local 404 for OpenCode Go because OpenCode Go does not expose an Anthropic count-tokens equivalent.

## Risks / Trade-offs
- OpenCode Go may add native Anthropic endpoints for some models. Keep a configurable native-model list, but default GLM-5.2 to Chat Completions conversion.
- Model ids are provider-owned. If OpenCode Go changes `glm-5.2` to a different id, admins can override `model_mapping`.

## Migration Plan
- Stop requiring `extra.opencode_go` for new behavior.
- Existing wrong OpenAI+extra accounts are not auto-migrated in code; admins should create a new `opencode_go` account/group or export/import with the new platform value.
