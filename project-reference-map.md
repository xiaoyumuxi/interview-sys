# 项目参考来源索引

## 1. 目的

这份文档记录个人 AI 面试训练平台计划的参考来源。由于参考项目分散在不同工作区，后续阅读 `ai-interview-backend-plan.md` 和 `ai-interview-roadmap.md` 时，可以通过本文快速找到每个设计点对应的本地项目和文件。

本索引只记录参考来源和可借鉴点，不表示直接复制实现。

---

## 2. 本地参考项目

### 2.1 AI-Meeting

路径：

```text
/Users/yaoyao/Documents/GitHub/AI-Meeting
```

主要参考点：

- 面试运行时。
- 追问推进。
- 答题幂等。
- Single-flight。
- Redis / Mongo 状态恢复思想。
- Agent 工作流契约。
- 面试报告和问答回放。

重点参考文件：

```text
README.md
skills/xunzhi-interview-domain/references/workflow-contracts.md
skills/xunzhi-interview-domain/references/state-machine.md
skills/xunzhi-interview-domain/references/answer-pipeline.md
skills/xunzhi-interview-domain/references/restore-and-finalize.md
skills/xunzhi-ai-runtime/references/ai-singleflight.md
skills/xunzhi-ai-runtime/references/ai-guard.md
skills/xunzhi-agent-domain/references/agent-binding.md
admin/src/main/java/com/hewei/hzyjy/xunzhi/interview/flow/session/InterviewSessionFacade.java
admin/src/main/java/com/hewei/hzyjy/xunzhi/interview/flow/session/InterviewAgentOrchestrationService.java
admin/src/main/java/com/hewei/hzyjy/xunzhi/interview/application/guard/singleflight
admin/src/main/resources/workflow
admin/src/main/resources/liteflow/interview-followup-chain.xml
```

在新计划中的落点：

```text
Interview Runtime
Idempotency
Single-flight
Runtime Snapshot
Agent Flow Contract
Follow-up Decision
Final Report
```

---

### 2.2 AI-Meeting-Frontend

路径：

```text
/Users/yaoyao/Documents/GitHub/AI-Meeting-Frontend
```

主要参考点：

- 当前 AI-Meeting 已有前端交互形态。
- 面试页面。
- 报告页面。
- 简历上传和预览。
- ASR/TTS 交互。
- 摄像头和神态分析入口。
- 草图板。
- 会话恢复相关 Hook。

重点参考文件：

```text
README.md
src/pages/interview/InterviewPage.tsx
src/pages/interview/InterviewReportPage.tsx
src/pages/interview/InterviewReportDetailPage.tsx
src/components/interview/report/InterviewQaReplayCard.tsx
src/components/interview/report/InterviewRadarChart.tsx
src/components/interview/InterviewResumeUploadCard.tsx
src/components/interview/InterviewResumeReferenceCard.tsx
src/hooks/interview/session/useInterviewSessionFlow.ts
src/hooks/interview/session/useInterviewRouteRecovery.ts
src/hooks/audio/useAudioTranscriptionController.ts
src/services/interviewService.ts
src/services/audioToTextWs.ts
```

在新计划中的落点：

```text
未来前端接入
Interview UI Contract
Report API Shape
ASR/TTS Future Extension
Session Recovery UX
```

当前计划中暂不改前端，但 API 设计需要兼容未来接入。

---

### 2.3 interview-guide

路径：

```text
/Users/yaoyao/Documents/GitHub/interview-guide
```

主要参考点：

- Skill 驱动出题。
- `SKILL.md + skill.meta.yml` 的 Skill Pack 组织方式。
- 多 Provider 管理。
- 知识库/RAG 基础实现。
- 简历、JD、语音面试、面试安排等产品模块。
- 统一评估服务。
- 结构化输出重试。
- Redis Stream 异步任务。

重点参考文件：

