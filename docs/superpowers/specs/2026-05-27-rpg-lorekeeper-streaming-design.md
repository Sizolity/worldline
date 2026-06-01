# RPG Lorekeeper + Streaming Narrative Design

> Status: implemented (2026-05-27, on branch `feature/worldline-mvp`).
> 紧随 RPG Role System (commit `ae2e188`) 和 WorldLine MVP (commit `a8bc14c`) 之后落地，
> 不引入新的 PR boundary — 全部 sub-task (0–5) 已在工作分支上完成。
>
> 本文档是**设计实录**：描述已落地的接口、流水线、提示词与 CLI 行为，并解释为什么
> 这样切分。具体的 task / file 级落点见同名 plan 文档
> [`docs/superpowers/plans/2026-05-27-rpg-lorekeeper-streaming.md`](../plans/2026-05-27-rpg-lorekeeper-streaming.md)。

## 0. 背景与动机

WorldLine MVP 落地后，剧情节奏与世界状态推进已经稳定。但在真实 LLM 对局中浮现出
**三类用户感知不可接受**的问题，本轮一并修复：

1. **叙事不可流式**：`session.RunBeat` 同步阻塞十几秒后才一次性吐出整段文本，
   玩家在终端面对一个静止的"推演中…"提示，体感像是 LLM 卡死。
2. **NPC 记忆缺失 → 剧情不连续**：在西游 demo 的两条典型回合里 — 白骨夫人化身白素诱
   惑悟空、慧心老僧第二次出现接待师徒 — LLM 完全"忘掉"前一次接触的细节，把同一个
   NPC 当成第一次登场处理。世界里其实存在 EntityLog，但 NPC 的"印象"从未沉淀。
3. **LLM 凭训练记忆"重新发明"角色**：哪怕 EntityLog 里写了"白骨夫人喜欢用白素之名
   伪装"，叙事仍然按模型对原著的二手印象写。世界知识从未被结构化抽取并回灌进
   prompt，整段对局没有"读到自己已经沉淀过的事实"。

三个问题共享一个底层缺口：**没有一个 role 专门负责"把刚刚发生的故事翻译成
结构化的世界知识"，也没有任何 prompt 段位真的展示这些知识**。本设计补齐这两层，
并顺手把第一层流式 UX 一起做掉。

## 1. 角色边界

### 1.1 GM 接口简化

旧形态（commit `ae2e188` 即 RPG Role System 初版）：

```go
type GM interface {
    Persona
    Rulebook
    Director
    Registrar // List world templates
}
```

新形态（本轮）：

```go
type GM interface {
    Persona
    Rulebook
    Director
}
```

- `Registrar` 接口被**删除**。"列出可用 world template"这件事不属于每回合的 GM 契约 —
  它是冷路径目录查询，在 seed / setup 时调用一次。继续放在 GM 上是污染 per-beat 接口。
- 取而代之的是包级函数 `narrator.AvailableTemplates() []role.WorldTemplate`
  （`rpg/gm/narrator/templates.go`），它直接从 `rpg/template` 包按
  `template.TemplateNames()` 顺序读取四种模板（fantasy / mystery / scifi / modern），
  CLI 在 setup 时按需调用。
- 与此同时，新增独立的 `role.Lorekeeper interface { ingest.Parser }`
  （`rpg/role/lorekeeper.go`）。它**不是** GM 的子接口，原因见 §1.2。

### 1.2 为什么 Lorekeeper 不是 GM 的一部分

GM 与 Lorekeeper 都需要 LLM 调用，但职责正交：

- **GM** 在 beat 内部驱动 ReAct 循环，输出**用户可见**的叙事文本，受
  `## Recent Events / ## NPC 记忆 / ## Locations` 等 prompt 段位约束。
- **Lorekeeper** 在 beat **结束之后**被动接收一段已经完成的叙事，把它翻译成
  `ingest.Draft`。它对玩家不可见，输出**完全不是文本**，是结构化条目。

把这两者揉在一个接口里会立刻引发三个问题：

1. **可替换性**：未来一定会出现"GM 用本地小模型、Lorekeeper 用云端大模型"或反过来的
   场景；接口耦合就必须捆绑切换。
