# 开发日志

记录开发过程中踩过的坑、绕过的限制、总结的范式。新条目追加在最前面，旧条目下沉，永远不要回填编辑历史条目（只能新增"勘误"条目）。

每条目格式：日期 + 一句话标题 + (情境 / 现象 / 根因 / 处置 / 预防)。

---

## 2026-05-28 — 给 cloudwego/eino-ext 提 issue/PR 时踩的几个坑

一次任务里连环碰到 4 个跟"对外协作 + 本机工具链"相关的坑，集中记录。

### 1. 给 upstream 的 issue / PR 描述里漏掉项目专有名词

**情境**: 给 cloudwego/eino-ext 起草 issue 和 PR，描述 "real-world driver" 段落时直接照搬本仓的内部术语：`Lorekeeper`、`world-state`、`narrative beat`、`free-form narration call`。

**现象**: issue 在 cloudwego 仓库公开发布后，第一时间被本人察觉项目专有信息泄露；PR 草稿在落地前被拦下来重写。

**根因**: 起草外部技术文档时把"内部讨论惯用语"当成了"通用技术语言"。"Lorekeeper" 在本项目里指特定角色，但在 upstream 看来是无意义的代号；同时它把本项目的功能定位（RPG / 叙事 / 世界线）顺带暴露了出去。

**处置**:
- 起草任何对外公开的文档（GitHub issue / PR / mailing list / blog / 第三方 SDK feature request）**前**，机械化地做一次"项目术语 grep"。本仓应至少扫这几个词：`Lorekeeper`、`WorldLine`、`world-runtime`、`nobody`、`Xiyou` / `西游`、`narrative beat`、`Director` / `Persona` / `Rulebook` 这种角色化命名。
- 通用替换表（在外部文档里见到左列就替换成右列）：

  | 项目专有词 | 通用替代 |
  |---|---|
  | `Lorekeeper` / `Lorekeeper sidecar` | `structured-extraction call` / `structured-output sidecar` |
  | `narrative beat` / `beat narrative` | `free-form text` / `LLM-generated text segment` |
  | `world-state` | `structured data` |
  | `the free-form narration call` | `the free-form text call` / `free-form generation call` |
  | `Director` / `Persona` / `Rulebook` | `the orchestrator` / `the system prompt persona` / `the rule set` |

- 文风层面：第一人称叙事（"We are building X that …"）比第三人称（"A common pattern is …"）更容易夹带项目语境。能用第三人称就用第三人称。

**预防**: 把这条命令加进给 upstream 提文档前的最后一道检查（即将合到本 devlog 顶的 `lint-public-docs` checklist 当雏形）：

```fish
# 起草完外部文档后，至少跑一遍下面这种反向检查
rg -i 'lorekeeper|worldline|world-state|narrative beat|nobody|xiyou|西游|director|persona|rulebook' /tmp/upstream-*.md
```

**勘误链**: issue cloudwego/eino-ext#856 第一版 body 含有专有名词，已通过 `gh issue edit 856 --body-file ...` 覆盖；PR #857 草稿在提交前重写。

---

### 2. `gh` 在 fish shell 下的使用范式（不要再被 bash heredoc 坑）

**情境**: 给 fish 用户写 `gh pr create` 的可粘贴 snippet 时，无脑套了 bash 的 `--body "$(cat <<'EOF' ... EOF)"` 写法。

**现象**: 用户在 fish 里粘贴执行后命令卡死无响应，被迫退出去用 GitHub web UI 手动开 PR。

**根因**:
- fish **不支持 `$(...)` 命令替换**，fish 用 `(...)`；`$(...)` 在 fish 里被解析成 `$()` 这种空变量展开然后接 `(...)`，与预期完全不一样。
- fish **没有 heredoc** (`<<EOF ... EOF`)。fish 在读到这种语法时会等待匹配的行结束符，但又永远等不到，所以挂死在等待续行的状态。

**处置**: 给 fish 用户的 `gh` snippet 一律遵循下面这套范式：

| ❌ 错误（bash 语法，fish 不支持） | ✅ fish 友好替代 |
|---|---|
| `gh ... --body "$(cat <<'EOF' ... EOF)"` | 先 `Write` 把 body 落到 `/tmp/foo.md`，然后 `gh ... --body-file /tmp/foo.md` |
| `gh ... --body "$(printf 'a\nb')"` | `gh ... --body (printf 'a\nb')` |
| `gh ... --body "$VAR"` | `gh ... --body $VAR` |
| `git commit -m "title\n\nlong body…"` （bash 风假装多行）| 多个 `-m`：`git commit -m "title" -m "body 第一段" -m "body 第二段"` |
| 真要复杂 commit body | `git commit --file=/tmp/msg.txt` |

**预防 / Canonical 模板**: 给 fish 用户开 issue 或 PR，从今往后只用 `--body-file`，永远不要在 snippet 里嵌 heredoc：

```fish
# Step 1: 用 Write 工具落到 /tmp（不在 snippet 里出现）
# Step 2: 给用户的可粘贴块
gh issue create -R OWNER/REPO --title "..." --body-file /tmp/issue.md
gh issue edit NUM   -R OWNER/REPO --body-file /tmp/issue.md
gh pr    create -R OWNER/REPO --title "..." --body-file /tmp/pr.md
gh pr    edit NUM   -R OWNER/REPO --body-file /tmp/pr.md
```

