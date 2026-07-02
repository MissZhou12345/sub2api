## Context
OpenCode Go accounts currently serve Claude Code Anthropic Messages requests by mapping Claude model names to `glm-5.2` and converting requests to OpenAI Chat Completions. The compatibility converters already support Anthropic `image` blocks, Responses `input_image`, and Chat Completions `image_url`. The failure occurs because `glm-5.2` rejects image input.

The requested user experience is: keep selecting `glm-5.2` in Claude Code, paste image + text into the CLI, and have the final answer produced by `glm-5.2` using text extracted from the image.

## Goals
- Preserve Claude Code CLI image input UX.
- Keep the final model as `glm-5.2` for the main response.
- Use a configured OpenCode Go vision-capable model for preflight image understanding.
- Strip image blocks before forwarding to `glm-5.2`.
- Make the behavior opt-in and auditable.

## Non-Goals
- Calling the user's local `zai-mcp-server` from sub2api. MCP installed in Claude Code is client-local; sub2api is a remote gateway process and cannot invoke it through the model request path.
- Native multimodal `glm-5.2`.
- Long-lived image storage.

## Decisions
- Decision: Implement a server-side "image-to-text bridge" in the OpenCode Go gateway path.
  - Rationale: sub2api can reliably inspect and transform Anthropic request bodies before upstream forwarding; this is the only place that can automatically prevent `glm-5.2` image-input errors.
- Decision: Use the same OpenCode Go account credentials and base URL to call the vision preflight model.
  - Rationale: keeps the bridge inside the configured upstream account boundary and avoids adding a new provider abstraction for the first implementation.
- Decision: Represent image understanding as injected user-visible text next to the original user text.
  - Rationale: `glm-5.2` receives a pure-text conversation with enough context, while history shape remains Anthropic-compatible.
- Decision: Make bridge activation conditional on request containing Anthropic image blocks and mapped upstream model matching configured text-only models, defaulting to `glm-5.2`.
  - Rationale: native multimodal models should still receive image blocks directly.

## Proposed Configuration
OpenCode Go account credentials MAY include:

- `image_text_bridge_enabled`: boolean, default `false`
- `image_text_bridge_model`: string, default empty; recommended `kimi-k2.7-code` or `kimi-k2.6`
- `image_text_bridge_target_models`: string array or comma-separated string, default `["glm-5.2"]`
- `image_text_bridge_max_images`: integer, default `4`
- `image_text_bridge_prompt`: optional custom prompt template

Exact key names may be adjusted during implementation to match existing credential naming conventions.

## Request Flow
1. Parse the inbound Anthropic Messages request.
2. Apply normal OpenCode Go model mapping.
3. If the mapped upstream model is a configured bridge target and messages contain image blocks:
   - validate bridge is enabled and a vision model is configured;
   - create one or more Chat Completions preflight requests to the vision model;
   - collect concise OCR/description text;
   - replace image blocks with text blocks containing the extracted image context.
4. Continue through the existing `AnthropicToResponses` and `ResponsesToChatCompletionsRequest` conversion.
5. Forward the final pure-text request to `glm-5.2`.

## Risks / Trade-offs
- Privacy: images are sent to the vision preflight model. Mitigation: opt-in configuration and documentation.
- Latency: image requests require one additional upstream call. Mitigation: only trigger when images exist.
- Cost: usage from the vision preflight needs accounting. Initial implementation can include it in logs/usage if upstream returns usage; exact billing behavior should be explicit in tasks.
- Quality: `glm-5.2` only sees the extracted text, not the original pixels. Fine-grained visual reasoning may be weaker than native multimodal inference.
- Failure behavior: if preflight fails, returning a clear error is safer than silently dropping images.

## Open Questions
- Should the vision preflight usage be billed separately, added to the final request usage, or logged as a child operation only?
- Should bridge configuration live only on account credentials, or should groups/channels expose a user-facing switch?
- Does OpenCode Go's `kimi-k2.7-code`/`kimi-k2.6` accept image input through the same `/v1/chat/completions` `image_url` schema used by current converters?
