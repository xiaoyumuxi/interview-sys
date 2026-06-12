# 个人 AI 面试训练平台后续执行路线图

## 1. 路线图目标

这份文档用于承接 `ai-interview-backend-plan.md`，从“架构设计”进入“可执行落地”。

目标不是一次性重写所有能力，而是按风险从低到高、按价值从核心到扩展，逐步完成后端建设。

优先级原则：

1. 先做后端，不动前端。
2. 先做可解释上下文，再做复杂 RAG。
3. 先做 DeepSeek 和 OpenAI-compatible，不做重 Provider 平台。
4. 先做记忆候选和人工审核，不自动污染画像。
5. 先做可调试、可回放、可验收的闭环，再扩展语音、搜索和图谱。

---

## 2. 总体阶段

```text
P0 需求冻结与技术底座确认
P1 Provider + Skill + Context Preview
P1.5 Python AI Runtime Foundation
P2 Retrieval Harness MVP
P2.5 Coding Question Bank MVP
P3 Memory Candidate + Profile Projection
P4 Interview Runtime MVP
P5 Retrieval Harness 增强版
P6 Evaluation Harness + 成本/质量评估
P7 前端接入准备
P8 语音、搜索、招聘扩展预留
```

建议先做到 P4，形成最小后端闭环。P5/P6 是为了让系统更有说服力，避免 RAG 和画像被质疑。

---

## 3. P0: 需求冻结与技术底座确认

### 目标

把第一版明确成“个人训练平台后端 MVP”，避免范围继续膨胀。

### 需要确认的决策

```text
主语言：Go + Python
主模型：DeepSeek
兼容模型：OpenAI-compatible
Embedding：免费额度 API
数据库：PostgreSQL + pgvector
缓存：Redis
对象存储：MinIO / RustFS
前端：暂不改
SaaS：暂不做
```

### 交付物

- 后端模块边界图。
- 数据库模块清单。
- API 分组清单。
- 第一版 Skill Pack 清单。
- 第一版评估样例清单。

### 验收标准

- 能清楚说明 MVP 做什么、不做什么。
- 能清楚说明为什么不用 LiteLLM、Neo4j、重型 GraphRAG。
- 能清楚说明 RAG 和画像如何避免“瞎编”。

### 预计成本

```text
Token: 20万 - 40万
时间: 1 - 2 天
```

---

## 4. P1: Provider + Skill + Context Preview

### 目标

先让系统能调用模型、加载 Skill、生成可解释上下文预览。

这是后续所有 Agent 能力的地基。

### 后端模块

```text
ai_provider
skill_registry
context_engine
agent_trace
```

### 功能清单

#### Provider

- DeepSeek Chat 调用。
- OpenAI-compatible Chat 调用。
- Embedding Provider 配置。
- Provider test endpoint。
- 任务路由配置。

#### Skill

- 扫描 Skill Pack。
- 解析 `skill.meta.yml`。
- 加载 `SKILL.md`。
- 加载 references。
- Skill 内容入库。

#### Context Preview

- `POST /api/context/preview`。
- 支持输入 task_type、skill_id、resume_id、jd_id、session_id。
- 返回 ContextItem 列表。
- 返回 token 预算。
- 返回上下文选择理由。

### 不做内容

- 不做完整面试流程。
- 不做复杂 RAG。
- 不做记忆审核。
- 不做前端页面。

### 验收标准

- 可以配置 DeepSeek 并成功调用。
- 可以加载至少一个 Skill Pack，比如 `java-backend`。
- 调用 context preview 能看到：Skill 指令、Skill references、任务说明。
- 每个上下文片段都有 source、score、reason。

### 风险

- Skill Pack 格式不稳定。
- DeepSeek JSON 输出不稳定。
- token 估算不准。

### 风险处理

- Skill meta schema 固定版本。
- 所有结构化输出做 JSON 修复和重试。
- token 先用粗估，后续接 tokenizer。

### 预计成本

```text
Token: 80万 - 150万
时间: 4 - 7 天
```

---

## 4.5 P1.5: Python AI Runtime Foundation

### 目标

建立独立 Python AI Runtime，让 Go Core API 不承载 Prompt、Agent 编排和结构化输出修复。

### 功能清单

- FastAPI Runtime 服务。
- OpenAI-compatible / DeepSeek Chat Adapter。
- Prompt 安全边界。
- 结构化 JSON 输出解析。
- Runtime task endpoint。
- 支持 question_generation、answer_evaluation、follow_up_decision、summary、memory_extraction。