2. **单一职责**：GM 接口里塞 `Parse` 之后，任何"只想看 GM 怎么写 prompt"的 reader
   都会被 ingest 的细节 (`ingest.Draft`, `ingest.SourceDocument`) 污染。
3. **LLM 边界**：GM 的每个方法对 LLM 的依赖在 spec §2.4 里有明确表
   (Tools 不调 LLM / Judge 不调 LLM)，Lorekeeper 是"100% 必须调 LLM"，性质不同。

接口签名故意复用 `ingest.Parser` 的方法集，因此**任何已经实现 `ingest.Parser` 的
类型 free 满足 `role.Lorekeeper`**，无需 adapter；`narrator.LoreParser`
正是这样同时被 `ingest.ImportFile` 和 `session.Config.Lorekeeper` 使用。

## 2. 流式叙事基础设施

### 2.1 BeatOutput 双通道

`session.RunBeat` 不再返回 `(BeatResult, error)`，而是返回一个含两个 channel 的
`BeatOutput`：

```go
type BeatOutput struct {
    NarrativeStream <-chan string    // chunk-by-chunk delta
    Done            <-chan BeatResult // exactly one terminal value
}
```

- `NarrativeStream` 携带 narrator 流式产出的 delta chunk，按 LLM 实际生产顺序到达。
  channel close 表示叙事段已经全部到位。
- `Done` 是 buffered（size 1），承载终态 `BeatResult`。producer 永远不会因为
  consumer 暂不读取 `Done` 而阻塞。
- 调用约定（在 `BeatOutput` 注释里成文）：caller 必须先 drain 完
  `NarrativeStream` 才能读 `Done`。`Done` 是在 `NarrativeStream` close
  之后写入的（见 §2.3），先读 `Done` 会死锁。
- 对于不关心流式 UX 的同步 caller（测试、脚本），提供 `BeatOutput.Wait()` helper
  —— 它 range 掉 stream（丢弃 chunk）、再读 `Done`、返回 `BeatResult`。所有
  现有的 `sess.RunBeat(...)` 测试都改成了 `sess.RunBeat(...).Wait()`，零行为
  变化。

### 2.2 BeatResult 字段

```go
type BeatResult struct {
    Err         error                 // hard fail
    World       worldmodel.World
    Narrative   string                // 全文（chunk 拼接后的 snapshot）
    ToolEffects []worldmodel.Effect
    Choices     role.ActionChoices
    SuggestErr  error                 // (既有) graceful-degrade
    LoreErr     error                 // (新增) graceful-degrade
    LoreReport  ingest.CompileReport  // (新增)
}
```

- `Narrative` 字段保留：caller 不必自己拼接 chunk，需要全文（测试、Lorekeeper、
  日志）可以直接拿。
- `LoreErr` 与既有的 `SuggestErr` 同性质 —— 都是"软失败"，不会污染 `Err`；
  叙事与 tool effects 已经落盘，只是没有新的 lore 沉淀（详见 §3.3）。
- `LoreReport` 在 Lorekeeper 未配置或 `LoreErr != nil` 时为零值。

### 2.3 runBeatStream goroutine 责任

`RunBeat` 启动一个 goroutine（`runBeatStream`），它有三条硬约束：

1. **永远 close `narrativeCh`**：即便 pipeline 在 LoadSnapshot 阶段就早退，
   range 它的 caller 必须能终止。
2. **`doneCh` 恰好写一次**：channel size = 1，写入永远不阻塞。
3. **顺序保证**：必须先 `close(narrativeCh)` 再 `doneCh <- result`。
   `runBeatStream` 用一个 `defer` 同时执行这两步以保证顺序无论早退还是正常
   完成都成立。具体 pipeline 逻辑放在 `runBeatPipeline` 里，pipeline 写
   `result.Err` 后直接 `return`，goroutine 的 defer 仍正确收尾。

### 2.4 Narrator 适配

`session.runBeatPipeline` 改成调用 `agent.Stream(ctx, messages)` 而非
`agent.Generate`：

- Eino 的 `react.Agent.Stream` 会逐 chunk 把 ReAct 循环里的 assistant
  message 吐出来。
