---
name: RPG Lorekeeper + Streaming Narrative
overview: Streaming beat output + Lorekeeper role for translating narrative into structured world knowledge, addressing NPC consistency through persisted memories.
todos:
  - id: sub0
    content: Streaming infrastructure — BeatOutput dual channel + agent.Stream + REPL streaming print.
    status: completed
  - id: sub1
    content: Role refactor — drop Registrar interface, add Lorekeeper, lift Templates to package-level.
    status: completed
  - id: sub2
    content: narrator.LoreParser — Eino structured output → ingest.Draft.
    status: completed
  - id: sub3
    content: Session.RunBeat Lorekeeper integration with graceful degradation.
    status: completed
  - id: sub4
    content: Narrator prompt — render NPC memory section with truth-status markers and caps.
    status: completed
  - id: sub5
    content: CLI integration — lore footer in streamBeat + world-knowledge stats in cmdStatus.
    status: completed
isProject: false
---

# RPG Lorekeeper + Streaming — 实施记录

> 实施于 `feature/worldline-mvp` 分支，紧随 commit `a8bc14c`（WorldLine MVP）。
> 本文档是**执行记录**：每个 sub-task 记录实际改了哪些文件、引入了哪些类型 /
> 函数 / 常量、对应哪些 tests，以及若干非显然决策。设计依据见同名 spec
> [`docs/superpowers/specs/2026-05-27-rpg-lorekeeper-streaming-design.md`](../specs/2026-05-27-rpg-lorekeeper-streaming-design.md)。

**Commit Policy:** Do not commit during execution unless the user explicitly asks.
本批次 6 个 sub 全部完成，未自动 commit；交付时由人决定 squash / 分次提交。

## 完成范围

本轮把 RPG 主流水线打通了三件事：(a) 把 `session.RunBeat` 从"阻塞十几秒一次性
返回"改成 chunk-by-chunk 流式输出（Sub 0），(b) 引入新的 `role.Lorekeeper` 角色把
每回合叙事翻译成 `ingest.Draft` 并合并进 world 快照（Sub 1–3），(c) 把沉淀下来
的 NPC 记忆回灌到下一回合的 Narrator prompt，并在 CLI 把世界知识可视化
（Sub 4–5）。GM 接口顺手做了一次清理：删掉 `Registrar`，把列模板下放成包级函数。

入口验证（在 §"出口验证"小节）显示 6 个 sub 全部 `go test ./... -short` 通过、
`go vet ./...` 干净、`go build ./...` 干净。

## Subtask 详情

### Sub 0: 流式基础设施

**Files changed:**

- `rpg/session/session.go` — `BeatOutput`、`runBeatStream`、`runBeatPipeline`
  改造、`Stream()` 迁移
- `rpg/cli/beat.go` — `RunBeat` 命令改成消费 `NarrativeStream` + `<-Done`
- `cmd/rpg-cli/main.go` — 新增 `streamBeat` helper，prologue / recap / 主循环
  全部走流式
- `rpg/session/session_test.go` — 全部 `sess.RunBeat(...)` 调用改成
  `sess.RunBeat(...).Wait()`，零行为变化
- `e2e/rpg/e2e_test.go`、`e2e/rpg/worldline_e2e_test.go` —
  同上，e2e 也切换到 `.Wait()`

**Key constructs:**

- `session.BeatOutput { NarrativeStream <-chan string; Done <-chan BeatResult }`
- `session.BeatOutput.Wait() BeatResult` — 测试 / 脚本同步消费 helper
- `session.runBeatStream(ctx, input, narrativeCh, doneCh)` — goroutine 入口，
  唯一职责是 deferred close + done 写入顺序保证
- `session.runBeatPipeline(ctx, input, narrativeCh, *result)` — 原 RunBeat 主体
  逻辑搬过来；用 `result.Err` 表达失败、`return` 早退；channel 的关闭由外层
  defer 负责
- Eino 接入：`agent.Stream(ctx, messages)` 替代 `agent.Generate`；用
  `if len(chunk.ToolCalls) > 0 { continue }` 过滤 tool-call frame，
  `chunk.Content` 累积到 `narrativeBuf` 并 forward 到 `narrativeCh`

**Tests:**

- `TestRunBeat_FullPipeline`、`TestRunBeat_WithToolCalls`、
  `TestRunBeat_WithFog`、`TestRunBeat_WorldLine_DriftAndMilestone`、
  `TestRunBeat_WorldLineDisabled_NoFile` — 既有用例迁移到 `.Wait()`，无行为变化
