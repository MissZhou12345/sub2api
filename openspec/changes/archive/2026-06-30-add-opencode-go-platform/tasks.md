## 1. Spec
- [x] 1.1 Add spec deltas for independent OpenCode Go account configuration.
- [x] 1.2 Add spec deltas for Claude Code `/v1/messages` proxying through OpenCode Go.
- [x] 1.3 Validate the OpenSpec change.

## 2. Backend
- [x] 2.1 Add `opencode_go` platform constants and default model mapping.
- [x] 2.2 Remove the OpenAI-account `extra.opencode_go` active branch.
- [x] 2.3 Add OpenCode Go account helpers for API key, base URL, endpoint derivation, and native-model detection.
- [x] 2.4 Add `GatewayService` OpenCode Go forwarding for streaming and non-streaming `/v1/messages`.
- [x] 2.5 Return local 404 for `/v1/messages/count_tokens` on OpenCode Go.
- [x] 2.6 Include OpenCode Go in scheduler snapshots, quotas, channels, model lists, and validation lists.
- [x] 2.7 Add or update backend tests.

## 3. Frontend
- [x] 3.1 Add `opencode_go` to platform types, labels, filters, quotas, channels, and icons.
- [x] 3.2 Add OpenCode Go account creation/editing UI using API Key + Base URL + model mapping.
- [x] 3.3 Add default OpenCode Go model whitelist/preset mappings to `glm-5.2`.
- [x] 3.4 Add or update frontend tests where coverage exists.

## 4. Verification
- [x] 4.1 Run focused Go tests for service/handler paths.
- [x] 4.2 Run focused frontend type/tests if available.
- [x] 4.3 Run `openspec validate --all --strict --no-interactive`.
- [x] 4.4 Archive the change after implementation.