- **过滤 tool-call chunks**：first-chunk checker 会把 ToolCalls 标在 first chunk
  的 `chunk.ToolCalls` 上；我们用 `if len(chunk.ToolCalls) > 0 { continue }`
  跳过这些 frame，只把 `chunk.Content != ""` 的纯文本 chunk 转发到
  `narrativeCh` 并累积到 `narrativeBuf`。
- `narrativeBuf.String()` 就是 `BeatResult.Narrative` 的来源；下游
  Lorekeeper / EventLog summary / 后续 SuggestActions 都消费它。
- ctx 取消时通过 `select { case <-ctx.Done(): ... case narrativeCh <- chunk.Content: }`
  正确 backpressure，不会泄漏 goroutine。

## 3. Lorekeeper 规约

### 3.1 接口契约

```go
package role

import "github.com/sizolity/worldline/internal/world/ingest"

type Lorekeeper interface {
    ingest.Parser // Parse(ctx context.Context, doc SourceDocument) (Draft, error)
}
```

- 方法集**严格等同**于 `ingest.Parser`，不是 superset。这确保：
  - 任何已实现 `ingest.Parser` 的类型自动满足 `role.Lorekeeper`，无 adapter。
  - Lorekeeper 可以传给任何接受 `ingest.Parser` 的 API（例如未来
    `ingest.ImportFile`），无需重写签名。
- 失败语义在 doc-comment 里成文：**callers MUST log and continue on error;
  a Lorekeeper failure MUST NOT abort the beat**。这条约束直接驱动 §3.3 的
  集成方式。

### 3.2 `narrator.LoreParser` 实现

文件：`rpg/gm/narrator/lorekeeper.go`。

私有 schema 类型 `loreDraft` **镜像** `ingest.Draft` 但故意删掉 `Canon`：

```go
type loreDraft struct {
    Entities  []ingest.DraftEntity   `json:"entities,omitempty"  jsonschema:"..."`
    Relations []ingest.DraftRelation `json:"relations,omitempty" jsonschema:"..."`
    Facts     []ingest.DraftFact     `json:"facts,omitempty"     jsonschema:"..."`
    Threads   []ingest.DraftThread   `json:"threads,omitempty"   jsonschema:"..."`
    Memories  []ingest.DraftMemory   `json:"memories,omitempty"  jsonschema:"..."`
}
```

为什么删 Canon：Canon (genre / tone / premise / laws / boundaries / secrets) 是
**世界级元数据**，由 world template 在创建时一次性写入，不应该被每回合 LLM
重新"想象"出来。元素类型直接用 `ingest.Draft*`，所以 `loreDraft → ingest.Draft`
是一次字段拷贝，没有平行 type 等级。

调用链：

1. `utils.GoStruct2ToolInfo[loreDraft]("record_lore", ...)` —— 用 `loreDraft`
   的 jsonschema 标签生成 Eino tool descriptor。
2. `chatModel.WithTools([]*schema.ToolInfo{toolInfo})` —— 绑定**唯一**这个工具。
3. `bound.Generate(ctx, [system, user], model.WithToolChoice(schema.ToolChoiceForced))`
   —— forced tool choice 让 LLM 必须调用 `record_lore`，避免它返回普通文本。
4. `schema.NewMessageJSONParser[loreDraft](&{ParseFrom: MessageParseFromToolCall}).Parse(...)`
   —— 从 tool-call 的 arguments 里解出 `loreDraft`。
5. 字段拷贝成 `ingest.Draft` 返回。

System prompt（中文，硬覆盖 LLM 的"我以为该这样"的猜测）覆盖了以下规约：

- **ID 规范**：lower_snake_case；按类型加前缀
  (`ent_` / `rel_` / `fact_` / `thr_` / `mem_`)；同次返回中 ID 不重复。
- **Entity Type**：character / location / item / faction / event 五种 ASCII 单词。
- **Relation Type / Fact Predicate**：均为 lower_snake_case，给出常用例
  (`disciple_of`, `allied_with`, `has_weapon`, `is_at` …)。
- **Thread Status**：必须是 `active` 或 `open`；Priority / Tension 在 [0,1]。
- **Memory Scope / Kind / OwnerKind**：
  - Scope ∈ {canonical, factual, subjective, rumor, emotional, procedural}
  - Kind  ∈ {observation, belief, rumor, summary}
  - OwnerKind 默认 `world`；选 character / faction / narrator 时 OwnerID 必须给。