- `mockChatModel.Stream(...)` 在 test fixture 里新增，配合 `Generate` 复用
  `callCount` 模拟两轮 ReAct（tool-call 一轮 + 终态一轮）

**Non-obvious decisions:**

- **NarrativeStream 必须先 close，再写 Done**。`runBeatStream` 的 defer 同时
  做这两件事并按顺序，保证 caller `for chunk := range out.NarrativeStream`
  正常退出后再 `<-out.Done`。先读 Done 是 documented misuse → 会死锁。
- **`Done` channel 用 size=1 buffered**，producer 永远不阻塞；caller 不读
  Done 也只是让 result GC 不掉，不会 hang goroutine。
- **`BeatResult.Narrative` 保留为字段**：流式拼接的全文同步反馈给 caller，
  Lorekeeper / EventLog summary / future 测试都直接用它而不必自己拼 chunk。
- **ctx 取消通过 select 进入 `case narrativeCh <- chunk.Content`**：缓冲满
  时 backpressure 正确；不会无限堆积也不会泄 goroutine。

### Sub 1: Role 接口重塑

**Files changed:**

- `rpg/role/gm.go` — 把 `Registrar` 子接口从 `GM` 里删除
- `rpg/role/lorekeeper.go` — **新建**，定义 `type Lorekeeper interface { ingest.Parser }`
- `rpg/narrator/templates.go` — **新建**，包级函数
  `AvailableTemplates() []role.WorldTemplate`
- `rpg/narrator/narrator_test.go` —
  `TestNarrator_Templates` 改成直接调包级函数；`TestNarrator_ImplementsGM`
  断言 `*Narrator` 仍满足新的 GM 接口（即 Persona + Rulebook + Director）

**Key constructs:**

- `role.Lorekeeper`（`rpg/role/lorekeeper.go`）— 仅嵌入 `ingest.Parser`，
  doc-comment 写明"failure semantics: log and continue; never abort the beat"
- `narrator.AvailableTemplates()` — 按 `template.TemplateNames()` 排序返回
  fantasy / mystery / scifi / modern 四个 `role.WorldTemplate`

**Tests:**

- `TestNarrator_Templates` — 断言长度 == 4，names 中存在四个预期模板
- `TestNarrator_ImplementsGM` — 编译期接口断言 `var _ role.GM = (*Narrator)(nil)`

**Non-obvious decisions:**

- **Lorekeeper 不是 GM 子接口**：spec §1.2 详述。可替换性 / 单一职责 / LLM
  边界三点都不允许耦合。签名故意复用 `ingest.Parser` 让任何已有 Parser
  自动满足 Lorekeeper、不写 adapter。
- **AvailableTemplates 是包级函数而非 Narrator 方法**：列模板是冷路径目录查询、
  不属于每回合 GM 契约，放方法上会污染 per-beat 接口。
- **删 `Registrar`** 不需要兼容 layer：`rpg/role/gm.go` 在 commit `ae2e188`
  之后没有 third-party 使用方，本仓 internal-only。

### Sub 2: `narrator.LoreParser` — Eino structured output → `ingest.Draft`

**Files changed:**

- `rpg/narrator/lorekeeper.go` — **新建**：
  `LoreParser` 类型 + `loreDraft` schema + 中文 system prompt
- `rpg/narrator/lorekeeper_test.go` — **新建**：
  fake chat model 覆盖 success / empty input / WithTools error / generate error
  / 接口断言

**Key constructs:**

- 私有 `loreDraft` struct：镜像 `ingest.Draft` 去掉 Canon；元素类型直接复用
  `ingest.DraftEntity / DraftRelation / DraftFact / DraftThread / DraftMemory`
- `LoreParser { chatModel model.ToolCallingChatModel }`
- `NewLoreParser(cm model.ToolCallingChatModel) *LoreParser`
- `(*LoreParser).Parse(ctx, doc ingest.SourceDocument) (ingest.Draft, error)`
- 编译期断言 `var _ role.Lorekeeper = (*LoreParser)(nil)`
- 中文 system prompt `lorekeeperSystemPrompt`：覆盖 ID 规范、Entity Type 范围、
  Relation Type / Fact Predicate 命名、Thread Status / Priority / Tension 取值、
  Memory Scope / Kind / OwnerKind、Confidence / TruthStatus / source_refs
