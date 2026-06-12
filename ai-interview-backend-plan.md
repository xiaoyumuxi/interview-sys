# 个人 AI 面试训练平台后端建设计划

## 1. 背景与目标

本计划用于整合以下项目中的优势能力，建设一个轻量、可解释、可扩展的个人 AI 面试训练平台后端：

- `AI-Meeting`：面试运行时、追问推进、幂等、single-flight、状态恢复、Agent 工作流契约。
- `AI-Meeting-Frontend`：现有面试交互、报告页、ASR/TTS、摄像头、草图板等前端能力，暂不改动。
- `interview-guide`：Skill 驱动出题、知识库、Provider 配置、统一评估、语音面试和产品完整度。
- `TechSpar`：用户画像、长期记忆、掌握度、SM-2 复习、LangGraph 面试状态机、训练闭环。
- `aural-oss`：招聘评估产品形态、候选人/邀请/反作弊/代码题/白板等能力，仅作为未来扩展参考。

当前阶段不做 SaaS 招聘协作，不改前端，优先完成后端架构与核心能力建设。

核心目标：

1. 支持 DeepSeek 作为主力 LLM。
2. 支持 OpenAI-compatible API，便于接入 Kimi、GLM、硅基流动、ModelScope、Ollama、vLLM 等。
3. Claude 只做 Adapter 预留，不阻塞 MVP。
4. Embedding 使用免费额度 API，不强制本地大模型。
5. 用户画像和长期记忆必须人工审核后写入。
6. 上下文选择、记忆权重、RAG 命中都必须可解释、可追踪、可回放。
7. RAG 不能停留在普通向量库 topK，要建设面向 Agent 的 Context Retrieval Harness。

---

## 2. 总体架构

推荐架构：

```text
Go Core API
  - Auth / User
  - Provider 配置
  - Skill Registry
  - Knowledge Base
  - Coding Question Bank
  - Interview Runtime
  - Interview session / flow / turn 状态机
  - PostgreSQL local outbox
  - Redis Stream worker contract
  - Memory Review
  - Context Preview
  - Trace / Debug
  - Redis 幂等与 single-flight
  - PostgreSQL 持久化

Python AI Runtime
  - LLM Adapter
  - Agent Flow
  - Question Generation
  - Answer Evaluation
  - Follow-up Decision
  - Final Summary
  - Memory Candidate Extraction
  - Context Retrieval Harness
  - RAG / KG / Search Adapter
  - Code Evaluation Adapter

Storage
  - PostgreSQL + pgvector
  - Redis
  - S3 / MinIO / RustFS
```

职责划分：

- Go 负责确定性业务逻辑、状态机、幂等、落库、API、审计。
- Python 负责 AI 推理、Prompt、RAG、记忆候选提取、Agent 编排。
- Python 不直接更新长期画像，不直接推进面试状态。
- 所有 AI 输出必须结构化，且带 schema version。
- 异步任务先写 PostgreSQL local outbox，再由 Redis Stream 分发；Redis 只做短期调度、fan-out 和协调。
- 数据库不保存 `locked_by` / `locked_until` 这类持久锁字段；并发控制依赖状态机、幂等约束、`FOR UPDATE SKIP LOCKED` 和短 TTL Redis 协调。

### 2.1 当前落地清单

- [x] Go Core API、Gin 路由、PostgreSQL、Redis、MinIO 基础环境。
- [x] Provider 配置、密钥来源、任务路由和模型切换 API。
- [x] DeepSeek 和 OpenAI-compatible Provider 兼容。
- [x] Skill Pack 扫描、创建、热加载、lint 和提示词注入基础校验。
- [x] Java 后端 Skill 覆盖通用后端、网络、分布式、系统设计、算法和代码题。
- [x] Context Preview、Python Runtime 调用和 `agent_traces`。
- [x] Python AI Runtime、prompt 安全边界和结构化 JSON 解析。
- [x] CodeTop100 / 后端工程题库 schema、seed 和查询 API。
- [x] Interview Runtime session / flow / turn 三层状态机。
- [x] Answer 提交异步化：API 返回 `202 Accepted`，worker 消费 Redis Stream 后评估。
- [x] PostgreSQL local outbox `async_messages`，支持 Redis Stream 补投、重试和 dedup。
- [x] Redis single-flight、短 TTL Redis 协调、stale turn reclaim 和数据库幂等。
- [x] PostgreSQL runtime snapshot。
- [x] 独立 `cmd/worker` 进程，API 默认只负责入队和查询。
- [x] Redis Stream pending reclaim、dead-letter / poison message 兜底。