- **Confidence / TruthStatus**：单点出现的从属信息不要给 confidence=1.0；
  对话里的猜测、传言用 `truth_status="unknown"` 或 `kind="rumor"`。
- **source_refs**：每一项填用户消息中提供的"来源 ID"，即 `doc.ID`。

**空文本短路**：`strings.TrimSpace(doc.Text) == ""` 时直接返回 `ingest.Draft{}, nil`，
不消耗任何 LLM 配额。这条让 silent setup beat（prologue / recap）零成本通过。

### 3.3 Session 集成

`session.Config` 新增可选字段：

```go
type Config struct {
    // ... 原有字段 ...
    Lorekeeper role.Lorekeeper // optional; nil → 跳过整个 lore pipeline
}
```

`session.Session` 持有同名字段。注入逻辑（CLI 在 §5.1）和未注入时的零开销退化
（`if s.lorekeeper == nil` 直接跳过 §3.4 整段）都成文。

**插入点**（`runBeatPipeline` 里的精确顺序）：

1. Load world + disclosure
2. Build tools (`gm.Tools`) + 三个 view 投影
3. `gm.SystemPrompt` → `react.Agent.Stream` → 累积 narrative（§2.4）
4. `runtime.ApplyEvent(world, beatEvent)` —— 把玩家行动 + 截断后的叙事
   写成 EventLog 单条 event
5. `world.Clock.Sequence++`
6. **WorldLine scheduler** (`story.Tick`)，把 milestone effects 也合并进 world
7. **【Lorekeeper】** (§3.4) —— 见下文
8. `store.SaveSnapshot(world)` —— 此时 world 已合并了 5–7 的所有结果
9. `fogStore.Save`（若启用）
10. `gm.SuggestActions`（可 graceful-fail）

**为什么 Lorekeeper 必须在 step 7（而不是 4 之后或 8 之后）**：

- 在 4–6 之后：这样 draft 能引用本回合产生的事件 ID（`beatEvent.ID` 作为
  `SourceDocument.ID`），以及 WorldLine 触发的 milestone effect。
- 在 8（SaveSnapshot）之前：成功时 lore 与本回合 world 一并落盘；失败时
  我们已经握有未变的 `world`，让 SaveSnapshot 仍然保存 step 6 的状态，
  不残留半成品。

**失败语义**（已在 `runLorekeeper` 注释里成文，也由 `TestRunBeat_LorekeeperParseFails`
锁住）：

- `s.lorekeeper.Parse` / `ingest.CompileDraft` 任意一步返回 error：
  - `runLorekeeper` 返回 `(world, ingest.CompileReport{}, wrappedErr)`，
    **第一个返回值是入参 world 原样回传** —— 调用者即使忘了检查 err 直接
    赋值，也不会进入半改动状态。
  - 当前 call site 仍然 gate 在 `if loreErr != nil`，只在成功分支才把 newWorld
    赋回；这个 belt-and-suspenders 让意图保持可读。
- `result.LoreErr = loreErr`，`result.LoreReport` 保持零值。
- `result.World / result.Narrative / result.ToolEffects` 不受影响 → CLI footer
  打印一行警告但游戏继续推进。

### 3.4 ValidateDraft + CompileDraft 流水线

```go
draft, err := s.lorekeeper.Parse(ctx, doc)        // sourceID = beatEvent.ID
if err != nil { return ..., err }
validation := ingest.ValidateDraft(draft)         // 仅产 findings，不抛 error
newWorld, report, err := ingest.CompileDraft(world, draft, ingest.CompileOptions{
    ConflictPolicy: ingest.ConflictPolicySkip,    // 见 §7 折中
    // Resolver: nil → CompileDraft 内部回落到 NoopAliasResolver
})
if err != nil { return ..., err }
for _, e := range validation.Errors   { report.Notes = append(report.Notes, "validate-error: "+e) }
for _, w := range validation.Warnings { report.Notes = append(report.Notes, "validate-warn: "+w) }
return newWorld, report, nil
```

设计要点：