```text
README.md
AGENTS.md
app/src/main/resources/skills/java-backend/SKILL.md
app/src/main/resources/skills/java-backend/skill.meta.yml
app/src/main/resources/skills/_shared/references
app/src/main/java/interview/guide/common/ai/StructuredOutputInvoker.java
app/src/main/java/interview/guide/common/ai/LlmProviderRegistry.java
app/src/main/java/interview/guide/common/evaluation/UnifiedEvaluationService.java
app/src/main/java/interview/guide/modules/interview/service/InterviewSessionService.java
app/src/main/java/interview/guide/modules/interview/service/InterviewQuestionService.java
app/src/main/java/interview/guide/modules/knowledgebase/service/KnowledgeBaseQueryService.java
app/src/main/java/interview/guide/modules/knowledgebase/service/KnowledgeBaseVectorService.java
app/src/main/java/interview/guide/modules/llmprovider/service/LlmProviderConfigService.java
app/src/main/java/interview/guide/modules/voiceinterview/service/VoiceInterviewService.java
```

在新计划中的落点：

```text
Skill Context System
Provider Config
Structured Output Invoker
Knowledge Base Ingestion
Unified Evaluation
Async Task Pattern
```

需要注意：

`interview-guide` 的 RAG 方案偏普通向量检索，容易被质疑，因此新计划只借鉴其知识库基础能力和 Skill 组织方式，不直接照搬 RAG 设计。

---

### 2.4 TechSpar

路径：

```text
/Users/yaoyao/Documents/GitHub/TechSpar
```

主要参考点：

- 长期用户画像。
- 掌握度。
- 弱点/强项。
- 行为模式。
- SM-2 复习调度。
- LangGraph 面试状态机。
- JD 备面。
- 录音复盘。
- Copilot 策略树。
- 训练闭环。

重点参考文件：

```text
README.md
docs/graph.md
docs/profile-retrospective.md
docs/training-results.md
docs/special-training.md
docs/jd-preparation.md
docs/recording-review.md
backend/graphs/resume_interview.py
backend/graphs/topic_drill.py
backend/graphs/job_prep.py
backend/graphs/review.py
backend/memory.py
backend/spaced_repetition.py
backend/vector_memory.py
backend/user_context.py
backend/copilot/strategy_tree.py
backend/copilot/answer_advisor.py
backend/copilot/interview_monitor.py
backend/copilot/asr_dedup.py
```

在新计划中的落点：

```text
Memory Candidate
Profile Projection
Behavior Signal
SM-2 Review
Training Loop
LangGraph Flow Reference
Copilot / Strategy Tree Future Reference
```

新计划的重要调整：

TechSpar 中的画像更新更偏自动化。新计划改成：AI 只生成候选记忆，必须人工审核后才进入长期画像。

---

### 2.5 aural-oss

路径：

```text
/Users/yaoyao/Documents/GitHub/aural-oss
```

主要参考点：

- 招聘评估产品形态。
- 组织/项目/候选人。
- 邀请链接。
- 面试模板。
- 代码题。
- 白板。
- 反作弊。
- 语音、聊天、视频面试。
- 活动追踪和 session scoring。

重点参考文件：

```text
README.md
src/server/routers/interview.ts
src/server/routers/session.ts
src/server/routers/candidate.ts
src/server/routers/organization.ts
src/lib/ai/generator-run.ts
src/lib/ai/prompts/generator.ts
src/lib/ai/prompts/interviewer.ts
src/lib/session-score.ts
src/hooks/use-anti-cheating.ts
src/components/interview/question-builder.tsx
src/components/interview/candidate-manager.tsx
src/components/code-editor/code-editor-canvas.tsx
src/components/whiteboard/whiteboard-canvas.tsx
```

在新计划中的落点：

```text
Future SaaS Extension
Candidate / Invite Future Design
Anti-cheating Future Design
Coding Question Future Design
Whiteboard Future Design
Session Score Reference
```

当前计划不做 SaaS，但保留 `space_id` 方便未来扩展。

---

## 3. 外部设计参考

### 3.1 Microsoft GraphRAG

参考点：

