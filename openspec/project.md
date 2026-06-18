# Project Context

## Purpose

Sub2API 是一个 **AI API 网关平台**，用于分发和管理 AI 产品订阅的 API 配额。用户通过平台生成的 API Key 调用上游 AI 服务（Anthropic Claude、OpenAI Codex、Google Gemini、Kiro 等），平台负责鉴权、计费、负载均衡、并发/速率控制与请求转发。

本仓库为 **Kiro 支持 Fork**（基于上游 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)），在保持与官方分支稳定合并的同时，长期维护 Kiro 渠道及相关能力（OAuth / AWS Builder ID / Token 导入、Anthropic Prompt Cache 用量模拟等）。

核心目标：

- 多上游账号统一接入与智能调度（含粘性会话）
- Token 级精确计费与用量追踪
- 用户自助充值与订阅管理
- 管理后台监控、运维与外部系统集成

## Tech Stack

### Backend

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.26.4 |
| Web 框架 | Gin |
| ORM | Ent (`entgo.io/ent`) |
| 依赖注入 | Google Wire |
| 数据库 | PostgreSQL 15+（开发常用 16） |
| 缓存 / 队列 | Redis 7+ |
| 配置 | Viper (`config.yaml`) |
| 认证 | JWT、TOTP、OIDC / OAuth（LinuxDo、微信、钉钉、GitHub、Google 等） |
| 支付 | EasyPay、支付宝、微信、Stripe、Airwallex |
| HTTP 客户端 | imroc/req、自定义 TLS 指纹 |
| 定时任务 | robfig/cron |
| 测试 | testify、testcontainers、build tags（`unit` / `integration` / `e2e`） |
| Lint | golangci-lint v2.9（CI） |

### Frontend

| 类别 | 技术 |
|------|------|
| 框架 | Vue 3.4+、TypeScript |
| 构建 | Vite 5 |
| 状态管理 | Pinia |
| 路由 | Vue Router 4 |
| 国际化 | vue-i18n |
| HTTP | Axios |
| UI 工具 | @vueuse/core、Tailwind CSS、Chart.js |
| 包管理 | **pnpm**（非 npm；必须提交 `pnpm-lock.yaml`） |
| 测试 | Vitest、@vue/test-utils |
| Lint | ESLint + @typescript-eslint + eslint-plugin-vue |

### Infrastructure & Tooling

- Docker / docker-compose（`deploy/`）
- GitHub Actions CI（`backend-ci.yml`、`security-scan.yml`、`release.yml`）
- 独立数据管理进程 `datamanagementd`
- SQL 迁移脚本（`backend/migrations/`）

## Project Conventions

### Code Style

**Go（后端）**

- 分层架构：`handler` → `service` → `repository`，Wire 组装依赖
- 包路径：`github.com/Wei-Shaw/sub2api/internal/...`
- 修改 `ent/schema/` 后必须 `go generate ./ent` 并提交生成代码
- 数据库变更通过 `backend/migrations/` 编号 SQL 文件
- 接口新增方法时，所有测试 stub/mock 必须同步补全
- 使用 `golangci-lint run ./...` 保持代码质量

**TypeScript / Vue（前端）**

- 组合式 API（Composition API）为主
- 目录约定：`api/`（接口）、`components/`、`views/`、`stores/`（Pinia）、`composables/`、`types/`、`i18n/`
- `pnpm install` 安装依赖；`pnpm run lint:check` + `vue-tsc --noEmit` 做静态检查
- 新增/修改函数时添加函数级注释（团队约定）

**通用**

- 保持最小改动范围，遵循现有命名与抽象风格
- 注释仅解释非显而易见的业务逻辑

### Architecture Patterns

```
backend/
├── cmd/server/           # 入口，Wire 依赖注入
├── ent/schema/           # 数据模型定义
├── internal/
│   ├── handler/          # HTTP 层（含 admin/ 子包）
│   ├── service/          # 业务逻辑（网关、计费、OAuth、支付等）
│   ├── repository/       # 数据访问
│   ├── server/           # 路由注册、中间件
│   ├── payment/          # 支付提供商抽象
│   ├── config/           # 配置结构
│   └── integration/      # E2E 测试
└── migrations/           # 顺序编号 SQL 迁移

frontend/src/
├── api/                  # REST 客户端
├── views/                # 页面（admin/、user/、auth/、setup/）
├── components/           # 可复用组件
├── stores/               # Pinia 全局状态
└── router/               # 路由与守卫
```

关键模式：

