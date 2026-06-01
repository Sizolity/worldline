# Worldline

Worldline is a local-first, LLM-driven narrative RPG runtime.

A single beat in Worldline takes a player action and produces:

- a streamed narrative segment (Eino ReAct + DeepSeek by default),
- structured world updates (entities, relations, facts, threads, memories)
  derived from the narrative by a separate **Lorekeeper** role,
- a deterministic **WorldLine** schedule tick that drifts thread tensions
  and fires milestone effects,
- two or four suggested next-step actions for the player.

The full world state, beat history, world lines, and disclosure (fog of war)
all persist as JSON / JSONL under a single workspace directory, so every
session is replayable and inspectable offline.

## Quick Start

```fish
set -x DEEPSEEK_API_KEY <your-key>

go build -o ./bin/rpg-cli ./cmd/rpg-cli

# Seed a Journey to the West (西游记) demo world. --scenario / --style are
# required (the engine ships no defaults); --world-id falls back to --scenario.
./bin/rpg-cli seed   --workspace /tmp/wl --scenario xiyou-changan --style shuoshu --world-id xiyou-changan

# Play it. Each non-empty line is one beat; digit N picks the Nth suggested
# action; "q" or Ctrl-D exits.
./bin/rpg-cli play   --workspace /tmp/wl --world-id xiyou-changan

# Inspect where you left off without entering the REPL.
./bin/rpg-cli status --workspace /tmp/wl --world-id xiyou-changan
```

## Repository Layout

```text
cmd/
├── rpg-cli/                 manual smoke-test CLI (seed / play / status)
└── rpg-server/              HTTP server scaffold
internal/
├── agent/                   agent orchestration layer (built on cloudwego/eino)
│   ├── react/               multi-step ReAct loop (streaming narrative + tool calls)
│   ├── typed/               single-shot structured extraction (ToolCall[T] / JSONObject[T])
│   ├── tool/                worldline-native Tool interface + eino adapter
│   └── provider/            LLM provider factories
│       ├── deepseek/        DeepSeek ChatModel factory
│       └── internal/        shared LLM provider infrastructure
│           └── httpretry/   http.Client with transient-error retry transport
├── world/                   world simulation framework
│   ├── model/               World, Entity, WorldEvent, MemoryRecord, clone methods
│   ├── runtime/             ApplyEvent, Step, Run, builtin rules
│   ├── store/               FileStore + WorldTemplate
│   ├── view/                NarrativeView, WorldDebugView, CharacterContextView
│   ├── ingest/              Draft → World compile pipeline
│   └── director/            event proposal interface + script/random/reconcile/
│                            event-table/LLM directors, StripMarkdownFences
├── rpg/                     RPG product code
│   ├── app/                 play-runtime wiring (PlayConfig → PlayRuntime)
│   ├── seed/                CLI seed command: compile mod → world snapshot + worldlines
│   ├── status/              CLI status command: world + worldlines progress report
│   ├── session/             beat pipeline orchestration (streaming + Lorekeeper)
│   ├── narrator/            default Narrator (Persona + Rulebook + Director)
│   ├── role/                role interfaces (Persona, Rulebook, Director,
│   │                        Lorekeeper, GM, Player)
│   ├── story/               WorldLine scheduler (drift, milestones, conditions)
│   ├── fog/                 progressive world disclosure
│   ├── mod/                 mod loading, markdown parsing, prompt rendering, scenarios/styles
│   ├── rule/                RPG narrative rule schema + helpers
│   ├── tools/               roll / update_state / lookup_rules / ...
│   └── intent/              LLM-driven REPL input interpreter
└── env/                     environment loading & project init
e2e/rpg/                     real-DeepSeek end-to-end tests (build tag e2e)
mod/                         mod data: scenarios, styles, persona markdown files
docs/                        design documents, devlog, security notes
```

## Verification

```fish
go vet ./...
go build ./...
go test ./... -short
```

Real-LLM E2E (requires `DEEPSEEK_API_KEY`):

```fish
go test -tags=e2e ./e2e/rpg/...
```

## Design Documents

- [`docs/superpowers/specs/2026-05-27-rpg-lorekeeper-streaming-design.md`](docs/superpowers/specs/2026-05-27-rpg-lorekeeper-streaming-design.md)
- [`docs/superpowers/plans/2026-05-27-rpg-lorekeeper-streaming.md`](docs/superpowers/plans/2026-05-27-rpg-lorekeeper-streaming.md)
- [`docs/engineering/devlog.md`](docs/engineering/devlog.md)
