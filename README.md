# MemPalace-Go

MemPalace rewritten in Go. AI memory system with SQLite FTS5, zero external APIs, single binary.

Original: [milla-jovovich/mempalace](https://github.com/milla-jovovich/mempalace) (Python/ChromaDB)

## What It Does

Every conversation you have with an AI disappears when the session ends. MemPalace stores everything — conversations, project files, decisions, preferences — and makes it searchable. Your AI remembers.

- **Mine** project files and conversation exports into a structured palace
- **Search** with BM25 full-text search (SQLite FTS5)
- **4-layer memory stack** — identity, essential story, on-demand recall, full search
- **AAAK dialect** — 30x compression for AI context windows
- **MCP server** — Claude Code integration over JSON-RPC stdio
- **Knowledge graph** — temporal entity relationships with validity tracking

## Why Go

The Python version depends on ChromaDB (which pulls in PyTorch, ONNX, tokenizers — ~500MB). This Go version replaces vector search with SQLite FTS5 keyword search:

| | Python (ChromaDB) | Go BM25 | Go Vector | Go Fused |
|---|---|---|---|---|
| **LongMemEval R@5** | 96.6% | 96.2% | 96.4% | **97.4%** |
| **LongMemEval R@10** | 98.2% | 97.6% | 98.2% | **99.2%** |
| Latency/query | 1,810ms | **1ms** | 41ms | 45ms |
| Search type | Semantic | Keyword | Semantic | Keyword+Semantic |
| ONNX model needed | Yes (internal) | No | Yes | Yes |
| External deps | ~500MB | None | ONNX Runtime | ONNX Runtime |

Three search modes: **BM25** (FTS5 keyword, zero deps, 1ms), **Vector** (ONNX MiniLM-L6-v2, same model as ChromaDB), **Fused** (BM25 + Vector via Reciprocal Rank Fusion — best quality).

Cross-validated: Python BM25+Vector fusion with same algorithm achieves 97.4% R@5, confirming the Go result.

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

# Search
./bin/mempalace search "why did we switch to GraphQL"

# Show identity + essential story
./bin/mempalace wake-up

# Show palace status
./bin/mempalace status
```

## MCP Server (Claude Code)

```bash
# Build the binary
make build

# Connect to Claude Code
claude mcp add mempalace -- /path/to/bin/mempalace mcp
```

Tools available: `mempalace_status`, `mempalace_search`, `mempalace_list_wings`, `mempalace_list_rooms`, `mempalace_add_drawer`, `mempalace_delete_drawer`, `mempalace_kg_query`, `mempalace_kg_add`, `mempalace_traverse_graph`

## Architecture

```
cmd/
  mempalace/     CLI (cobra)
  bench/         LongMemEval benchmark runner

internal/
  store/         SQLite + FTS5 storage layer
  config/        Config (env > file > defaults)
  normalize/     Chat export format conversion (Claude, ChatGPT, Slack, JSONL, plain text)
  miner/         Project mining, conversation mining, chunking, room detection, memory extraction
  search/        FTS5 search + hybrid keyword boosting
  layers/        4-layer memory stack (L0 identity, L1 story, L2 recall, L3 search)
  graph/         Knowledge graph (temporal triples) + palace graph (BFS traversal)
  dialect/       AAAK compressed symbolic memory language
  entity/        Entity detector (regex heuristics) + persistent JSON registry
  mcp/           MCP JSON-RPC server (9 tools)
```

### The Palace Structure

Memories are organized as:

- **Wings** — people, projects, topics (e.g., `backend`, `alice_chats`)
- **Rooms** — categories within a wing (e.g., `architecture`, `debugging`, `database`)
- **Drawers** — individual text chunks with metadata

### Memory Types

The general extractor classifies text into 5 types:
- **Decisions** — "we decided to use PostgreSQL"
- **Preferences** — "I always use vim"
- **Milestones** — "shipped the new API"
- **Problems** — "critical bug in auth module"
- **Emotional** — "frustrated with the deployment process"

### AAAK Dialect

A lossless shorthand for AI agents. Compresses months of context into ~120 tokens:

```
[T: database, migration, postgresql]
[E: trust, hope]
[F: DECISION, MILESTONE]
[ENT: A1, B1]
A1 B1 discussed migration. decided postgresql for jsonb support.
migration completed, all tests passing.
```

## Benchmark

Compare Go FTS5 vs Python ChromaDB on LongMemEval:

```bash
# Build benchmark runner
go build -o bin/bench ./cmd/bench

# Run (download data first — 265MB)
./bin/bench --data longmemeval_s_cleaned.json --limit 50 --csv results.csv
```

## Development

```bash
make help     # Show all targets
make build    # Build binary
make test     # Run all tests
make lint     # Run go vet
```

All tests:

```
internal/store      7 tests    SQLite CRUD + FTS5 search
internal/config     3 tests    Config loading
internal/normalize  8 tests    Format conversion
internal/miner     23 tests    Mining, chunking, room detection, extraction
internal/search    10 tests    Search + hybrid boosting
internal/layers     4 tests    Memory stack
internal/graph     11 tests    Knowledge graph + palace graph
internal/dialect   11 tests    AAAK compression
internal/entity    10 tests    Entity detection + registry
internal/mcp       24 tests    MCP server + protocol
```

## Dependencies

- `modernc.org/sqlite` — Pure Go SQLite (no CGo)
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/spf13/cobra` — CLI framework

## License

MIT