### 不做内容

- 不直接写长期画像。
- 不推进 Go 面试状态机。
- 不在 Python 内做幂等和 single-flight。

### 验收标准

- `GET /healthz` 可用。
- `POST /api/runtime/tasks` dry-run 可返回 prompt messages。
- Provider 配置缺失时给出明确错误。
- Go 后续可以通过 HTTP 调用 Runtime。

---

## 5. P2: Retrieval Harness MVP

### 目标

完成比普通向量库 RAG 更有说服力的检索系统 MVP。

第一版重点不是“复杂”，而是“结构正确、可解释、可评估”。

### 核心模块

```text
document_ingestion
chunk_builder
summary_builder
fulltext_index
vector_index
retrieval_planner
evidence_selector
rag_debug
```

---

## 5.5 P2.5: Coding Question Bank MVP

### 目标

把编程考核作为独立模块纳入后端，而不是只在 Prompt 中口头要求。

### 核心模块

```text
code_question_bank
test_case_manager
submission_record
code_evaluation_trace
judge_worker_contract
```

### 功能清单

- 代码题集合。
- 题目标签、难度、题面、输入输出格式。
- 来源标记，第一批算法题以 CodeTop100 高频题为主。
- 按题型拆分：algorithm、backend_engineering、sql、debugging、system_design_small。
- 样例测试和隐藏测试。
- 参考解法和复杂度说明。
- 提交记录和人工/自动评估结果。
- 为未来 Docker sandbox judge worker 预留契约。

### 不做内容

- 第一版不直接开放不受限代码执行。
- 不在 Go API 进程内执行用户代码。
- 不允许 judge worker 默认访问网络。

### 验收标准

- 数据库中有可查询的代码题库和测试用例。
- Java 后端 Skill 能选择至少一道代码题作为考核项。
- 报告中能记录代码题表现。
- 后续可以无破坏接入独立 judge worker。

### 功能清单

#### Ingestion

- 文档入库。
- section 识别。
- semantic chunk。
- chunk summary。
- document summary。

#### Multi-index

- PostgreSQL full-text。
- pgvector。
- Skill reference index。
- summary index。

#### Query Router

支持任务类型：

```text
question_generation
answer_evaluation
follow_up_decision
knowledge_qa
review_planning
```

#### Retrieval Planner

根据任务选择 retriever：

```text
skill_reference_retriever
keyword_retriever
vector_retriever
summary_retriever
rubric_retriever
```

#### RAG Debug

`POST /api/rag/debug` 返回：

- 原始 query。
- query plan。
- retriever 列表。
- 每个 retriever 命中结果。
- 最终入选 context。
- 被丢弃候选及原因。

### 不做内容

- 不做外部 Web search。
- 不做 Neo4j。
- 不做完整 GraphRAG 社区摘要。
- 不做复杂 reranker。

### 验收标准

- 同一个问题可以看到 keyword、vector、summary 的不同命中。
- 可以解释为什么某段上下文进入 prompt。
- 可以解释为什么某段上下文被丢弃。
- Embedding 不可用时可以降级到 keyword + Skill。

### 预计成本

```text
Token: 120万 - 220万
时间: 1 - 2 周
```

---

## 6. P3: Memory Candidate + Profile Projection

### 目标

实现“AI 提取候选记忆 -> 人工审核 -> 长期画像 -> 下次训练可用”的闭环。

### 核心模块

```text
memory_candidate
memory_review
profile_projection
review_scheduler
memory_context_score
```

### 功能清单

#### Candidate Extraction

面试或训练结束后提取：

```text
weak_point
strong_point
behavior_signal
preference
project_fact
review_task
```

#### Review

支持：

```text
approve
reject
edit
conflict review
```

#### Profile Projection

只基于 approved memory 生成。

#### Scoring

实现：

```text
memory_context_score =
  task_match_score       * 0.30
+ evidence_score         * 0.25
+ review_priority_score  * 0.20
+ recency_score          * 0.10
+ repeat_score           * 0.10
+ user_confirmed_score   * 0.05
```

#### SM-2 Review

- 弱点 approve 后生成复习计划。
- 每次训练后更新复习间隔。
- 提供 due review 查询。

### 不做内容

