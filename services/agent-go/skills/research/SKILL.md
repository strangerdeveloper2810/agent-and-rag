---
name: research
description: Deep research on a topic with web search, synthesis, and cited sources
when_to_use: When user asks for research, look up information, or find answers online
tools: [web.search, web.fetch, memory.save, file.write]
---

# Research Skill

Perform deep, structured research on a topic and synthesize findings with cited sources.

## Process
1. **Understand the question** — clarify scope, depth, and what the user already knows.
2. **Search broadly** — use `web.search` with multiple query angles.
3. **Read deeply** — use `web.fetch` on the most promising results.
4. **Cross-reference** — verify claims across at least 2 independent sources.
5. **Synthesize** — write a clear, structured summary.
6. **Cite sources** — every factual claim should have a source link.

## Output Format
```
## Research: [Topic]

### Key Findings
- [Finding 1] — [Source](url)
- [Finding 2] — [Source](url)

### Detailed Analysis
[Structured synthesis of findings]

### Conflicting Information
- [Claim A] vs [Claim B] — resolution or note that it's unresolved

### Sources
1. [Title] — [URL]
2. [Title] — [URL]

### Further Reading (optional)
- [Link] — why this might be useful
```

## Guidelines
- Prioritize authoritative sources: official docs, academic papers, reputable publications.
- Note the date of information — what was true in 2023 may not be true today.
- Be transparent about uncertainty: "Sources disagree on..." or "This claim appears in only one source."
- Save noteworthy findings to memory with `memory.save` so the user can recall them later.
- Time-box: for a quick question, 2-3 sources is enough. For deep research, 5-10 sources.