- helper `buildLorePrompt(doc)` — user message 里附 "## 叙事文本" + "## 来源 ID"
  两段，让 LLM 在每个 source_refs 里填入 doc.ID 实现 trace

**Eino glue:**

1. `utils.GoStruct2ToolInfo[loreDraft]("record_lore", description)` → `*schema.ToolInfo`
2. `chatModel.WithTools([]*schema.ToolInfo{toolInfo})` → bound instance（新实例，不污染共享）
3. `bound.Generate(ctx, [system, user], model.WithToolChoice(schema.ToolChoiceForced))`
4. `schema.NewMessageJSONParser[loreDraft](&MessageJSONParseConfig{ParseFrom: MessageParseFromToolCall}).Parse(...)`
5. 字段拷贝成 `ingest.Draft` 返回

**Tests:**

- `TestLoreParser_ParseSuccess` — fake model 返回 tool-call JSON，断言
  Entities / Relations / Memories 三段字段正确转换
- `TestLoreParser_ParseEmpty` — `doc.Text = "   \n  "` 时短路返回零值 Draft，
  fake model `callCount == 0`
- `TestLoreParser_BindToolsError` — WithTools 返回错误时，Parse 包成
  `lorekeeper bind tool: %w`
- `TestLoreParser_GenerateError` — Generate 返回错误时，Parse 包成
  `lorekeeper generate: %w`
- `TestLoreParser_ImplementsRoleLorekeeper`、`TestLoreParser_ImplementsIngestParser`
  — 编译期接口断言

**Non-obvious decisions:**

- **`loreDraft` 故意没有 Canon 字段**：Canon 是世界级元数据（genre / tone /
  premise / laws / boundaries / secrets），由 world template 在创建时写入，
  不应该被每回合 LLM 重新猜测。schema 里不开口子，LLM 就没法误填。
- **空文本短路**：`strings.TrimSpace(doc.Text) == ""` 直接返回零值，无 LLM
  请求。这条让 prologue / recap / silent setup beat 零开销通过。
- **ToolCallingChatModel（不是 deprecated ChatModel）**：`WithTools` 返回**新**
  bound 实例，可与同一 chatModel 上的其他 caller（GM / SuggestActions）安全
  共存于并发；继续使用旧 `ChatModel.WithTools` 会原地变更状态。
- **error 包裹按阶段命名**（`build tool schema` / `bind tool` / `generate` /
  `parse`）：CLI footer 打出来直接能定位失败阶段。

### Sub 3: Session.RunBeat Lorekeeper 集成

**Files changed:**

- `rpg/session/session.go` —
  - `Config.Lorekeeper role.Lorekeeper`（optional）
  - `Session.lorekeeper` 字段
  - `BeatResult.LoreErr error`、`BeatResult.LoreReport ingest.CompileReport`
  - `runBeatPipeline` 在 WorldLine tick 之后、SaveSnapshot 之前调用
    `s.runLorekeeper(ctx, world, beatEvent.ID, narrative)`
  - **新增** `(*Session).runLorekeeper(ctx, world, sourceID, narrative)
    (worldmodel.World, ingest.CompileReport, error)` — Parse → Validate →
    Compile 三步流水，graceful-degrade 返回不变 world
- `rpg/session/session_test.go` — 新增 4 个 lorekeeper 用例 + `mockLorekeeper`
  fixture

**Key constructs / behaviors:**

- 调用顺序固定（spec §3.3）：load → tools → stream → ApplyEvent (beat event) →
  Sequence++ → **WorldLine tick** → **Lorekeeper** → SaveSnapshot → fog save
  → SuggestActions
- `SourceDocument.ID = string(beatEvent.ID)`、`Kind = "rpg_beat"`、`Text = narrative`
  — 让每条 lore item 通过 SourceRefs 都能溯源到 EventLog 单条 event
- `runLorekeeper` **失败语义**：返回 `(入参 world, ingest.CompileReport{}, wrappedErr)`，
  调用者即使忘了检查 err 直接赋值，也得到一个未变更的 world（safe-by-default）
- 当前 `CompileOptions { ConflictPolicy: ConflictPolicySkip, Resolver: nil }` —
  CompileDraft 内部回落到 `NoopAliasResolver`（spec §7 折中）
- `ValidateDraft.Errors / Warnings` 被收集进 `report.Notes`，前缀
  `validate-error:` / `validate-warn:`；不作为 hard abort

