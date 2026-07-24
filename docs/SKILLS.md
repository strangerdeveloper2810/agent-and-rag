# JARVIS Skills System Guide

He thong Skills cua JARVIS su dung co che "progressive disclosure" — nap kien thuc chuyen mon VUA KHI CAN, thay vi nhet tat ca vao system prompt. Moi skill la mot file `SKILL.md` chua huong dan cho agent.

The Skills system uses progressive disclosure — specialized knowledge is loaded on-demand rather than bloating the system prompt. Each skill is a `SKILL.md` file that contains specialized instructions for the agent.

> **Status:** Skills system is in planning (Phase 9). The infrastructure (`internal/skills/` package, `skills/` data directory) is scaffolded. Full loading + injection into agent context is upcoming.

---

## What Are Skills? (Progressive Disclosure)

### Problem

If you put ALL instructions in the system prompt, you get:
- **Token bloat** — system prompt grows with every capability
- **Confusion** — LLM gets conflicting instructions from different domains
- **Cost** — you pay for every token in the prompt on every call even if the skill is never used

### Solution

Skills are loaded dynamically based on what the user is asking:

```
User: "Help me debug this Go code"
  → Orchestrator detects keyword "debug", "Go", "code"
  → Routes to "code" agent
  → Agent checks active skills directory
  → Loads skills/code-review/SKILL.md
  → Injects skill instructions into the agent's context
  → Agent now has code-review expertise for THIS turn only
```

The skill content is NEVER in the system prompt by default. It's loaded fresh when needed and discarded after the turn.

---

## How to Create a SKILL.md

### File Structure

```
services/agent-go/skills/
├── code-review/
│   └── SKILL.md          # Code review instructions
├── document-search/
│   └── SKILL.md          # RAG search instructions
├── task-management/
│   └── SKILL.md          # Task CRUD instructions
└── web-research/
    └── SKILL.md          # Web search + fetch instructions
```

### SKILL.md Format

Each `SKILL.md` is a Markdown file with YAML frontmatter + body:

```markdown
---
name: code-review
description: Review Go and TypeScript code for bugs, style, and security
version: 1.0.0
triggers:
  - review
  - code review
  - check my code
  - audit
  - refactor
tools:
  - file.read
  - file.search
---

# Code Review Skill

You are a code review specialist. When activated, follow these rules:

## Process
1. Read the files the user mentions using `file.read`
2. Check for:
   - Bugs and logic errors
   - Security issues (SQL injection, XSS, hardcoded secrets)
   - Performance problems (N+1 queries, unnecessary allocations)
   - Style issues (naming, comments, structure)
3. Provide SPECIFIC line references in your feedback
4. Suggest concrete fixes, not vague complaints

## Output Format
- **Severity**: CRITICAL / WARNING / INFO
- **File**: path/to/file.go
- **Line**: 42
- **Issue**: description
- **Fix**: concrete suggestion
```

### Frontmatter Fields

| Field | Required | Type | Description |
|-------|:--------:|------|-------------|
| `name` | Yes | `string` | Unique skill identifier (kebab-case) |
| `description` | Yes | `string` | Human-readable summary — shown when listing skills |
| `version` | No | `string` | Semver version (e.g., `1.0.0`) |
| `triggers` | Yes | `string[]` | Keywords that cause this skill to load. Matched against user input (case-insensitive). |
| `tools` | No | `string[]` | Tool names this skill is authorized to use. If empty, inherits the agent's full tool registry. |

### Body

The body (everything after `---`) is injected directly into the agent's context as system instructions. Write it like you're instructing a smart but junior engineer:

- **Be specific** — "Check for unhandled errors" not "Be thorough"
- **Give examples** — show good vs bad patterns
- **Define output formats** — "Return JSON with fields: severity, file, line, issue, fix"
- **Keep it focused** — one skill = one domain. Create separate skills for separate concerns.

---

## How Skills Are Loaded and Triggered

### Loading Flow

```
┌──────────────────────────────────────────────────────────────┐
│ 1. User sends message                                        │
│    "Please review my auth.go file"                           │
│                                                              │
│ 2. Orchestrator routes to agent                              │
│    If message contains any trigger keyword → load skill      │
│                                                              │
│ 3. Skill loader (internal/skills/)                           │
│    - Reads skills/ directory                                 │
│    - Parses SKILL.md frontmatter                             │
│    - Matches triggers against user input (case-insensitive)  │
│    - Returns matched skill(s)                                │
│                                                              │
│ 4. Context injection                                         │
│    - Skill body is injected into agent context              │
│    - Skill's tool allowlist filters the registry             │
│    - Agent responds with specialized knowledge              │
│                                                              │
│ 5. After turn: skill context discarded                       │
│    Next turn may load different skills                       │
└──────────────────────────────────────────────────────────────┘
```