- 不自动写画像。
- 不自动覆盖旧事实。
- 不把 pending memory 放进正式 prompt。

### 验收标准

- AI 生成的所有记忆默认 pending。
- 用户 approve 后进入 profile projection。
- 新事实质疑旧事实时产生 conflict，而不是覆盖。
- 每条画像都能追溯证据。
- 每条记忆进入上下文时能解释分数。

### 预计成本

```text
Token: 100万 - 180万
时间: 1 - 2 周
```

---

## 7. P4: Interview Runtime MVP

### 目标

完成一次完整后端面试闭环：创建 session、出题、答题、评分、追问、总结、生成报告、生成候选记忆。

### 核心模块

```text
interview_session
question_generation
answer_evaluation
follow_up_decision
final_report
runtime_snapshot
single_flight
idempotency
```

### 状态机

```text
created
context_preparing
question_generating
waiting_answer
evaluating
follow_up_deciding
finished
failed
```

### 功能清单

- 创建面试 session。
- 按 Skill + 上下文生成问题。
- 提交回答。
- 幂等处理重复回答。
- 调用评分 Agent。
- 调用追问决策 Agent。
- 生成最终报告。
- 生成 memory candidates。
- 写入 agent trace。
- Redis 热状态。
- PostgreSQL snapshot。

### 验收标准

- 可以完成一次 Java 后端 Skill 模拟面试。
- 重复提交同一答案不会重复调用 AI。
- AI 失败时状态不错误推进。
- Redis 丢失后可以从 snapshot 恢复。
- 报告包含 Q&A、评分、追问、上下文来源。
- 结束后生成 pending memory candidates。

### 预计成本

```text
Token: 150万 - 280万
时间: 2 - 3 周
```

---

## 8. P5: Retrieval Harness 增强版

### 目标

解决别人质疑 RAG 太普通的问题，补上实体、关系、claim、rubric、历史问题去重等能力。

### 核心模块

```text
entity_extractor
relation_extractor
claim_extractor
rubric_index
question_history_index
graph_expansion
```

### 功能清单

#### 实体

```text
concept
technology
project
skill
question
weakness
rubric
```

#### 关系

```text
related_to
used_in
weak_at
asked_in
prerequisite_of
supported_by
```

#### Graph Expansion

- 1-hop expansion。
- 2-hop limited expansion。
- relation evidence lookup。
- entity disambiguation。

#### Claim

从文档中抽取可验证陈述：

```text
subject
predicate
object
evidence_chunk_id
confidence
```

### 验收标准

- 出题时能根据 Skill + 用户弱点 + 知识关系扩展考察点。
- 评分时能引用 rubric 和 reference evidence。
- 追问时能沿着 concept relation 深挖。
- 历史题去重生效。
- 每条图谱关系有 evidence。

### 预计成本

```text
Token: 150万 - 300万
时间: 2 - 4 周
```

---

## 9. P6: Evaluation Harness

### 目标

让系统不是“看起来高级”，而是能被评估、能被调参。

### 核心模块

```text
golden_dataset
retrieval_eval
context_eval
json_output_eval
cost_eval
latency_eval
```

### 数据集

准备固定样例：

```text
简历样例
JD 样例
Skill Pack 样例
知识库样例
问答样例
评分期望
弱点提取期望
RAG 命中期望
```

### 指标

```text
retrieval_hit_rate
context_precision
citation_coverage
json_success_rate
score_stability
memory_candidate_precision
avg_latency
avg_token_cost
```

### 验收标准

- 可以对比 keyword、vector、summary、entity expansion 的效果。
- 可以看到某次 Agent 调用成本。
- 可以看到哪个模型 JSON 成功率更高。
- 可以看到 RAG 命中质量是否提升。

### 预计成本

```text
Token: 80万 - 160万
时间: 1 - 2 周
```

---

## 10. P7: 前端接入准备

### 目标

虽然暂不改前端，但后端要为未来前端接入做好准备。

### 需要准备的接口能力

```text
Provider 设置页接口
Skill 列表接口
Context Preview 调试接口
RAG Debug 接口
Memory Review 列表接口
Profile 页面接口
Interview Session 接口
Report 页面接口
Trace 查看接口
```

### API 设计原则

- 当前前端可以不接。
- API response 稳定。
- 所有 ID、status、schema version 固定。
- 对调试页面友好。

### 验收标准