- **`ValidateDraft` 的错误不是 hard abort**：它会找出 "memories[i] 的 owner_id
  缺失" 这类问题，但 `CompileDraft` 内部对每个 item 也做了一次 `Validate()`，
  会丢掉非法 item 单独 `report.Rejected++`、其他 item 照常 insert。因此我们把
  validate finding 收集进 `report.Notes`（前缀 `validate-error:` /
  `validate-warn:`）方便 CLI / 测试观察，但不让单条非法 item 毁掉整个 draft。
  `TestRunBeat_LorekeeperReportNotesIncludeValidate` 直接锁住"一好一坏"的样本，
  断言 Inserted=1 且 Notes 中存在 validate prefix。
- **ConflictPolicySkip + Noop Resolver**：当前默认策略 — 见 §7。
- `sourceID := beatEvent.ID`：每条沉淀的 lore item 通过
  `CompileReport.Provenance.SourceRefs` 都能溯源到产生它的 EventLog 条目。
  注释里明确"do not change this without updating downstream callers"。

## 4. NPC 记忆渲染（Narrator Prompt）

新增 prompt 段位 `## NPC 记忆`（`rpg/gm/narrator/prompt.go::buildNPCMemorySection`）。
这是闭环里**最后一环**：Lorekeeper 写入 memory 后，下一回合 Narrator 才能从
prompt 里读到它，从而打破"LLM 凭训练记忆重新发明角色"的死循环。

### 4.1 入参

```go
buildNPCMemorySection(wc.Entities, wc.Memories)
```

`wc` 是 `role.PromptOptions.WorldCtx`，类型 `view.WorldDebugContext` —— 这是 GM
看到的"无 filter 全真世界"投影（实体已 ID-sorted、记忆已 clone）。这里**不**用
`view.CharacterContext`：NPC 记忆段是 GM 视角，需要看到所有 NPC 的全部记忆，
而 CharacterContext 是单一玩家视角下的可见子集。

### 4.2 过滤与分组

- **memory 过滤**：仅保留 `m.Owner.Kind == MemoryOwnerKindCharacter` 的记忆
  （`"character"`）。World/Narrator/Faction 三种 OwnerKind 不会归属到具体 NPC bucket。
- **bucket**：按 `Owner.ID` (`model.EntityID`) 分桶。
- **NPC 过滤**：遍历 `wc.Entities`，跳过 `e.Type != "character"` 的实体；
  并通过 `hasPromptTag(e.Tags, "player")` 跳过玩家角色 —— 玩家不在 NPC 列表里。
  （未来如果 seed/Lorekeeper 一致地不给 NPC 打 "player" tag，可以删掉这一条
  filter，但当前 demo world 仍带 `"player"` tag。）
- **排序**：每个 NPC 的 memory list 按 `Importance` desc、ID asc 二级排序，
  以保证渲染稳定（`sort.SliceStable`）。
- **分组渲染**（顺序固定）：
  - `summary` (`MemoryKindSummary`) → label `长期记忆`
  - `observation` (`MemoryKindObservation`) → label `短期记忆`
  - `belief` (`MemoryKindBelief`) → label `信念`
  - `rumor` (`MemoryKindRumor`) → label `传言`
  - 其余（未识别 Kind 或空字符串） → label `(未分类)`
- 空桶 silently 跳过，不留空标题。

### 4.3 真伪标记（renderMemoryLine）

```go
switch m.TruthStatus {
case model.TruthStatusFalse, model.TruthStatusOutdated:
    marker = " (可能有误)" // npcMarkerUntrusted
case model.TruthStatusDisputed:
    marker = " (有争议)"   // npcMarkerDisputed
}
```

- `false` / `outdated` 两种状态被合并显示为「(可能有误)」——它们对 LLM 的语义
  都是"不要把这条当成 canonical"。
- `disputed` 单独显示为「(有争议)」，提示"两方说法都有，但谁是真不确定"。
- 其他（`true`, `unknown`, `secret`, 空字符串）不打 marker。
- 全部 label 与 marker 作为包级 const（`npcLabel*`, `npcMarker*`,
  `npcSectionEmpty`）暴露给 tests，避免 prompt 与 assertion 字面量漂移
  （全角 / 半角 / 多空格的常见 bug）。

### 4.4 容量约束