### 2.2 后续任务清单

- [ ] Redis Stream 消费延迟指标和 worker 可观测性。
- [ ] Retrieval Harness MVP：多索引检索、证据选择、debug trace。
- [ ] Python Runtime Memory candidate / review / profile projection。
- [ ] Docker sandbox judge worker。
- [ ] Final report generation。
- [ ] Evaluation Harness 和成本/质量回归。
- [ ] 前端接入异步 trace 和报告接口。

### 2.3 参考项目对齐原则

详细索引见 [project-reference-map.md](/Users/yaoyao/Documents/SelfProject/project-reference-map.md)。后续实现不能只按当前仓库文档闭门推进，需要按模块回看原项目：

- `AI-Meeting`：优先参考面试状态机、追问推进、answer pipeline、single-flight、恢复和 finalize。
- `interview-guide`：优先参考 Skill Pack、Provider 配置、结构化输出、统一评估、知识库基础和 Redis Stream 异步任务。
- `TechSpar`：优先参考长期画像、弱点/强项、SM-2 复习、LangGraph flow 和训练闭环；本项目保留人工审核边界。
- `AI-Meeting-Frontend`：优先参考未来前端面试页、报告页、问答回放、会话恢复和 ASR/TTS 接口契约。
- `aural-oss`：只作为未来招聘 SaaS、候选人、邀请、反作弊、代码题和白板能力预留，不进入当前 MVP 主链路。
- GraphRAG / RAPTOR / HyDE / RAG-Fusion：只作为 Retrieval Harness 设计参考，第一版按轻量、可解释、可回放实现。

---

## 3. LLM Provider 设计

不使用 LiteLLM，避免引入过重的配置和运行时复杂度。第一版自研轻量 Provider 层。

### 3.1 Provider 类型

```text
DeepSeekProvider
OpenAICompatibleProvider
ClaudeAdapter
EmbeddingProvider
```

默认策略：

- 出题、评分、追问、总结、记忆提取：DeepSeek。
- OpenAI-compatible：作为兼容层，支持后续替换模型。
- Claude：保留接口，具体接入方式后续确认。
- Embedding：独立配置，不依赖 DeepSeek。

### 3.2 Provider 配置字段

```text
provider_id
provider_type: deepseek | openai_compatible | claude | embedding
base_url
api_key
chat_model
embedding_model
supports_streaming
supports_json
enabled
created_at
updated_at
```

### 3.3 任务路由

```text
question_generation -> DeepSeek / OpenAI-compatible
answer_evaluation   -> DeepSeek / OpenAI-compatible
follow_up_decision  -> DeepSeek / OpenAI-compatible
summary             -> DeepSeek / OpenAI-compatible
memory_extraction   -> DeepSeek / OpenAI-compatible
embedding           -> EmbeddingProvider
```

### 3.4 Embedding 策略

DeepSeek 不提供通用 Embedding，因此 Embedding 单独配置。

优先支持：

- 硅基流动 Embedding API。
- ModelScope Embedding API。
- DashScope Embedding API。
- 任意 OpenAI-compatible embeddings endpoint。

Embedding 不可用时，RAG 降级为：

```text
Skill metadata match
  -> PostgreSQL full-text search
  -> keyword / category match
  -> approved memory match
  -> recent history match
```

---

## 4. Skill Context System

参考 `interview-guide` 的 `SKILL.md + skill.meta.yml` 模式，建立 Skill Pack。

### 4.1 目录结构

```text
skills/
  java-backend/
    skill.meta.yml
    SKILL.md
    references/
      backend-foundation.md
      java.md
      network.md
      distributed.md
      mysql.md
      redis.md
      spring.md
      system-design.md
      algorithm.md
      coding.md
      project.md
```

### 4.2 skill.meta.yml

```yaml
id: java-backend
displayName: Java 后端
description: Java / MySQL / Redis / Spring / 项目实战
categories:
  - key: JAVA
    label: Java
    priority: CORE
    ref: java.md
  - key: MYSQL
    label: MySQL
    priority: CORE
    ref: mysql.md
  - key: REDIS
    label: Redis
    priority: CORE
    ref: redis.md
  - key: SPRING
    label: Spring
    priority: NORMAL
    ref: spring.md
  - key: PROJECT
    label: 项目经历
    priority: ALWAYS_ONE
```