- 使用 Postman / curl 可以完成完整后端流程。
- 未来前端只需要按 API 接入，不需要理解 Python 内部逻辑。

### 预计成本

```text
Token: 40万 - 80万
时间: 3 - 5 天
```

---

## 11. P8: 后续扩展

### 语音面试

后续接入：

```text
ASR
TTS
WebSocket
录音复盘
语音转写 Q&A 抽取
```

暂不做 WebRTC。

### Web Search

默认关闭，后续接入：

```text
SearXNG
Tavily
Brave Search
SerpAPI
```

搜索结果必须标记 `web_unverified`，不能覆盖 Skill 和用户审核画像。

### SaaS 招聘

后续如果需要再做：

```text
space / organization
candidate
invite link
anti-cheating
coding question
whiteboard
team dashboard
```

当前仅保留 `space_id`。

---

## 12. 推荐实施顺序

最推荐顺序：

```text
P0 -> P1 -> P2 -> P3 -> P4 -> P6 -> P5 -> P7
```

原因：

- 先做 P1/P2，避免 Agent 没有可靠上下文。
- 再做 P3，避免画像系统空转。
- 再做 P4，形成面试闭环。
- P6 应该尽早做，否则不知道 RAG 和评分是否真的有效。
- P5 是增强 RAG 说服力，可以在 MVP 后继续打磨。

如果追求最快可演示：

```text
P0 -> P1 -> P4 简化版 -> P3 简化版 -> P2
```

但这个路径会牺牲 RAG 说服力，不推荐作为长期路线。

---

## 13. 粗略总预算

### 后端 MVP

包含 P0-P4：

```text
Token: 450万 - 870万
时间: 5 - 9 周
```

### 后端增强版

包含 P0-P6：

```text
Token: 680万 - 1330万
时间: 8 - 15 周
```

### 完整后端可展示版

包含 P0-P7：

```text
Token: 720万 - 1410万
时间: 9 - 16 周
```

如果后续加入语音和前端重构，另算。

---

## 14. 最大风险与处理方式

### 风险 1: RAG 仍被质疑普通

处理：

- 必须做 RAG Debug。
- 必须做多索引。
- 必须做 evaluation dataset。
- 必须能解释每个 context item 的来源和分数。

### 风险 2: 画像被质疑是 AI 编造

处理：

- 所有记忆 pending。
- 所有记忆必须带 evidence。
- 人工 approve 后才进入 profile。
- 新旧事实冲突只提示，不覆盖。

### 风险 3: LLM 输出不稳定

处理：

- JSON schema。
- 重试。
- 本地 JSON repair。
- golden tests。
- provider trace。

### 风险 4: 成本超出学生预算

处理：

- DeepSeek first。
- Embedding 用免费额度 API。
- Web search 默认关闭。
- GraphRAG 不全量上。
- Rerank 先不用专用模型。

### 风险 5: 后端太复杂做不完

处理：

- P1-P4 是最小闭环。
- P5/P6 可增量。
- 前端暂不动。
- SaaS 不做。

---

## 15. 每阶段验收问题

每阶段完成后都要能回答这些问题。

### P1

```text
模型能不能调通？
Skill 能不能加载？
上下文能不能预览？
为什么拼这些上下文能不能解释？
```

### P2

```text
RAG 命中了什么？
为什么命中？
为什么没选其他片段？
Embedding 不可用时能否降级？
```

### P3

```text
记忆从哪里来？
有没有证据？
谁审核了？
为什么这条记忆进入上下文？
```

### P4

```text
面试能否跑完？
重复提交会不会重复扣费？
AI 失败状态是否安全？
Redis 丢失是否可恢复？
```

### P5

```text
系统是否理解知识点关系？
追问是否能沿着概念深入？
评分是否能引用 rubric？
历史题是否能去重？
```

### P6

```text
RAG 质量有没有数据？
模型输出稳定性有没有数据？
成本和延迟有没有数据？
```

---

## 16. 下一步建议

下一步建议先做 P0 的输出文档：

1. `backend-module-boundary.md`
2. `database-draft-schema.md`
3. `api-contract-draft.md`
4. `skill-pack-schema.md`
5. `context-recipe-v1.yml`
6. `rag-evaluation-samples.md`

这些文档确定后，再开始写代码会稳很多。

---

## 附录：参考来源索引

详细参考来源见：[`project-reference-map.md`](./project-reference-map.md)。