> 项目层面 `fish-shell-default.mdc` 规则里其实早就写了 fish 用 `(cmd)`、不要 `$(cmd)`，但当时为了图省事直接套了 bash 的"`$()` + heredoc"组合，违反了 own rule。下次起草 fish snippet 前先重读那条规则。

---

### 3. `bytedance/mockey v1.3.0` 与 Go 1.26 不兼容（fork 仓的 CI 与本机 verify 都会撞）

**情境**: clone cloudwego/eino-ext 到 `/home/karo/workspace/eino-ext` 后跑 `go test ./libs/acl/openai/...`，本机 Go 是 1.26.2。

**现象**:

```
# github.com/cloudwego/eino-ext/libs/acl/openai.test
github.com/bytedance/mockey/internal/monkey/inst.duffcopy·f: relocation target runtime.duffcopy not defined
github.com/bytedance/mockey/internal/monkey/inst.duffzero·f: relocation target runtime.duffzero not defined
FAIL	github.com/cloudwego/eino-ext/libs/acl/openai [build failed]
```

**根因**: `bytedance/mockey` 是基于运行时函数 hook 的 mock 框架，依赖 Go runtime 内部符号 `runtime.duffcopy` / `runtime.duffzero`。Go 1.26 改了这两个符号的链接可见性，mockey v1.3.0 找不到了 → link error。这是 mockey 的固有缺点（与具体 Go 版本耦合），跟当前 PR 的改动无关。

**处置**: 不要降级本机默认 Go（其他项目还需要 1.26+），改用 Go 的 `GOTOOLCHAIN` 机制临时拉一个兼容版本来跑：

```fish
GOTOOLCHAIN=go1.24.4 MOCKEY_CHECK_GCFLAGS=false \
  go test -count=1 -gcflags='all=-N -l' ./...
```

- `GOTOOLCHAIN=go1.24.4`：让 `go` 自动下载并切到 1.24.4 跑这一次（不会改 `go env`）。
- `MOCKEY_CHECK_GCFLAGS=false`：mockey 自己的 startup 检查会要求带 `-gcflags='all=-N -l'`，否则会 panic；这个 env 关掉检查。
- `-gcflags='all=-N -l'`：mockey hook 需要禁内联和优化；不加也能跑，但 mockey 会在 startup 警告。

**预防**:
- 任何用了 `bytedance/mockey` 的项目，在 Go 1.25+ 上跑前先用上面的 toolchain 切换组合做 smoke。
- 上面这条命令本质上是 mockey 项目的官方 workaround，不算 hack，可以放心写到给 reviewer 看的"Tests" 段（PR #857 描述里就这么写了）。

---

### 4. fork PR 在 upstream CI 上 403：`Resource not accessible by integration`

**情境**: PR #857 提到 cloudwego/eino-ext 后，`check-submodule-changes` job 红了，stderr 是：

```
RequestError [HttpError]: Resource not accessible by integration
  POST https://api.github.com/repos/cloudwego/eino-ext/issues/857/comments
  status: 403
  x-accepted-github-permissions: issues=write; pull_requests=write
```

action 是 `peter-evans/create-or-update-comment@v4`，想往 PR 上 post 一条 "需要发新 tag" 的提醒评论。

**根因**: GitHub Actions 的硬安全约束 —— 当 workflow 触发器是 `on: pull_request`（不是 `on: pull_request_target`）**且** PR 来自 fork 时，分配给那次 run 的 `GITHUB_TOKEN` **永远是 read-only**，无论 workflow 里写多少 `permissions: write`。这是平台层面的隔离，目的是防止 fork PR 的恶意 workflow 用 base 仓的 token 干坏事。

具体到 eino-ext 的 `tests.yml`：
- 文件用的是 `on: pull_request` —— fork PR 进入受限模式。
- `check-submodule-changes` job 里写了 `permissions: contents: write, pull-requests: write` —— 对 fork PR 完全无效。
- 同 workflow 里 `Post coverage report` 那一步加了 `continue-on-error: true`，所以那个失败不会让 job 红；但 `check-submodule-changes` 里的同款 comment step **漏加** `continue-on-error`，所以 job 红了。

**处置**: 这是 upstream 的 workflow 问题，contributor 这边**无法在 PR 里 fix**。具体地：
- 不用动你的 fork 设置（不是 fork 仓 token scope 不够）。
- 不用动 PR 的 commit（CI 失败的不是 PR 改动，是 workflow 自己想 comment 但被拒）。
- Maintainer 一看 `Resource not accessible by integration` + post-comment step 就知道是这个常见问题，不会因此卡评审。

如果想替 upstream 提一个非阻塞建议，可以在 PR 评论里贴：

> Heads up: failing `check-submodule-changes` is the standard `Resource not accessible by integration` from `peter-evans/create-or-update-comment` on fork PRs — `on: pull_request` + `permissions: write` can't bypass GitHub's read-only-token policy for fork-sourced runs. Trivial fix is either `continue-on-error: true` (matching the coverage step at line 100) or `if: github.event.pull_request.head.repo.fork == false` on the post-comment step.

**预防**: 给任何用 `on: pull_request` + `peter-evans/create-or-update-comment` 组合的项目提 PR 前，先看一眼对应 workflow 是否有 `continue-on-error` 兜底；没有的话，PR 上会自带一个红 X，得提前心理建设、不要慌。