**Tests:**

- `TestRunBeat_LorekeeperDisabled` — `Config.Lorekeeper == nil` 时整段跳过；
  `LoreErr == nil`、`LoreReport == zero`、entity 数不变
- `TestRunBeat_LorekeeperSuccess` — `mockLorekeeper.draft` 含一个合法 entity，
  断言 Inserted == 1、`result.World` 与 `LoadSnapshot` 都看到 `ent_crystal_keeper`、
  `lk.lastDoc.ID == "beat_world-test-01_5"`（与 Clock.Sequence 对齐）、
  `lk.lastDoc.Kind == "rpg_beat"`、Text 非空
- `TestRunBeat_LorekeeperParseFails` — `mockLorekeeper.err = errors.New(...)`：
  `result.Err == nil`（不影响主流水）、`LoreErr` 包 `"lorekeeper parse"`、
  `LoreReport == zero`、Narrative 仍非空（graceful degrade）、世界仍持久化
  （Clock.Sequence 推进到 6）、无新 entity
- `TestRunBeat_LorekeeperReportNotesIncludeValidate` — 一好一坏两条 entity，
  断言 Inserted == 1、`report.Notes` 含 `validate-error:` 或 `validate-warn:`
  前缀

**Non-obvious decisions:**

- **Lorekeeper 必须在 WorldLine tick 之后、SaveSnapshot 之前**：让 draft 能引用
  本回合 milestone 触发的事件；又让成功的 lore 与本回合 world 一次原子落盘。
- **失败时 runLorekeeper 返回入参 world**（而不是 `worldmodel.World{}`）：
  safe-misuse 设计。当前 call site 仍然额外 gate 在 `if loreErr != nil`，
  让"成功才赋值 newWorld"这层意图保持显式可读。
- **`AliasResolver` 留 nil**：CompileDraft 回落 `NoopAliasResolver`，
  Lorekeeper 自己写出来的 entity ID 就是最终 ID。后续若出现重复 NPC 再做
  resolver。

### Sub 4: NPC Memory Prompt 段

**Files changed:**

- `rpg/narrator/prompt.go` —
  - 新增 `## NPC 记忆` 段位插入 `narratorSystemTemplate` 模板
  - 新增 `buildNPCMemorySection(entities, memories)`
  - 新增 `writeMemoryGroup`、`writeMemoryGroupOthers`、`renderMemoryLine`、
    `hasPromptTag` helper
  - 新增 const：`npcSectionMaxNPCs = 6`、`npcSectionMaxMemoriesPerNPC = 5`、
    `npcSectionMaxMemoryRunes = 200`、`npcLabelSummary = "长期记忆"`、
    `npcLabelObservation = "短期记忆"`、`npcLabelBelief = "信念"`、
    `npcLabelRumor = "传言"`、`npcLabelOthers = "(未分类)"`、
    `npcMarkerUntrusted = " (可能有误)"`、`npcMarkerDisputed = " (有争议)"`、
    `npcSectionEmpty = "(no NPC memories yet)"`
- `rpg/narrator/prompt_test.go` — **新建**，单元测试 9 例覆盖
  empty / 无 NPC 持有记忆 / 跳过 player 标签 / 按 Kind 分组 / TruthStatus
  marker / 单 NPC 5 条上限 / NPC 6 个上限 / 未分类 Kind 兜底 / SystemPrompt
  集成

**Key behaviors（已在 spec §4 详述）：**

- 入参：`opts.WorldCtx.Entities` + `opts.WorldCtx.Memories`（即 `WorldDebugContext`）
- 过滤：memory 仅保留 `Owner.Kind == "character"`；entity 仅保留 `Type == "character"`
  且 `tags` 不含 `"player"`
- 排序：每个 NPC 内部按 `Importance` desc、ID asc 二级排序（`sort.SliceStable`）
- 分组顺序：summary / observation / belief / rumor / 未分类
- TruthStatus 标记：`false`/`outdated` → `(可能有误)`；`disputed` → `(有争议)`；
  其他不打 marker
- 容量上限：6 NPC × 5 memory × 200 runes
- 空段返回 `(no NPC memories yet)`，模板段位标题不缩塌

**Tests:**

- `TestBuildNPCMemorySection_Empty` — 空 entity + 空 memory → `(no NPC memories yet)`
- `TestBuildNPCMemorySection_NoNPCsWithMemories` — 有 NPC 实体但没有 character-owned memory
  → `(no NPC memories yet)`