Java 后端 Skill 第一版不能只覆盖 Java/MySQL/Redis/Spring，还必须覆盖：

```text
通用后端基础
计算机网络
分布式系统
系统设计
算法与数据结构
编程题/代码题库
项目经历
```

代码题库是独立模块，不应只藏在 Skill prompt 中。第一批算法题以 CodeTop100 高频题作为主要来源，同时按题型拆分，不把所有编程考核都简化成算法题。

### 4.5 Coding Question Bank

后端需要维护可复用代码题库：

```text
code_question_set
code_question
code_question_test_case
code_submission
code_evaluation_trace
```

题目字段：

```text
question_id
title
difficulty: easy | medium | hard
source: CodeTop100 | local | imported
source_url
question_type: algorithm | backend_engineering | sql | debugging | system_design_small
frequency_rank
company_tags
topic_tags
prompt
input_format
output_format
constraints
sample_tests
hidden_tests
reference_solution
rubric
status
```

题型策略：

```text
algorithm              CodeTop100 高频算法与数据结构题
backend_engineering    LRU、限流器、缓存、并发控制、API 小题
sql                    SQL 查询、索引和事务推理
debugging              故障代码修复
system_design_small    小型设计题
```

MVP 阶段可以先支持题库管理、人审和离线测试；后续再做隔离代码执行。

代码执行要求：

- 使用 Docker / 独立 worker 沙箱。
- 禁止容器联网。
- 限制 CPU、内存、进程数、运行时间和文件系统。
- 每次执行记录 submission、test result、stdout/stderr、资源使用和 trace。
- 代码题评估结果进入面试报告和记忆候选，但不能直接写长期画像。

### 4.3 SKILL.md

`SKILL.md` 用于定义：

- 面试官角色。
- 考察目标。
- 提问顺序。
- 追问原则。
- 评分倾向。
- 禁止事项。
- 应优先参考的知识资源。

### 4.4 SkillRegistry

后端提供 `SkillRegistry`：

- 扫描本地 Skill Pack。
- 解析 `skill.meta.yml`。
- 加载 `SKILL.md` 和 references。
- 支持热加载本地 Skill Pack，不需要重启服务。
- 支持通过 API 新建 Skill Pack。
- 新建 Skill 必须经过 lint 和提示词注入校验。
- 热加载已有 Skill 时返回 lint 结果，不能静默吞掉风险。
- 将 references 入库。
- 支持 full-text index。
- 如果 Embedding 可用，则生成向量索引。
- 面试时按 skill、JD、简历、已审核弱点动态组装上下文。

---

## 5. Context Engine

上下文处理不要散落在 Agent Prompt 里。需要独立建设 Context Engine。

### 5.1 Pipeline

```text
Context Sources
  -> Retrieval
  -> Scoring
  -> Budgeting
  -> Compression
  -> Packing
  -> Prompt
  -> Trace
```

### 5.2 Context 类型

```text
system_context       系统规则
skill_context        Skill 指令、考察范围、rubric
task_context         当前任务说明
profile_context      已审核用户画像
evidence_context     简历、JD、知识库、历史问答、RAG 命中片段
session_context      当前会话上下文
```

优先级：

```text
system > skill > task > approved_profile > evidence > session_history
```

未审核记忆不能进入正式上下文。

### 5.3 ContextItem 结构

```json
{
  "id": "ctx_001",
  "source_type": "skill_reference",
  "source_id": "java-backend/redis.md",
  "trust_level": "trusted",
  "content": "...",
  "tokens": 320,
  "score": 0.86,
  "reason": "matched topic REDIS and weak_point distributed_lock",
  "created_at": "2026-06-12T10:00:00+08:00"
}
```

### 5.4 Context Recipe

不同 Agent 使用不同 Context Recipe。

出题：

```text
skill_context        30%
resume_jd_context    25%
approved_memory      20%
rag_context          15%
session_history      10%
```

评分：

```text
current_qa           35%
skill_rubric         25%
reference_context    20%
resume_jd_context    10%
approved_memory      10%
```

追问：