### Trigger Matching

Triggers are matched case-insensitively against the user's message:

```
User: "audit my React components"
  → Matches trigger "audit" → loads code-review skill

User: "can you review this pull request?"
  → Matches trigger "review" → loads code-review skill

User: "what's the weather?"
  → No match → no skill loaded (default agent behavior)
```

Multiple skills can match simultaneously. They are loaded in order of discovery and concatenated into the context.

### Tool Filtering

When a skill specifies `tools`, only those tools are available to the agent during that turn:

```yaml
tools:
  - file.read
  - file.search
```

This means the agent can ONLY call `file.read` and `file.search` — not `web.search`, `web.fetch`, or any destructive tools. This provides defense-in-depth: even if the LLM hallucinates a tool call, the registry filter prevents it.

---

## Built-in Skills (Planned)

| Skill | Description | Triggers | Status |
|-------|-------------|----------|:------:|
| **code-review** | Review code for bugs, style, security | review, audit, check code, refactor | Planned |
| **document-search** | Search and retrieve from knowledge base | search, find, lookup, document | Planned |
| **task-management** | Create, list, update, delete tasks | task, todo, remind, schedule | Planned |
| **web-research** | Search web and summarize findings | research, google, look up, what is | Planned |
| **file-explorer** | Navigate and inspect project files | list files, show me, find file | Planned |

---

## Skill Loader Package (Upcoming)

The `internal/skills/` package provides:

```go
// Loader doc tat ca SKILL.md trong thu muc skills/.
type Loader struct {
    dir  string               // skills/ directory path
    all  map[string]*Skill    // name -> skill
}

// Skill dai dien cho mot SKILL.md da parse.
type Skill struct {
    Name        string
    Description string
    Version     string
    Triggers    []string
    Tools       []string     // tool allowlist; empty = all tools
    Body        string       // markdown body (instructions)
}

// Load nap toan bo skills tu thu muc.
func (l *Loader) Load() error

// Find tim skills co trigger khop voi user input.
// Tra ve danh sach skill da sap xep theo do uu tien (trigger match count).
func (l *Loader) Find(userInput string) []*Skill

// Render tao system prompt tu mot danh sach skill.
func Render(skills []*Skill) string
```

### Usage in Engine

```go
// In the engine's Run method (simplified):
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error) {
    // Load matching skills
    activeSkills := e.skillLoader.Find(in.UserMessage)

    // Inject into context
    if len(activeSkills) > 0 {
        skillPrompt := skills.Render(activeSkills)
        // Prepend to system or inject as a separate message
        injectSystemMessage(s, skillPrompt)
        emit(Event{Type: "skill", Node: activeSkills[0].Name})
    }

    // Rest of the ReAct loop...
}
```

---

## Skills vs Agent Specialization (Orchestrator)

Don't confuse skills with the multi-agent orchestrator:

| Concept | Skills | Orchestrator Agents |
|---------|--------|---------------------|
| **What it does** | Adds temporary knowledge to an agent | Routes to a specialized agent |
| **Scope** | Per-turn (loaded, used, discarded) | Per-request (agent handles entire request) |
| **Tools** | Can filter/restrict tool access | Each agent has its own tool registry |
| **Memory** | No persistent memory of skill usage | Agent has its own memory store |
| **When to use** | Adding domain expertise on-demand | Complex routing when different domains need different system prompts and tools |

**Combined usage:** The orchestrator routes to the "code" agent, which then loads the "code-review" skill for this specific turn, giving the code agent temporary expertise in code review.

---

## Best Practices for Writing Skills

1. **One skill = one capability** — Don't create a "mega-skill" that covers everything. Small, focused skills compose better.

2. **Write triggers carefully** — Use 5-10 specific keywords. Avoid overly generic triggers like "help" or "do" that would load your skill on every request.

3. **Specify tools you need** — Set `tools: []` to an allowlist. Don't give a code-review skill access to `web.fetch` or `deleteTask`.

4. **Version your skills** — Use semver. Increment MAJOR when changing expected behavior, MINOR when adding capabilities, PATCH for fixes.

5. **Test skills in isolation** — Before deploying a new skill, test it with a variety of user inputs. Verify:
   - Triggers activate on expected messages
   - Triggers DON'T activate on unrelated messages
   - The agent with the skill produces better output than without

6. **Keep skill bodies concise** — Under 500 words is ideal. The body is injected as context, so every word competes with conversation history for the LLM's attention.