- `TestBuildNPCMemorySection_SkipsPlayerCharacters` — 带 `"player"` tag 的角色不出现
- `TestBuildNPCMemorySection_GroupsByKind` — 四类 Kind 各一条，断言渲染按
  summary → observation → belief → rumor 顺序、各自挂在正确 label 下
- `TestBuildNPCMemorySection_TruthStatusMarker` — `false`/`outdated`/`disputed`
  三种状态各自显示正确 marker、其余不打 marker
- `TestBuildNPCMemorySection_TruncatesPerNPC` — 单 NPC 6 条 memory 时截到 5 条，
  且按 Importance desc 优先保留高 importance 项
- `TestBuildNPCMemorySection_CapsNPCCount` — 7 个 NPC 各有 memory 时只渲染 6 个
- `TestBuildNPCMemorySection_OthersGroup` — Kind 空字符串或非法值 → 落在 `(未分类)`
- `TestSystemPrompt_IncludesNPCMemorySection` — 端到端，断言 prompt 字符串
  里 `## NPC 记忆` 段后是预期的 memory bullet 而不是 `(no NPC memories yet)`

**Non-obvious decisions:**

- **空段保留标题、内容 `(no NPC memories yet)`**：不让段位缩塌成隐式空白，
  避免 LLM 误把后续 `## Locations` 当作 NPC 记忆的延续。
- **label / marker 全部抽 const**：测试断言用同一份常量，避免全角/半角空格
  drift 让 prompt 与 assertion 静默脱钩。
- **仍按 `"player"` tag filter NPC**：当前 demo world (`buildDemoWorld`) 给
  `hero-wukong` 打 `"玩家"` 中文 tag、`hero-arin` 打英文 `"player"` tag；
  这两套混用阶段先用 tag 黑名单兜底。未来 seed 统一后可以删 filter。
- **TruncateRunes 是 200 runes 而不是 bytes**：中文每字 3 bytes，按 bytes
  截会出现半字符 — UTF-8 不可见 garbage 会让 deepseek 拒绝整段 prompt。
- **未分类 Kind 也渲染（不丢弃）**：当 Lorekeeper 偶尔写出非标准 Kind 时，
  让信息仍然出现在 prompt 里、只是带 `(未分类)` label，便于事后回溯而非
  silently 蒸发。

### Sub 5: CLI 接入

**Files changed:**

- `cmd/rpg-cli/main.go` —
  - `cmdPlay`：新建 `lk := narrator.NewLoreParser(chatModel)` 并注入
    `session.Config.Lorekeeper`
  - `streamBeat`：BeatResult footer 增加 LoreErr 行 + `summarizeLoreReport`
    单行（任一非零时打印）
  - **新增** `summarizeLoreReport(r ingest.CompileReport) string`
  - **新增** `npcMemoryStat { Name; ID; Count }`
  - **新增** `topNPCsByMemoryCount(world, n) []npcMemoryStat`
  - **新增** `printWorldKnowledge(w)` — `cmdStatus` 末尾调用
- `cmd/rpg-cli/main_test.go` — 新增 5 个测试覆盖
  `summarizeLoreReport` / `topNPCsByMemoryCount`（已有 `TestBuildCombo` /
  `TestIsAllDigits` 不动）

**Key CLI behaviors:**

- `streamBeat` footer 顺序：`回合=N 效果=M 张力=...` → 可选 `(行动建议失败：…)`
  → 可选 `(知识沉淀失败：…)` 或 `沉淀: 插入=… (含 X 条提示)?` → 可选行动菜单
- `summarizeLoreReport` "全零空" 时返回 `""`：CLI 据此**不打印**整行，避免
  prologue / silent beat 把 REPL 刷成 `沉淀: 插入=0 跳过=0 …` 噪声
- `printWorldKnowledge` 整段在 `len(Entities) == 0 && len(Memory) == 0`
  时跳过（fresh seed 不打 "世界知识" 标题）
- 实体计数：character / location / item 三类显式列出；其余归 `其他`，且
  `(其他=N)` 仅在 N > 0 时打出；末尾 `总计=N`
- 记忆计数：world / character 两类显式列出；其余（narrator / faction）归 `其他`，
  同样 > 0 才打 `(其他=K)`