```text
current_qa           35%
evaluation_result    30%
missing_points       20%
follow_up_history    15%
```

记忆提取：

```text
transcript           40%
evaluation_result    25%
old_profile          20%
skill_context        15%
```

这些比例是工程默认策略，不宣称由 LLM 学习得到，后续通过评估集调参。

### 5.5 Context Preview

提供调试接口：

```text
POST /api/context/preview
```

输入：

```json
{
  "task_type": "question_generation",
  "skill_id": "java-backend",
  "resume_id": "...",
  "jd_id": "...",
  "session_id": "..."
}
```

输出：

```json
{
  "recipe": "question_generation_v1",
  "token_budget": 12000,
  "items": [],
  "final_prompt_preview": "...",
  "warnings": ["embedding_unavailable_fallback_to_keyword"]
}
```

该接口用于回答：Agent 这次到底看到了哪些上下文，为什么看到这些。

---

## 6. 可解释记忆评分

需要明确：记忆分数不是 LLM 学出来的，不是拍脑袋，而是可解释的规则打分。

### 6.1 两类分数

```text
memory_confidence      这条候选记忆本身是否可靠
memory_context_score   这条已审核记忆本次是否应该进入上下文
```

### 6.2 memory_context_score

```text
memory_context_score =
  task_match_score       * 0.30
+ evidence_score         * 0.25
+ review_priority_score  * 0.20
+ recency_score          * 0.10
+ repeat_score           * 0.10
+ user_confirmed_score   * 0.05
```

解释：

```text
task_match_score      当前任务是否匹配该记忆 topic
evidence_score        是否有真实问答/评分证据
review_priority_score 是否到复习时间，SM-2 是否提示该练
recency_score         最近是否出现过
repeat_score          是否多次暴露同类问题
user_confirmed_score  是否被用户审核确认
```

对外解释：

> 记忆权重是确定性 ranking score，综合任务匹配度、证据强度、复习优先级、时间衰减、重复出现次数和用户审核状态。LLM 只负责提取候选观察，最终是否进入长期记忆和上下文，由规则和人工审核决定。

---

## 7. 人工审核记忆系统

AI 不直接写长期画像。AI 只生成候选记忆。

### 7.1 memory_candidates

```text
candidate_id
user_id
type
topic
content
evidence
confidence
source_session_id
source_answer_id
conflicts_with
status: pending | approved | rejected | edited
created_at
updated_at
```

候选类型：

```text
weak_point
strong_point
behavior_signal
preference
project_fact
review_task
```

### 7.2 规则

- 默认全部 pending。
- 用户 approve 后进入长期画像。
- 用户 reject 后不再使用。
- 用户 edit 后以人工编辑版本为准。
- 新事实质疑旧事实时，只创建 conflict，不自动覆盖。
- profile projection 只基于 approved memory 生成。

### 7.3 画像解释

每条画像必须能追溯到证据：

```json
{
  "type": "weak_point",
  "topic": "redis",
  "content": "对 Redis 分布式锁续约机制理解不足",
  "status": "approved",
  "evidence": [
    {
      "source_type": "interview_answer",
      "session_id": "sess_001",
      "question": "Redisson watchdog 是怎么工作的？",
      "answer_excerpt": "锁过期后应该 Redis 会自动帮忙延长吧",
      "evaluation_excerpt": "回答混淆了锁过期和 watchdog 续约机制"
    }
  ]
}
```

---

## 8. RAG 升级方案：Context Retrieval Harness

普通 RAG 容易被质疑，因为它通常只是：

```text
文档切块 -> 向量化 -> topK 召回 -> 拼 Prompt
```

本项目需要升级为面向 Agent 的 Context Retrieval Harness。

### 8.1 总体 Pipeline

```text
Ingestion Layer
  -> Multi-index Layer
  -> Query Router
  -> Retrieval Planner
  -> Candidate Retrieval
  -> Rerank / Evidence Selection
  -> Context Packing
  -> Agent Use
  -> Evaluation / Trace
```

### 8.2 Ingestion Layer

文档入库时不只切 chunk，还要生成：

```text
raw_document          原始文档
document_section      章节结构
semantic_chunk        语义 chunk
chunk_summary         chunk 摘要
document_summary      文档摘要
topic_summary         主题摘要
entity                实体
relation              关系
claim                 可验证陈述
rubric                评分标准
question_candidate    候选问题
```