- **API 网关**：`/api/v1` 管理接口 + 独立 Gateway 路由（API Key 鉴权转发上游）
- **多租户计费**：Token 级 usage log、分组定价、订阅与余额
- **账号调度**：按平台/分组选择上游账号，支持粘性会话与并发限制
- **缓存分层**：Redis 用于鉴权、订阅、Dashboard 等热数据

### Testing Strategy

| 层级 | 命令 | 说明 |
|------|------|------|
| 后端单元测试 | `go test -tags=unit ./...` | 默认开发验证 |
| 后端集成测试 | `go test -tags=integration ./...` | 需 PostgreSQL / Redis |
| 后端 E2E | `go test -tags=e2e ./internal/integration/...` | 完整链路 |
| 前端关键路径 | `make test-frontend-critical` | 支付、OAuth 回调等 Vitest |
| 全量前端 | `pnpm --dir frontend run test:run` | Vitest |
| Lint | `golangci-lint run ./...`、`pnpm run lint:check` | CI 必过 |

PR 提交前检查清单（见 `DEV_GUIDE.md`）：

- 单元测试 + 集成测试通过
- golangci-lint 无新增问题
- 若改 `package.json`，`pnpm-lock.yaml` 已同步
- 若改 Ent schema，生成代码已提交
- 若改 interface，测试 stub 已补全

### Git Workflow

- 上游：`Wei-Shaw/sub2api`；本 Fork 长期维护 Kiro 能力
- 功能开发：`feature/<name>` 分支
- 同步上游：`git fetch upstream && git rebase upstream/main`
- Commit 风格：简洁、说明「为什么」；常见前缀 `feat:`、`fix:`、`chore:`、`refactor:`
- **仅在用户明确要求时创建 commit / push / PR**
- Release 通过 tag `v*` 触发 `release.yml`

## Domain Context

### 核心领域概念

| 概念 | 说明 |
|------|------|
| **Account（上游账号）** | 接入 Claude / OpenAI / Gemini / Kiro 等平台的 OAuth 或 API Key 凭证 |
| **Group（分组）** | 账号池与调度策略容器；可配置模型映射、并发、缓存模拟等 |
| **API Key** | 平台发给终端用户的调用凭证，关联用户与配额 |
| **Channel** | 对外暴露的服务通道（模型列表、定价、限制） |
| **Usage Log** | Token 级用量与计费记录 |
| **Subscription** | 用户订阅计划与额度 |
| **Gateway** | 将用户请求按策略转发到上游 AI API |

### 平台类型

支持多种上游：`anthropic`、`openai`、`gemini`、`antigravity`、`kiro` 等。批量修改账号时注意按平台分组，避免跨平台模型映射被覆盖。

### Kiro Fork 特有能力

- Kiro OAuth / AWS Builder ID / Token 导入
- Anthropic Prompt Cache 用量模拟（按分组配置开关与模拟比例）

### 用户角色

- **普通用户**：API Key 管理、充值、用量查看
- **管理员**：账号/分组/通道/运维/支付/合规等全量后台能力

## Important Constraints

- **合规风险**：使用本项目可能违反上游服务商 ToS；仅供学习研究，风险由使用者承担
- **官方域名**：Sub2API 官方仅使用 `sub2api.org` 与 `pincc.ai`
- **Windows 开发注意**：PowerShell 中 `$` 转义、无 `make` 时直接用 `go test`、psql 建议用 `127.0.0.1`
- **前端必须用 pnpm**：CI 使用 `--frozen-lockfile`，lock 文件必须提交
- **Go 版本锁定**：与 `backend/go.mod` 一致（当前 1.26.4）
- **OpenSpec 门禁**：新功能/破坏性变更需先提案并获批，再实现；Bug 修复、格式、非破坏性依赖升级可直接改

## External Dependencies

### 上游 AI 服务

- Anthropic API（Claude）
- OpenAI API（含 Codex / Responses）
- Google Gemini API
- Kiro（AWS Builder ID / OAuth）
- Antigravity 等扩展渠道

### 基础设施

- PostgreSQL（主数据存储）
- Redis（缓存、限流、会话）
- 可选 S3（备份等）

### 第三方服务

- Cloudflare Turnstile（人机验证）
- 支付：EasyPay、支付宝、微信支付、Stripe、Airwallex
- OAuth 提供方：LinuxDo、微信、钉钉、GitHub、Google、OIDC 通用
- 邮件发送（注册/通知，依配置而定）

### 参考文档

- `README.md` / `README_CN.md` — 功能与部署
- `DEV_GUIDE.md` — 本地环境、CI、常见坑点
- `docs/PAYMENT.md` / `docs/PAYMENT_CN.md` — 支付配置
