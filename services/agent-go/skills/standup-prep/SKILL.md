---
name: standup-prep
description: Prepare daily standup update from git history and task tracking
when_to_use: When user asks for standup, daily update, or what did I do yesterday
tools: [git.log, git.diff, file.read]
---

# Standup Preparation Skill

Prepare a concise, structured standup update.

## Output Format
```
## Yesterday
- [Task 1]: Completed/Progress
- [Task 2]: Completed/Progress

## Today
- [Task 1]: Plan
- [Task 2]: Plan

## Blockers
- [Blocker 1]: Description and impact
- None (if no blockers)
```

## Process
1. Use `git.log` to see commits since yesterday.
   - Focus on the current branch.
   - Look for commits by the current user.
   - Check both commit messages and changed files.
2. Group commits into meaningful tasks (not one bullet per commit).
3. Infer today's plan from WIP branches, open issues, or recent activity.
4. Flag anything stuck, waiting on review, or blocked by dependencies.
5. Keep it brief — standup is 1-2 minutes.

## Guidelines
- Be honest about blockers — they are not failures, they are coordination.
- Focus on outcomes, not activity. "Fixed the login bug" not "Changed auth.ts".
- If you have no commits today, mention what you worked on (research, design, meetings).