- NPC 记忆 Top：调 `topNPCsByMemoryCount(w, 5)`；零结果时整段省略；
  显示 `- 名字 (ID): N 条`
- `topNPCsByMemoryCount` 内部：只统计 `Owner.Kind == "character"` 且 `Owner.ID != ""`
  的记忆，count == 0 不入 stats；排序 Count desc → ID asc；n > 0 时裁剪到前 n

**Tests:**

- `TestSummarizeLoreReport_Empty` — 全零空 → `""`
- `TestSummarizeLoreReport_Basic` — Inserted=2 / Skipped=1 / Rejected=0 / Filtered=0
  → `沉淀: 插入=2 跳过=1 拒绝=0 过滤=0`
- `TestSummarizeLoreReport_WithNotes` — 同上 + Notes 非空 →
  `... (含 1 条提示)`
- `TestTopNPCsByMemoryCount_Empty` — world.Memory 空 → nil
- `TestTopNPCsByMemoryCount_OrderAndCap` — 5 NPC × 不同 count，断言
  desc/ID-tiebreak 排序、cap=3 时返回前 3
- `TestTopNPCsByMemoryCount_OnlyCharacterOwned` — world-owned / faction-owned
  / narrator-owned 三种记忆都不计入任何 NPC tally

**Non-obvious decisions:**

- **LoreParser 复用 GM 的 chatModel**：Eino `WithTools` 返回新实例 → 共用安全；
  避免 CLI 启动时构造两个 deepseek client、占两份连接池 / API 配额。
- **summarizeLoreReport 全零空返回空字符串 → 调用方不打整行**：用户感知优先
  于"始终结构化"。每回合都打 `沉淀: 插入=0 ...` 会把 REPL 刷成噪声。
- **printWorldKnowledge 在没数据时整段跳过**：fresh seed 后还没跑过任何 beat 时，
  `status` 应该聚焦在 Threads / WorldLines / 最近事件，不该出现一行
  `实体: character=0 location=0 item=0 总计=0`。
- **(其他=N) 仅在 N > 0 时打**：常量分母 0 不暴露给用户，输出始终保持紧凑。
- **NPC 记忆 Top 用 character-owned memory count 而不是"NPC 出场次数"**：
  和 Lorekeeper 的"沉淀"指标对齐 — 我们想知道"世界对哪个 NPC 形成的印象最多"，
  而不是"哪个 NPC 露脸最多"。

## 出口验证

按 spec § "Verification Steps" 顺序执行：

- `go test ./... -short` — PASS（含本轮新增的 lorekeeper / prompt /
  summarizeLoreReport / topNPCsByMemoryCount 全部用例）
- `go vet ./...` — 干净
- `go build ./...` — 干净
- 手动验证（`cmd/rpg-cli`）：
  - `rpg-cli play --workspace … --world-id xiyou-changan` 进入 REPL，
    叙事 chunk-by-chunk 滚动；底部出现 `沉淀: 插入=… (含 N 条提示)?` 行
  - `rpg-cli status --workspace … --world-id xiyou-changan` 末尾出现
    "世界知识" 段，含实体/记忆计数与 NPC 记忆 Top
  - 连跑两回合后 `## NPC 记忆` 段在第二回合的 prompt 里可见（通过添加临时
    `--debug-prompt` 打印或在 `--world-id` 上重新跑 status 后核对 memory 数）

## 未做（推迟）

- **ConflictPolicy Replace 切换 + Resolver 注入**：当前 `ConflictPolicySkip` +
  `NoopAliasResolver` 让重名 entity 直接跳过；剧情需要"升级 / 改名 / 替身"时
  要切。未来按场景动态选 policy。
- **记忆遗忘衰减 / 短→长期晋升**：所有 memory 一旦入库就永远在 `world.Memory`。
  需要按 importance × age 做被动衰减、observation → summary 主动提升。
- **NPC 多轮自主行动**：当前 Narrator 一次 ReAct 输出涵盖所有 NPC 的反应；
  多 NPC 各自流式仍在路上，目前因 token budget 还是单调用。
- **时间感知粒度**：玩家显式 "快进半天" / "慢入一拍" 没有 first-class 支持，
  靠 WorldLine drift 推进。等 PerceptionWindow 落地后再做。
- **StoryPack YAML loader**：WorldLine 二轮的 follow-up，本轮不动。
- **PerceptionWindow**：同上，WorldLine 二轮的 follow-up，本轮不动。
