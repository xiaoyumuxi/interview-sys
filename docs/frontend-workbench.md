# 前端工作台

`frontend` 是面向用户的训练工作台，不是后端 API 调试页。它使用 Vanilla TypeScript、CSS、Vite 和少量图标依赖，目标是在不引入重型前端框架的前提下，把 Go Core API、Worker、Python Runtime 和 Evaluation Harness 串成可操作的训练流程。

## 当前页面

| 页面 | 目的 | 主要交互 |
|---|---|---|
| 工作台 | 给用户一个全局入口和运行概览 | 查看 API、队列、outbox、judge 状态；跳转到面试、代码题和 memory review |
| 面试 | 以直播会议房间的方式完成一次异步面试训练 | 主舞台展示 AI 面试官和题目，参会者小窗展示候选人/Runtime，底部控制条提供静音、摄像头、笔记、共享题面和结束入口；右侧面板管理 session、trace 和 report |
| 代码题 | 练习并提交完整程序到 judge | 浏览题库、选择题目、编辑代码、提交判题、查看异步 verdict |
| 记忆 | 审核 Runtime 产出的候选记忆 | 加载 pending candidates、approve/reject，避免未审核 memory 进入 Prompt |
| 管理 | 查看系统运维和配置状态 | 查看 Provider route、Provider 列表、worker summary、coding judge summary |
| 评测 | 维护轻量质量样例 | 保存 evaluation case、dry-run 执行、查看 run 记录和最近结果 |

## 交互原则

前端按用户任务设计，而不是按 API endpoint 堆按钮：

- 状态可见：顶部 interaction strip 显示当前页面是 `Ready`、`Working` 还是 `Needs attention`，并给出下一步动作。
- 错误预防：表单提交前做基础校验，空答案、空代码、非法追问次数和空评测输入不会直接发请求。
- 可恢复：回答和代码编辑会同步到前端 state，避免普通重渲染时丢失用户输入。
- 操作反馈：请求进行中时按钮禁用，并把文案改成 `Updating`、`Creating`、`Sending`、`Submitting` 或 `Saving`。
- 空状态可行动：无题目、无候选记忆等状态会给出明确说明和可点击动作。
- 危险动作确认：结束面试会话前使用确认对话，避免误触。
- 双语可切换：语言按钮直接切换中文 / 英文，并保存到 `localStorage`。
- 不绕过 Go：前端只通过 Go API 读写业务事实，不直接推进 Python Runtime 或数据库状态。

这些原则对应 Nielsen Norman Group 的可用性启发式中“系统状态可见、错误预防、识别而非记忆、帮助用户恢复错误”等基础要求。

## 面试房间交互

面试页按“会议房间 + 后端状态机”的方式组织，而不是简单表单：

- 主舞台：展示 AI 面试官、当前题目、session/flow/turn 状态和候选人视图；顶部房间仪表条集中展示 room、flow、dry-run/real runtime 和 trace 数量。
- 字幕层：在舞台下方保留直播字幕胶囊，当前复用题面或空状态文案，后续可接 ASR/TTS 实时转写。
- 控制条：麦克风、摄像头、字幕和共享题面都是可点击的本地状态控件，状态保存到 `localStorage`，用于后续接入真实音视频、ASR/TTS 或题面共享 API。
- 发言面板：候选人回答区按直播发言控制台设计，展示本地麦克风/摄像头状态、回答字数、当前轮次和 dry-run/real runtime 模式。
- 会话回放：历史 turn 以会议记录时间线呈现，包含候选人回答摘要、Runtime 分数、追问信号、错误提示和 trace 句柄。
- 右侧 Companion 面板：提供简报、成员和笔记三个 tab。简报汇总当前题目和本地控制状态；成员展示 AI 面试官、候选人和 Runtime；笔记区域先本地保存，后续可接后端 notes API。
- 后端事实：创建 session、提交答案、轮询 trace、生成 report 和结束会话仍全部走 Go API，前端不会自行推进业务状态。

当前会议控件是轻量状态层，目的是先把用户操作路径、状态反馈和布局稳定性做出来；真正的音视频采集、字幕转写和共享题面同步会作为独立接口继续接入。

## 本地启动

前端需要 Node.js 运行 Vite：

```bash
make run-frontend
```

默认监听 `5173`。如果端口被占用，Vite 会自动选择后续端口。

建议同时启动：

```bash
make docker-up
make init-db
make run
make run-worker
make run-runtime
```

说明：

- 登录、题库、Provider、memory、evaluation 等 API 依赖 Go API。
- 面试异步评估和代码判题依赖 worker。
- 非 dry-run 的 Runtime task 和 memory API 依赖 Python Runtime。
- `make run-frontend` 会通过 Vite dev server 把 `/api` 代理到 Go API。

## 构建与验证

```bash
make build-frontend
```

该命令执行：

- `tsc --noEmit`
- `vite build`

前端改动至少应运行 `make build-frontend`。只改文案或文档时可以不跑完整 Go/Python 测试。

## 当前限制

- 还不是完整产品级前端：memory、admin 和 evaluation 的细节视图仍需要继续产品化。
- 目前没有路由库，页面切换由本地 state 和 `localStorage` 管理。
- 没有引入复杂组件库，样式集中在 `frontend/src/styles.css`。
- report、trace、worker summary 等仍保留部分 JSON 预览，后续应继续转成更适合用户阅读的结构化视图。
- 会议房间的麦克风、摄像头、字幕和共享题面当前是本地可配置状态，还没有接入真实设备、ASR/TTS 或同步服务。
- 暂未实现键盘快捷键、批量操作、搜索过滤和可配置 dashboard。

## 后续方向

1. 把 Memory Review 做成队列式审核体验，支持候选详情、风险标签和批量处理。
2. 把 Interview Report 做成正式报告页，减少 raw JSON 暴露。
3. 给直播面试房间补真实音视频/ASR/TTS 接口契约，当前麦克风、摄像头和共享题面是会议式状态控件。
4. 给 Evaluation Harness 增加 assertion 明细、失败解释和批量回归汇总。
5. 给 Admin 增加 Provider route 编辑、连通性测试入口和 worker health drill-down。
6. 增加前端 smoke 测试，覆盖登录页渲染、语言切换、导航和关键按钮状态。
