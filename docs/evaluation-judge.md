# Evaluation Judge V2

Evaluation Harness V2 在现有确定性断言之上增加可选的 LLM-as-a-Judge，并支持 Human Golden Set 校准。Rule Evaluator 继续负责字段、格式和确定性回归；Judge 负责开放式答案的事实正确性、知识点覆盖度和技术深度；人工标注用于检查 Judge 与人的评分是否一致。

## 设计原则

- Rule Evaluator 不删除，用于低成本、可复现的基础回归。
- Judge 使用独立 `evaluation_judge` task route，便于与被测模型解耦。
- Judge 输入显式包含参考答案、关键知识点、常见错误和 rubric，而不是只要求模型“打一个总分”。
- Judge 必须返回结构化 JSON，至少包含 `total_score`、`dimensions`、`summary` 和 `fatal_error`。
- Evaluation Run 同时保留 `rule_score`、`judge_score`、最终分、权重和版本元数据，便于后续回放与比较。
- Golden Case 可额外配置人工分，Run 自动计算 Judge 偏差、绝对误差和容差命中情况。

## Case 示例

```json
{
  "case_id": "eval_redis_answer_001",
  "suite": "java-backend-golden",
  "task_type": "answer_evaluation",
  "skill_id": "java-backend",
  "input": {
    "user_input": "问题：Redis 为什么快？\n候选回答：Redis 基于内存，并通过 IO 多路复用降低网络等待开销。"
  },
  "expected": {
    "required_fields": ["score"],
    "judge": {
      "enabled": true,
      "reference_answer": "Redis 的性能来自内存访问、高效数据结构、事件驱动与 IO 多路复用等因素；线程模型需要结合版本和命令场景准确描述。",
      "key_points": [
        "基于内存访问",
        "IO 多路复用或事件驱动",
        "高效数据结构",
        "线程和锁开销需要准确描述"
      ],
      "common_errors": [
        "Redis 所有版本和所有操作都严格单线程",
        "Redis 快的主要原因只是因为使用 C 语言"
      ],
      "rubric": {
        "correctness": 40,
        "completeness": 30,
        "depth": 20,
        "clarity": 10
      },
      "rule_weight": 0.4,
      "judge_weight": 0.6,
      "pass_score": 70,
      "prompt_version": "judge.v1",
      "rubric_version": "redis-answer.v1"
    },
    "calibration": {
      "human_score": 82,
      "tolerance": 8,
      "annotation_version": "java-backend-golden.v1",
      "human_dimensions": {
        "correctness": 36,
        "completeness": 22,
        "depth": 15,
        "clarity": 9
      }
    }
  },
  "tags": ["redis", "llm-judge", "golden"],
  "status": "active"
}
```

如果 `rule_weight` 与 `judge_weight` 都未配置，默认使用 `0.4 / 0.6`。`pass_score` 默认 `70`。人工标注的 `calibration.tolerance` 默认 `10` 分；配置 calibration 时必须同时开启 Judge。

## Judge 输出

Judge Runtime 目标结构：

```json
{
  "total_score": 86,
  "dimensions": {
    "correctness": {
      "score": 37,
      "max_score": 40,
      "reason": "核心技术事实正确"
    },
    "completeness": {
      "score": 23,
      "max_score": 30,
      "reason": "未覆盖高效数据结构"
    },
    "depth": {
      "score": 17,
      "max_score": 20,
      "reason": "解释了 IO 多路复用，但线程模型仍可深入"
    },
    "clarity": {
      "score": 9,
      "max_score": 10,
      "reason": "表达清晰"
    }
  },
  "summary": "核心答案正确，但覆盖面仍可扩展。",
  "fatal_error": false
}
```

## 最终分

当 Judge 开启时：

```text
final_score = rule_score * rule_weight + judge_score * judge_weight
```

实际实现会对权重归一化，因此权重不要求和严格等于 1。最终分达到 `pass_score` 时 Run 为 `passed`，否则为 `failed`。

Run 的 `output` 中会记录：

- `runtime_response`
- `rule_score`
- `judge_response`
- `judge_score`
- `judge_trace_id`
- `scoring_mode`
- `score_breakdown`
- `calibration`（仅 Golden Case）

`score_breakdown` 同时记录 `prompt_version` 与 `rubric_version`，便于后续 Judge Prompt、Rubric 和模型版本发生变化时定位分数差异来源。

## Human Golden Set 校准

当 `expected.calibration.human_score` 存在时，Evaluation Run 会额外记录：

```json
{
  "human_score": 82,
  "judge_score": 86,
  "signed_error": 4,
  "absolute_error": 4,
  "tolerance": 8,
  "within_tolerance": true,
  "annotation_version": "java-backend-golden.v1",
  "prompt_version": "judge.v1",
  "rubric_version": "redis-answer.v1"
}
```

其中：

- `signed_error = judge_score - human_score`，正数表示 Judge 偏高，负数表示 Judge 偏低。
- `absolute_error` 用于衡量单条样例的评分误差。
- `within_tolerance` 表示 Judge 是否落在人工分允许的误差范围内。

Golden Set 的人工分应由人工阅读答案后给出，而不是由另一个模型生成。`annotation_version` 用于标记人工标注规则或数据集版本。

## 校准汇总

已有的 Run 查询接口支持附带校准汇总：

```text
GET /api/evaluation/runs?task_type=answer_evaluation&suite=java-backend-golden&include_calibration_summary=true
```

返回的 `calibration_summary` 包含：

- `sample_count`：参与校准的最新 Golden Run 数量。
- `mean_absolute_error`：Judge 与人工评分的平均绝对误差（MAE）。
- `mean_signed_error`：Judge 整体偏高或偏低的平均趋势。
- `within_tolerance_rate`：落在人工容差范围内的样例比例。
- `items`：每个 Case 最近一次具有 calibration 结果的 Run。

建议先用 20～50 条人工标注样例观察 MAE 和偏差方向，再迭代 Judge Prompt。不要为了让 Judge 贴合某一批样例而不断手工特化 Prompt；Golden Set 应保留一部分未用于调 Prompt 的验证样例。

## Dry Run

`dry_run=true` 时只验证被测 Runtime 调用链路，不执行第二次 Judge 模型调用，因此不会产生额外模型成本，也不会生成 calibration 结果。

## 后续演进

1. 将 Golden Set 拆分为 calibration / validation 两部分，避免 Judge Prompt 对测试集过拟合。
2. 增加 Pairwise Evaluator，用于比较 Prompt/Model V1 与 V2 的胜率。
3. 将 generator model、judge model、dataset version 独立版本化，形成稳定的离线回归基线。
4. 在样例规模扩大后补充相关系数、分桶误差等统计，进一步判断 Judge 与人工的一致性。
