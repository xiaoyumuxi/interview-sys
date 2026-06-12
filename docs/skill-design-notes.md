# Skill 设计笔记

## 当前结论

Skill Pack 要保持“小而明确”，用于驱动可解释上下文，而不是把所有提示词都塞进一个大 Prompt。

推荐结构：

```text
skills/{skill_id}/
  skill.meta.yml
  SKILL.md
  references/
    topic-a.md
    topic-b.md
```

## 设计规则

- `skill.meta.yml` 负责检索和选择：`id`、`displayName`、`description`、`categories`、`priority`、`ref`。
- `SKILL.md` 负责行为约束：角色、考察目标、提问顺序、追问原则、评分倾向、禁止事项。
- `references/` 负责事实和 rubrics：知识点、评分标准、常见误区、追问路径。
- description 要短、清楚、有边界，便于后续隐式匹配。
- 每个 Skill 只服务一个主要岗位或训练场景。
- procedural 内容尽量写成“输入 -> 步骤 -> 输出”的确定流程。
- 所有上下文片段都要保留 source、score、reason，避免进入 Prompt 后不可追踪。
- 第三方 Skill 或用户上传 Skill 不能直接信任，要做 schema 校验、来源标记和禁止越权指令检查。

## 对本项目的落地

- `java-backend` 继续作为第一版验收 Skill。
- P1 先做本地扫描和 Context Preview，不急着引入复杂 marketplace。
- P2 再把 references 入库，建立 full-text、summary、vector 多索引。
- 后续可以增加 `skill.lint`，检查：缺少分类、引用文件不存在、description 过长、禁用事项缺失、Prompt 注入风险。

## 参考来源

- OpenAI Codex Agent Skills：Skill 是包含 instructions、resources 和可选 scripts 的目录；采用 progressive disclosure；`SKILL.md` 需要 name 和 description；最佳实践包括 focused skill、明确输入输出、测试触发描述。
- DeepSeek API：OpenAI format base URL 为 `https://api.deepseek.com`，Chat endpoint 为 `/chat/completions`，当前模型为 `deepseek-v4-flash` / `deepseek-v4-pro`。
- Agent Skill 近期研究提示：Skill 检索和选择在真实大规模集合里会变脆弱，且 `SKILL.md` 可能成为语义供应链攻击面，所以需要简洁 metadata、质量检查和安全治理。

参考链接：

- https://api-docs.deepseek.com/
- https://developers.openai.com/codex/skills
