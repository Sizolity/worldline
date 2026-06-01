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

# Seed a Journey to the West (西游记) demo world.
./bin/rpg-cli seed   --workspace /tmp/wl --world-id xiyou-changan

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
agent/                       framework-neutral agent interfaces
├── chat/, tool/, beat/,
│   structured/, director/   abstract agent contracts
└── eino/                    cloudwego/eino adapters (chat, tool, beat, structured)
rpg/                         RPG product code
├── session/                 beat pipeline orchestration (streaming + Lorekeeper)
├── narrator/                default Narrator (Persona + Rulebook + Director)
├── role/                    role interfaces (Persona, Rulebook, Director,
│                            Lorekeeper, GM, Player) — small, composable
├── story/                   WorldLine scheduler (drift, milestones, conditions)
├── fog/                     progressive world disclosure
├── rule/                    RPG narrative rule schema + helpers
├── template/                fantasy / mystery / scifi / modern world templates
└── tools/                   roll / update_state / lookup_rules / ...
world/                       world simulation framework (see "Provenance" below)
├── model/                   World, Entity, WorldEvent, MemoryRecord, ...
├── runtime/                 ApplyEvent, Step, Run, builtin rules
├── store/                   FileStore + WorldTemplate
├── view/                    NarrativeView, WorldDebugView, CharacterContextView
├── ingest/                  Draft → World compile pipeline
└── director/                event proposal interface + script/random/reconcile/
                             event-table directors
e2e/rpg/                     real-DeepSeek end-to-end tests (build tag e2e)
internal/app/                application wiring
├── llm/                     LLM client setup
├── seed/                    world seeding
└── status/                  status display
```

## Provenance

`world/*` is a fork-and-copy of the `internal/world` subset from
`github.com/sizolity/nobody` at commit **`f88508c`** (2026-05-27), promoted
to a top-level package. Worldline and Nobody evolve independently from this
point. Fixes do not flow either direction automatically.

The packages copied: `model`, `director`, `runtime`, `store`, `view`,
`ingest`. The packages intentionally **not** copied:

- `internal/world/devcli`, `internal/world/runner` — Nobody's framework CLI
- `internal/world/system` — currently dead code in Nobody itself

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
