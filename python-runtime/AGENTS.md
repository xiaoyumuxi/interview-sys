# 仓库协作指南

## 适用范围

本文件适用于 `python-runtime` 子目录。它是个人 AI 面试训练平台中的 FastAPI AI Runtime，负责非确定性 AI 推理、LLM 调用、Prompt 安全、结构化输出解析/修复，以及 memory candidate/review/profile projection 等大模型应用层逻辑。

## 项目结构与模块边界

- `app/main.py`：FastAPI 入口，注册 Runtime HTTP API。
- `app/providers.py`：根据 Go Core API 传入的 Provider 配置执行模型请求。
- `app/prompt_security.py`：Prompt 安全检测、注入风险识别和输入约束。
- `app/structured_output.py`：结构化输出解析、修复和校验。
- `app/memory.py`：memory candidate、review、profile projection 等 API。
- `app/models.py`、`app/config.py`、`app/tasks.py`：共享 schema、运行配置和任务定义。
- `tests`：Python Runtime 单元测试，文件命名为 `test_*.py`。
- `pyproject.toml`、`uv.lock`：Python 依赖声明与锁定版本。
- `Dockerfile`：Runtime 容器构建文件。

## 本地开发命令

- `uv sync`：安装并同步 Python 依赖。
- `uv run uvicorn app.main:app --host 0.0.0.0 --port 8090 --reload`：本地启动 Runtime，监听 `8090`。
- `uv run python -m unittest discover -s tests -p 'test_*.py' -v`：运行 Python Runtime 单元测试。
- `docker build -t ai-interview-python-runtime .`：构建 Runtime 镜像。

从父级 monorepo 执行时，优先使用 `make run-runtime` 和 `make test-python`。

## Go / Python 职责边界

Go Core API 负责确定性业务事实、状态推进、幂等、审计、Provider 管理和对外 API。Python Runtime 只处理模型相关的非确定性逻辑。

关键约束：

- 面试 session / flow / turn 状态机只能由 Go 推进。
- Python 不直接写 Go 业务主表，不绕过 Go interview runtime。
- Provider 配置、密钥来源、task routing 和连通性测试由 Go 管理。
- Python 只使用请求中传入的 Provider 配置执行模型调用，不自行决定 task routing。
- Retrieval Harness MVP、Evaluation Harness 记录/断言、Final report 持久化和 coding judge 由 Go 管理；Python 只提供 memory search/scoring、Runtime task 输出和 summary 文本生成。
- Python 不执行用户代码，不参与代码题判题状态推进。
- Python trace 不记录 API key；审计事实由 Go 写入，例如 `agent_traces`。
- Redis single-flight、outbox、worker claim、dead-letter、重试等可靠性逻辑留在 Go。


## 编码风格

Python 代码遵循 PEP 8，并保持 Python 3.13 兼容。新增请求和响应结构优先使用明确的 Pydantic model。模块名保持短小、小写，并贴合当前 `app` 下的职责划分。

改动应优先沿用现有 Runtime 模块，不为局部需求引入跨 Go/Python 边界的抽象。涉及业务状态推进、Provider 路由或主业务表写入的逻辑，应放回 Go Core API。

## 测试要求

测试使用标准库 `unittest`，位于 `tests`，命名为 `test_*.py`。新增 Prompt 安全、结构化输出、Provider 请求组装、memory API 或任务 schema 行为时，应补充聚焦测试。

提交 Python Runtime 变更前运行：

```bash
uv run python -m unittest discover -s tests -p 'test_*.py' -v
```

只修改文档时无需运行测试，但应确认命令、目录和父级 `AGENTS.md`、Makefile 约定一致。

## 配置与安全

不要提交真实密钥。本地配置可使用父级项目约定的 `.env`。Provider key 应通过 Go 管理的 Provider 配置流转；Python Runtime 不持久化 API key。

Runtime 日志、错误信息和 trace 中不得输出 API key、敏感 header 或包含凭据的原始 payload。未设置加密密钥时，是否允许写入 Provider key 的判断属于 Go，不应在 Python 中实现替代路径。

## Commit 与 Pull Request

提交信息使用简短祈使句，和父级仓库历史一致，例如 `Add worker metrics summary API` 或 `Add runtime memory tests`。PR 应包含简要 summary、测试结果、配置变化说明，以及任何 Runtime API contract 变化。