例如 Redis 文档：

```text
Sections:
  - Redis 持久化
  - Redis 分布式锁
  - Redisson watchdog
  - Redis 缓存一致性

Entities:
  - Redis
  - Redisson
  - watchdog
  - Lua script
  - leaseTime

Relations:
  - Redisson watchdog -> renews -> lock lease
  - leaseTime -> affects -> lock expiration
```

### 8.3 Multi-index Layer

同时建设多个索引，而不是单一向量库：

```text
Full-text Index       关键词、术语、专有名词
Vector Index          语义相似
Summary Index         文档/章节摘要
Entity Index          实体、概念、技术点
Relation Index        概念关系、前置知识、项目使用关系
Question Index        历史问题、候选题、追问题
Rubric Index          评分标准、参考答案、考察点
Memory Index          已审核弱点、强项、行为信号
```

### 8.4 Query Router

查询前先判断任务类型：

```text
question_generation
answer_evaluation
follow_up_decision
final_summary
memory_extraction
knowledge_qa
review_planning
```

不同任务走不同检索策略：

```text
出题：skill rubric + 用户弱点 + 历史题去重 + JD/简历 + concept graph
评分：当前答案 + rubric + reference answer + missing points
追问：当前答案 + 评分缺口 + skill 深挖路径
总结：topic summary + session summary + approved memory
知识问答：hybrid search + summary + graph expansion
复习规划：approved weak points + SM-2 + recent performance
```

### 8.5 Retrieval Planner

输出检索计划：

```json
{
  "task": "question_generation",
  "retrievers": [
    "skill_rubric",
    "approved_memory",
    "resume_jd",
    "history_dedup",
    "concept_graph"
  ],
  "budget": 12000
}
```

### 8.6 Candidate Retrieval

候选召回来源：

```text
skill_reference_retriever
keyword_retriever
vector_retriever
summary_retriever
entity_retriever
relation_retriever
question_history_retriever
rubric_retriever
memory_retriever
web_search_retriever
```

Web search 第一版默认关闭，只保留接口。

### 8.7 Evidence Selection

候选上下文打分：

```text
final_score =
  source_trust       * 0.25
+ task_relevance     * 0.25
+ retrieval_score    * 0.20
+ evidence_quality   * 0.15
+ freshness          * 0.05
+ diversity_bonus    * 0.05
+ user_approved      * 0.05
```

解释：

```text
source_trust      来源可信度：Skill > 用户审核画像 > 用户上传文档 > Web
task_relevance    与当前任务匹配度
retrieval_score   检索器原始分数
evidence_quality  是否有明确证据、是否可引用
freshness         时间新鲜度
diversity_bonus   避免上下文都来自同一来源
user_approved     是否经过用户确认
```

### 8.8 Context Packing

输出顺序：

```text
trusted system rules
skill instructions
rubric / scoring criteria
selected evidence
approved memory
session state
citations
```

每段都必须带来源和选择理由。

---

## 9. 轻量 KG 设计

不引入 Neo4j，先用 PostgreSQL 表做轻量图谱。

### 9.1 rag_entities

```text
id
type: concept | technology | project | skill | question | weakness | rubric
name
aliases
source_id
created_at
```

### 9.2 rag_relations

```text
id
source_entity_id
target_entity_id
relation_type: related_to | used_in | weak_at | asked_in | prerequisite_of | supported_by
evidence_chunk_id
confidence
created_at
```

### 9.3 rag_claims

```text
id
subject
predicate
object
evidence
source_id
confidence
created_at
```

### 9.4 Graph Expansion

第一版只做：

```text
1-hop expansion
2-hop limited expansion
entity disambiguation
relation evidence lookup
```

例如查询 Redis 分布式锁时扩展：

```text
Redis 分布式锁
  -> Redisson
  -> watchdog
  -> leaseTime
  -> Lua 解锁
  -> 锁续约失败
  -> 项目重复提交场景
```

---

## 10. Agent Harness

所有 Agent 调用必须可追踪、可回放。

### 10.1 agent_runs

```text
run_id
user_id
session_id
flow_type
task_type
status
model_provider
model_name
input_hash
output_hash
token_usage
latency_ms
started_at
finished_at
```

### 10.2 agent_steps

