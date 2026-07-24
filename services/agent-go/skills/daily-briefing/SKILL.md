---
name: daily-briefing
description: Morning briefing with weather, calendar, tasks, and news headlines
when_to_use: When user asks for morning briefing, daily summary, or start of day overview
tools: [web.search, web.fetch, calendar.read, memory.recall]
---

# Daily Briefing Skill

Provide a structured morning briefing to start the day informed.

## Output Format
```
## Weather — [Location]
- Current: [temp], [condition]
- Today: [high]/[low], [forecast]
- Alerts: [any weather warnings]

## Calendar — [Date]
- [09:00] Meeting with [person] re: [topic]
- [14:00] [Event name]
- No more events today (or continue listing)

## Tasks — Priority
1. [Task] — [Due date / Context]
2. [Task] — [Due date / Context]

## News Headlines
- [Headline 1] — [Source]
- [Headline 2] — [Source]
- [Headline 3] — [Source]

## Memory Context
- [Relevant memory / user preference to keep in mind today]
```

## Process
1. Use `web.search` to get weather for user's location and today's top news.
2. Check calendar for today's events (integrate with calendar tool when available).
3. Review task list and prioritize (integrate with task tracking when available).
4. Use `memory.recall` to surface relevant context about the user.
5. Present in a clean, scannable format (not a wall of text).

## Guidelines
- Time-box the briefing: it should take under 1 minute to read.
- Prioritize actionable information over background.
- If a data source is unavailable (no calendar integration yet), note it gracefully.
- Adapt to user preferences: some want weather first, others want tasks first.
