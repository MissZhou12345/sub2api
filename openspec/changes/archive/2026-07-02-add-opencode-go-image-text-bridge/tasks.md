## 1. Specification
- [x] 1.1 Confirm desired billing behavior for vision preflight usage.
- [x] 1.2 Confirm OpenCode Go image request schema for `kimi-k2.7-code` and/or `kimi-k2.6`.

## 2. Backend
- [x] 2.1 Add OpenCode Go bridge configuration parsing on `Account`.
- [x] 2.2 Add helpers to detect and count Anthropic image blocks in messages and tool results.
- [x] 2.3 Add a transformer that replaces image blocks with generated text while preserving surrounding user text.
- [x] 2.4 Add OpenCode Go vision preflight call using configured model and existing account credentials.
- [x] 2.5 Integrate preflight before the existing `glm-5.2` Chat Completions conversion path.
- [x] 2.6 Return Anthropic-compatible errors for disabled bridge, too many images, invalid images, or preflight failure.
- [x] 2.7 Add logging/ops metadata that marks image bridge preflight usage without logging image data.

## 3. Tests
- [x] 3.1 Unit-test image detection and image-to-text request transformation.
- [x] 3.2 Unit-test opencode_go path sends image requests to the bridge model and pure text to `glm-5.2`.
- [x] 3.3 Unit-test disabled bridge and preflight failure error behavior.
- [x] 3.4 Run focused backend tests for `apicompat` and `service` packages.