```text
step_id
run_id
step_type
input_json
output_json
error_json
latency_ms
```

### 10.3 agent_context_items

```text
run_id
context_item_id
source_type
source_id
score
reason
tokens
position
```

### 10.4 用途

可以回答：

- 为什么出了这个问题？
- 为什么给这个评分？
- 为什么召回这些上下文？
- 为什么某条记忆进入了 prompt？
- 哪个模型失败了？
- 这次调用花了多少 token？

---

## 11. Interview Runtime

Go 维护确定性状态。

### 11.1 状态机

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

### 11.2 规则

- 同一 `session_id + question_id + answer_round` 重复提交必须返回同一结果。
- 同一 AI 阶段调用通过 single-flight 去重。
- Redis 丢失时从 PostgreSQL snapshot 恢复。
- Python 调用失败时不推进业务状态。
- 所有 AI 输出必须带 schema version。

---

## 12. 后端 API 计划

### 12.1 Provider

```text
GET    /api/ai/providers
POST   /api/ai/providers
PUT    /api/ai/providers/{id}
POST   /api/ai/providers/{id}/test
GET    /api/ai/routes
PUT    /api/ai/routes
```

### 12.2 Skill / Context

```text
GET    /api/skills
GET    /api/skills/{id}
POST   /api/skills/reload
POST   /api/skills/{id}/index
POST   /api/context/preview
```

### 12.3 RAG

```text
POST   /api/knowledge-bases
POST   /api/knowledge-bases/{id}/documents
POST   /api/knowledge-bases/{id}/index
POST   /api/rag/query
POST   /api/rag/debug
```

### 12.4 Memory

```text
GET    /api/runtime/memory/candidates
POST   /api/runtime/memory/candidates
POST   /api/runtime/memory/candidates/{id}/approve
POST   /api/runtime/memory/candidates/{id}/reject
POST   /api/runtime/memory/candidates/{id}/edit
GET    /api/runtime/memory/profile
GET    /api/runtime/memory/search
GET    /api/runtime/reviews/due
```

### 12.5 Interview

```text
POST   /api/interview-sessions
GET    /api/interview-sessions/{id}
POST   /api/interview-sessions/{id}/answers
POST   /api/interview-sessions/{id}/finalize
GET    /api/interview-sessions/{id}/trace
GET    /api/reports/{sessionId}
```

---

## 13. 实施阶段

### Phase 1: Provider + Skill

- 实现 DeepSeek Provider。
- 实现 OpenAI-compatible Provider。
- 实现 Embedding Provider。
- 实现 Skill Pack 扫描、解析、入库。
- 实现 Context Preview 初版。

### Phase 2: Context Engine

- 实现 ContextItem。
- 实现 ContextRecipe 配置。
- 实现 token budget。
- 实现上下文评分。
- 实现 prompt packing。
- 所有 Agent 调用记录 context trace。

### Phase 3: Retrieval Harness

- 文档解析、section 识别、semantic chunk。
- full-text index。
- embedding index。
- summary index。
- entity / relation / claim 抽取。
- retrieval planner。
- rag debug。

### Phase 4: Memory Candidate

- 面试结束后提取候选记忆。
- 全部进入 pending。
- 支持 approve / reject / edit。
- 支持 conflict detection。
- approved 后更新 profile projection。
- 实现 due review 生成。

### Phase 5: Interview Runtime

- 实现 session 状态机。
- 实现出题、答题、评分、追问、总结。
- 实现幂等提交。
- 实现 single-flight。
- 实现 Redis 热状态 + PostgreSQL snapshot。
- 每次 AI 调用写入 trace。

### Phase 6: Lightweight KG

- 从 Skill、简历、JD、知识库中抽取实体。
- 存入 `rag_entities`、`rag_relations`、`rag_claims`。
- RAG 检索时做 entity match 和 1-hop expansion。
- 暂不做 Neo4j、社区摘要、全量 GraphRAG。

### Phase 7: Evaluation Harness

- 准备固定简历、JD、Skill、知识库、问答样例。
- 评估不同检索策略：keyword only、vector only、hybrid、summary、entity expansion。
- 指标：召回命中率、引用覆盖率、上下文相关性、JSON 成功率、评分稳定性、成本、延迟。

---

## 14. 测试计划

### Provider Tests