- 普通 topK RAG 在全局理解和复杂关系推理上不足。
- 图谱和社区摘要可以提升大语料的整体理解能力。
- 但完整 GraphRAG 成本较高，不适合第一版全量接入。

在新计划中的落点：

```text
Retrieval Harness
Lightweight KG
Entity / Relation / Claim
Graph Expansion
```

本项目策略：

```text
不直接引入完整 GraphRAG。
先用 PostgreSQL 表做轻量实体、关系、claim。
只做 1-hop / limited 2-hop expansion。
```

### 3.2 RAPTOR

参考点：

- 长文档不应只切短 chunk。
- 需要章节摘要、文档摘要、主题摘要等层级信息。

在新计划中的落点：

```text
chunk_summary
document_summary
topic_summary
summary_index
```

### 3.3 HyDE

参考点：

- 对短 query 或模糊 query，可生成 hypothetical document 辅助召回。
- HyDE 结果不能直接作为事实使用，只能作为检索 query。

在新计划中的落点：

```text
Retrieval Planner future enhancement
Complex Query Expansion
```

第一版默认不启用 HyDE，后续可选。

### 3.4 RAG-Fusion / Multi-query

参考点：

- 多 query 可以提高召回，但会增加成本和延迟。
- 在 rerank 和 token budget 下，效果未必稳定提升。

在新计划中的落点：

```text
Query Router
Complex Query only multi-query
```

第一版默认不启用多 query，只在复杂/模糊问题下预留。

---

## 4. 设计点到参考来源映射

| 新计划设计点 | 主要参考来源 |
| --- | --- |
| 面试状态机 | AI-Meeting, TechSpar |
| 答题幂等 | AI-Meeting |
| Single-flight | AI-Meeting |
| Agent 工作流契约 | AI-Meeting |
| Skill Pack | interview-guide |
| Provider 配置 | interview-guide |
| 结构化输出重试 | interview-guide |
| 用户画像 | TechSpar |
| 行为信号 | TechSpar |
| SM-2 复习 | TechSpar |
| Copilot 策略树 | TechSpar |
| 招聘 SaaS 预留 | aural-oss |
| 反作弊预留 | aural-oss |
| 代码题/白板预留 | aural-oss |
| Retrieval Harness | GraphRAG / RAPTOR / 自定义轻量化 |
| 轻量 KG | GraphRAG 思路轻量化 |
| Context Preview | 结合 Agent trace 需求自定义 |
| Memory Review | TechSpar 自动画像方案的安全化改造 |

---

## 5. 推荐阅读顺序

如果后续工程师或 Agent 要继续执行，建议按这个顺序阅读：

1. `/Users/yaoyao/Documents/SelfProject/ai-interview-backend-plan.md`
2. `/Users/yaoyao/Documents/SelfProject/ai-interview-roadmap.md`
3. 本文档 `/Users/yaoyao/Documents/SelfProject/project-reference-map.md`
4. `AI-Meeting` 的 `skills/xunzhi-interview-domain/references/workflow-contracts.md`
5. `interview-guide` 的 `app/src/main/resources/skills/java-backend/SKILL.md`
6. `TechSpar` 的 `backend/memory.py`
7. `TechSpar` 的 `backend/spaced_repetition.py`
8. `TechSpar` 的 `backend/graphs/resume_interview.py`
9. `aural-oss` 的 `README.md`

---

## 6. 注意事项

- 本项目不是直接合并多个仓库，而是抽象它们的设计优点。
- `interview-guide` 的 RAG 方案不要直接照搬，需要升级为 Retrieval Harness。
- `TechSpar` 的画像方案不要直接自动写入，需要加入人工审核。
- `AI-Meeting` 的 Mongo snapshot 思路可以迁移到 PostgreSQL JSONB。
- `aural-oss` 的 SaaS 能力当前只作为未来扩展，不进入 MVP。
- 外部 GraphRAG/RAPTOR/HyDE/RAG-Fusion 只作为设计参考，不代表第一版直接引入对应框架。
