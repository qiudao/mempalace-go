# MemPalace-Go

**Give your AI a memory — fast, local, multilingual.**

MemPalace rewritten in Go. Single binary, SQLite storage, Ollama-powered semantic search.

Original: [milla-jovovich/mempalace](https://github.com/milla-jovovich/mempalace) (Python/ChromaDB)

## The Problem

Every AI conversation disappears when the session ends. Six months of decisions, debugging sessions, architecture debates — gone. You start over every time.

MemPalace stores everything and makes it searchable. Your AI remembers.

## What Changed from Python

| | Python (ChromaDB) | **Go (this repo)** |
|---|---|---|
| **LongMemEval R@5** | 96.6% | **98.0%** (Smart Search) |
| **Search latency** | 555ms | **58ms** (10x faster) |
| **Mine latency** | 15.8s | **7.1s** (2.2x faster) |
| **Chinese/multilingual** | Partial (MiniLM) | **Full** (Ollama nomic) |
| **Binary** | ~500MB deps | **~15MB** single binary |
| **Embedding backend** | ChromaDB internal | **Ollama** (any model) / ONNX fallback |
| **API keys** | None | None |

### Key Improvements

**Smart Search** — Auto-routes queries to the optimal search strategy:
- CJK/multilingual queries → pure Vector (FTS5 can't segment Chinese)
- Preference/recommendation queries → pure Vector (avoids BM25 vocabulary mismatch)
- Fact queries → Fused BM25-heavy (exact keywords dominate)
- Temporal queries → Fused balanced (both signals help)

**Ollama Integration** — Supports any local embedding model:
```bash
mempalace mine ./project                               # default: nomic-embed-text
mempalace mine ./project --embed-model mxbai-embed-large  # higher quality
```
Auto-detection: Ollama → ONNX → BM25 fallback. No configuration needed.

**Three Search Modes** combined via Reciprocal Rank Fusion:
- **BM25** (FTS5 porter stemmer) — 1ms, zero deps, exact keyword matching
- **Vector** (Ollama/ONNX) — 41ms, semantic similarity
- **Fused** — BM25 + Vector RRF, best overall quality

**Real-World Validated** — Tested on actual Claude Code sessions (Chinese + English):
```
Python ChromaDB:  15/15 hits, 555ms avg
Go + Ollama:      15/15 hits,  58ms avg  ← same quality, 10x faster
```

Cross-validated on LongMemEval (500 questions): Python BM25+Vector RRF achieves the same 97.4% R@5, confirming Go's result is not inflated.

## Quick Start

```bash
# Build
make build

# Initialize palace
./bin/mempalace init

# Mine a project
./bin/mempalace mine ~/projects/myapp

# Mine conversations (Claude, ChatGPT, Slack exports)
./bin/mempalace mine ~/chats/ --mode convos --wing my_chats

# Search (auto-detects best mode)
./bin/mempalace search "why did we switch to GraphQL"

# Show identity + essential story
./bin/mempalace wake-up

# Show palace status
./bin/mempalace status
```

### Prerequisites

- Go 1.22+
- [Ollama](https://ollama.com/) (recommended, for semantic search):
  ```bash
  ollama pull nomic-embed-text   # 274MB, multilingual
  ```
  Without Ollama, falls back to BM25 keyword search (still 96.2% R@5).

## MCP Server (Claude Code)

```bash
claude mcp add mempalace -- /path/to/bin/mempalace mcp
```

9 tools available: `status`, `search`, `list_wings`, `list_rooms`, `add_drawer`, `delete_drawer`, `kg_query`, `kg_add`, `traverse_graph`

Claude Code calls these automatically — you never type `mempalace search` manually.

## How It Works

**1. Mine** — Conversations and projects are split into focused chunks (1 exchange = 1 drawer), classified into wings/rooms, embedded via Ollama, and indexed in SQLite.

**2. Search** — SmartSearch classifies the query and routes to the optimal strategy. Results are ranked by BM25 keyword scores, vector similarity, or both fused via RRF.

**3. Remember** — 4-layer memory stack loads only what's needed:
```
L0: "Who am I"           (~100 tokens)   ← always loaded
L1: "My core story"      (~500 tokens)   ← always loaded
L2: Wing/room recall     (on demand)     ← AI walks to the right room
L3: Full search          (on demand)     ← AI searches across everything
```

**4. Compress** — AAAK dialect shrinks months of context into ~1,000 tokens for the AI's context window. A full day of conversation (32K tokens) compresses to ~1K tokens.

### The Palace Structure

```
Palace
├── Wing: tradingview           ← project
│   ├── Room: frontend          ← topic
│   │   └── Drawer: "涨幅扫描 UI 播放功能..."
│   └── Room: backend
│       └── Drawer: "screen-gain command..."
├── Wing: alice_chats           ← person
│   └── Room: architecture
│       └── Drawer: "decided to use GraphQL..."
```

### Knowledge Graph

Temporal entity relationships with validity tracking:
```
Alice —[works_at]→ Acme    (2024-01 ~ 2025-06)
Alice —[works_at]→ NewCo   (2025-06 ~ now)
→ "Where does Alice work?" → NewCo (auto-tracks changes)
```

## Architecture

```
cmd/
  mempalace/     CLI (cobra)
  bench/         LongMemEval benchmark runner

internal/
  store/         SQLite + FTS5 + vector storage
  config/        Config (env > file > defaults)
  normalize/     Format conversion (Claude, ChatGPT, Slack, JSONL, text)
  miner/         Project + conversation mining, chunking, room detection
  search/        SmartSearch router, BM25, vector, fused RRF, hybrid boost
  layers/        4-layer memory stack
  graph/         Knowledge graph (temporal triples) + palace graph (BFS)
  dialect/       AAAK compressed symbolic memory language
  entity/        Entity detector + persistent JSON registry
  embed/         Ollama + ONNX embedding backends (EmbedderI interface)
  mcp/           MCP JSON-RPC server (9 tools)
```

## Benchmark

```bash
# Build benchmark runner
go build -o bin/bench ./cmd/bench

# Run LongMemEval (download data first — 265MB)
./bin/bench --data longmemeval.json --mode smart --csv results.csv

# Available modes: raw (BM25), vector, fused, smart
```

| Mode | R@5 | R@10 | Latency |
|------|-----|------|---------|
| BM25 (raw) | 96.2% | 97.6% | 1ms |
| Vector | 96.4% | 98.2% | 41ms |
| Fused | 97.4% | 99.2% | 45ms |
| **Smart** | **98.0%** | **99.0%** | 71ms |
| Python ChromaDB | 96.6% | 98.2% | 1,810ms |

## Development

```bash
make help     # Show all targets
make build    # Build binary
make test     # Run all tests (~110 tests across 10 packages)
make lint     # Run go vet
```

## Dependencies

- `modernc.org/sqlite` — Pure Go SQLite (no CGo)
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/spf13/cobra` — CLI framework
- [Ollama](https://ollama.com/) — Local embedding API (optional, recommended)

## License

MIT