- DeepSeek chat。
- OpenAI-compatible chat。
- Embedding API。
- Provider test endpoint。
- AI 调用失败降级。

### Skill Tests

- meta 解析。
- reference 加载。
- Skill index。
- context preview。

### Context Tests

- ContextRecipe 比例。
- token budget。
- ContextItem score。
- 不可信上下文隔离。
- 未审核记忆不进入上下文。

### RAG Tests

- keyword only。
- vector enabled。
- summary search。
- entity expansion。
- relation lookup。
- rubric retrieval。
- rag debug trace。

### Memory Tests

- pending。
- approve。
- reject。
- edit。
- conflict。
- profile projection。
- SM-2 due review。

### Runtime Tests

- 重复提交。
- 重复 finalize。
- AI 超时。
- Redis 丢失恢复。
- Python 失败不推进状态。

### Trace Tests

- 每次 Agent run 都能看到 prompt、context items、模型、输出、错误。
- 每条记忆和上下文分数都能解释来源、原因、分数。

---

## 15. 对外解释口径

### 15.1 画像数据怎么来的

标准回答：

> 画像数据来自用户的面试问答记录、上传的简历/JD、知识库文档、历史训练结果和人工审核后的画像条目。AI 不直接把判断写入最终画像，而是先基于具体证据生成候选记忆，用户审核确认后才进入长期画像。

### 15.2 记忆分数怎么来的

标准回答：

> 记忆权重不是 LLM 凭空学出来的，而是确定性 ranking score。它综合任务匹配度、证据强度、复习优先级、时间衰减、重复出现次数和用户审核状态。LLM 只负责提取候选观察，最终是否进入长期记忆和本次上下文，由规则和人工审核共同决定。

### 15.3 RAG 为什么不是普通向量库

标准回答：

> 这里不是简单的 pgvector topK，而是面向面试 Agent 的 Context Retrieval Harness。索引阶段同时构建 chunk、摘要、实体、关系、claim、rubric 和历史问题索引；查询阶段先判断任务类型，再选择 keyword、vector、summary、graph、skill、memory 等不同检索器。每个上下文片段都有来源、分数和选择理由，最终上下文可以 debug 和回放。

---

## 16. 明确不做的内容

第一版不做：

- SaaS 招聘协作。
- 组织、项目、候选人邀请。
- 强反作弊。
- 代码题和白板。
- 前端重构。
- LiteLLM。
- Neo4j。
- 全量 Microsoft GraphRAG。
- 大规模 Web Crawler。
- WebRTC。

保留扩展空间：

- `space_id` 预留未来团队空间。
- `ClaudeAdapter` 预留 Claude 接入。
- `SearchProvider` 预留外部搜索。
- `rag_entities/rag_relations` 可迁移到图数据库。
- Agent trace 可扩展为完整评估平台。

---

## 17. 里程碑建议

### M1: 后端最小闭环

完成：Provider、Skill、Context Preview、简单出题、简单评分。

验收：可以用 DeepSeek + Skill Pack 生成问题，并看到上下文来源。

### M2: RAG 可解释闭环

完成：full-text、embedding、summary、rag debug。

验收：每次 RAG 查询都能解释命中来源、分数和最终上下文。

### M3: 记忆审核闭环

完成：候选记忆提取、审核、画像投影、复习任务。

验收：面试结束后生成 pending memory，用户 approve 后进入下次上下文。

### M4: 面试运行时闭环

完成：session 状态机、幂等、single-flight、snapshot restore、报告。

验收：完整完成一次面试，重复提交不重复调用 AI，Redis 丢失可恢复。

### M5: Retrieval Harness 升级

完成：entity、relation、claim、rubric index、graph expansion。

验收：出题/评分/追问能利用 Skill、知识点关系、用户弱点、历史题去重。

---

## 18. 默认技术选择

```text
Backend API: Go
AI Runtime: Python
LLM: DeepSeek first
LLM Compatibility: OpenAI-compatible adapter
Embedding: 免费额度 API
Database: PostgreSQL + pgvector
Cache / Lock: Redis
Object Storage: MinIO / RustFS
Graph: PostgreSQL lightweight KG tables
Search: 默认关闭，预留 SearXNG / external search provider
Frontend: 暂不改
```

---

## 附录：参考来源索引

详细参考来源见：[`project-reference-map.md`](./project-reference-map.md)。