- 最多 `npcSectionMaxNPCs = 6` 个 NPC（按 entity 遍历顺序，达到上限后 break）。
- 每个 NPC 最多 `npcSectionMaxMemoriesPerNPC = 5` 条 memory。
- 每条 content 最多 `npcSectionMaxMemoryRunes = 200` runes（多字节安全）。

近似上限：6 × 5 × 200 ≈ 6 KiB runes，对 deepseek-chat 8 KiB 上下文留够余量。

### 4.5 空段语义

当**没有任何 NPC 持有 memory** 时，返回单行常量 `(no NPC memories yet)`
（`npcSectionEmpty`）。**保留** `## NPC 记忆` 标题不缩塌，让 LLM 知道这段
位置存在、只是当前为空 —— 否则模型可能把后续 `## Locations` 段的内容
误判为 NPC 记忆的延续。

## 5. CLI 集成

### 5.1 cmdPlay 接线

`cmd/rpg-cli/main.go::cmdPlay`：

```go
chatModel, _ := openai.NewChatModel(...)
gm, _       := narrator.New(chatModel)
lk          := narrator.NewLoreParser(chatModel) // 与 GM 共用同一 chatModel
sess, _     := session.New(session.Config{
    GM:            gm,
    Players:       []role.Player{player},
    WorkspacePath: *workspace,
    ChatModel:     chatModel,
    MaxStep:       *maxStep,
    StoryEnabled:  !*noStory,
    Lorekeeper:    lk,
})
```

**为什么 LoreParser 可以与 GM 共用同一个 `chatModel`**：Eino 的
`ToolCallingChatModel.WithTools` 返回**新的 bound instance**，不会污染原
chatModel；GM 这边走 ReAct agent 的 Stream，Lorekeeper 这边走 forced
`record_lore` 工具调用，两路并不互踩。

### 5.2 streamBeat 状态尾

`cmd/rpg-cli/main.go::streamBeat` 把 BeatResult 渲染成一段 footer：

```
回合=N 效果=M 张力=th1=0.25 th2=0.10
(行动建议失败：%v — 请自由输入下一步行动)  ← 仅 SuggestErr 非空时
(知识沉淀失败：%v — 本回合无新增世界知识)  ← 仅 LoreErr 非空时
沉淀: 插入=N 跳过=M 拒绝=K 过滤=L (含 X 条提示)? ← 仅 summarizeLoreReport 非空时
```

`summarizeLoreReport` 的语义（在源码 doc-comment 里成文）：

- 全 0（Inserted=Skipped=Rejected=Filtered=0）且 Notes 空 → 返回 `""`，
  整行**不打印**。**这条很重要**：prologue / recap / 任何叙事极短的回合，
  Lorekeeper 大概率拿不到信号；如果每回合都强行打 `沉淀: 插入=0 跳过=0 ...`
  会把 REPL 刷成噪声。
- 否则总是先输出 `沉淀: 插入=N 跳过=M 拒绝=K 过滤=L`。
- `len(Notes) > 0` 时追加 ` (含 X 条提示)` 提示 caller 完整 report 里还有
  validate findings / compile rejection notes。

### 5.3 cmdStatus 世界知识段

`cmd/rpg-cli/main.go::printWorldKnowledge`（在 `cmdStatus` 末尾调用）：

- **整段跳过条件**：`len(w.Entities) == 0 && len(w.Memory) == 0` —— 全新 seed、
  尚无任何 beat 的世界直接不打 "世界知识" 段。
- **实体计数**：character / location / item 三类显式打出，其余归入 `其他`
  桶。`其他` 计数 > 0 时才追加 `(其他=N)`，避免在干净世界里出现 `(其他=0)`
  这种无用噪声。最后 `总计=N`。
- **记忆计数**：`world` / `character` 两类显式打出，其余（narrator / faction）
  并入 `其他`，同样 > 0 才追加。最后 `总计=N`。
- **NPC 记忆 Top**：`topNPCsByMemoryCount(w, 5)` 按 character-owned memory
  count desc、ID asc 取前 5。零计数的 NPC 不算。子段为空时整段
  silently 省略 —— 早期世界完全没 NPC memory 时不渲染空标题。

## 6. LLM / Deterministic Boundary

| 调用点 | LLM？ | 失败处理 |
|---|---|---|
| `Narrator.SystemPrompt` | 否（纯模板）| —— |
| `Narrator.Tools` | 否（基础工具）| 直接返 err |
| `Narrator.Judge` | 否（spec 硬约束）| —— |
| `react.Agent.Stream` (per beat) | **是** | `result.Err`（hard fail）|
| `Narrator.SuggestActions` | **是**（forced tool call）| `result.SuggestErr` graceful |
| `narrator.LoreParser.Parse` | **是**（forced tool call）| `result.LoreErr` graceful |
| `runtime.ApplyEvent` | 否 | hard error |
| `ingest.ValidateDraft` | 否 | 收集进 `report.Notes` |
| `ingest.CompileDraft` | 否 | hard error（包成 `LoreErr`）|
| `story.Tick` (WorldLine) | 否 | hard error |

**核心不变量**：deterministic 路径（ApplyEvent / Validate / Compile / Tick）
不调 LLM、不会因网络抖动失败。三处 LLM 调用各自有边界与降级语义；
Lorekeeper 与 SuggestActions 是 graceful-degrade 而 ReAct 主流是 hard fail —
因为没有叙事就没有这一回合可言。

## 7. 已知折中与未来工作

- **单 LLM 调用串行 NPC 行动**：当前 Narrator 在一次 ReAct 输出里把所有 NPC 的
  反应一并写出。多 NPC 的"轮询 + 各自流式"应在未来版本拆开，并仍走
  `NarrativeStream`。
- **时间感知粒度未做**：玩家显式"快进半天"或"慢入一拍"目前没有 first-class
  支持，世界推进完全靠 WorldLine drift。WorldLine 第二轮设计里会引入
  PerceptionWindow + 显式时间动作。
- **没有记忆遗忘 / 沉淀机制**：所有 memory 一旦写入就永远在 world.Memory 里。
  短-长期晋升（observation → summary）目前完全交给 LLM 在 `Kind` 字段上自我标注，
  没有 deterministic 衰减。下一轮按 importance × age 做被动衰减。
- **ConflictPolicy 固定 Skip**：`CompileDraft(ConflictPolicySkip)` 让重名 entity
  跳过；未来在剧情明确"角色升级 / 改名 / 死亡 → 替身"的场景里要切到 `Replace`。
  目前 spec 不做切换 plumbing。
- **AliasResolver 留空**：CompileDraft 当前回落到 `NoopAliasResolver`，
  Lorekeeper 自己写出来的 entity ID 是什么就是什么。一旦 LLM 同一 NPC 在两个
  回合里写出两个 ID（"baigu_furen" vs "ent_baigu_furen"），就会出现重复
  entity。下一轮做一个 narrator-driven AliasResolver，让 GM 在 SystemPrompt
  里"声明本世界已有的 alias 表"。
- **StoryPack YAML loader + PerceptionWindow** 留在 WorldLine 二轮 plan
  (`docs/superpowers/plans/2026-05-22-world-runtime-risk-first-reuse.md`)
  之后的 follow-up，不在本设计范围。

## 8. 决策日志

- **GM 拆出 Registrar → 包级 `AvailableTemplates()`**：列模板是冷路径目录查询，
  不属于每回合 GM 契约；继续放在 GM 上会污染 per-beat 接口契约的可读性。
- **Lorekeeper 独立成 role，签名复用 `ingest.Parser`**：可替换 / 单一职责 / 不与
  GM 的 LLM 边界耦合；签名一致意味着任何已有 Parser 自动满足 Lorekeeper。
- **流式失败语义**：narrative 流是 hard fail（没有叙事就没有这一回合）；
  Lorekeeper 与 SuggestActions 是 graceful-degrade（这俩是"锦上添花"，
  失败不能毁掉已经写入 EventLog 的玩家行动）。
- **Lorekeeper 在 SaveSnapshot 之前、WorldLine tick 之后**：保证 draft 能引用
  当回合产生的事件 + milestone effect；同时整段 lore + world 一次原子落盘。
- **CLI footer 与 status 段位"零信息时静默"**：summarizeLoreReport 空 report
  返回 "" 不打印；printWorldKnowledge 在 Entities + Memory 都空时整段跳过。
  避免把 REPL / status 输出刷成噪声 —— 用户感知优先于"始终结构化"。
