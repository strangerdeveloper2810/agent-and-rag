---
name: debug
description: Systematic debugging: reproduce, isolate, identify, fix, verify
when_to_use: When user reports a bug, error, crash, or unexpected behavior
triggers: [debug, sửa lỗi, sua loi, bị lỗi, bi loi, báo lỗi, bao loi, không chạy, khong chay, crash, stack trace, tại sao lỗi, tai sao loi]
tools: [shell.exec, file.read, git.log, git.diff]
---

# Debugging Skill

Apply a systematic debugging methodology to find and fix issues quickly.

## The 5-Step Debug Process

### 1. Reproduce
- Ask the user for exact steps, inputs, and environment.
- Reproduce the bug yourself to confirm it exists.
- Note: what is the expected behavior vs actual behavior?

### 2. Isolate
- Narrow down the scope: which component/module/function is responsible?
- Use `git.log` to check recent changes that might have introduced the bug.
- Use `git.diff` to see what changed in suspect files.
- Create a minimal reproduction case.

### 3. Identify
- Form a hypothesis about the root cause.
- Use `shell.exec` to run tests, add debug logging, or check outputs.
- Use `file.read` to examine the code around the suspected issue.
- Validate or reject the hypothesis with evidence.

### 4. Fix
- Make the smallest possible change that fixes the root cause.
- Do NOT refactor unrelated code while fixing.
- Add a test that reproduces the bug (to prevent regression).

### 5. Verify
- Run the existing test suite: `shell.exec` the test command.
- Confirm the original reproduction steps now produce expected behavior.
- Check for side effects: did the fix break anything else?

## Anti-Patterns to Avoid
- Guessing at the cause without evidence.
- Changing multiple things at once ("shotgun debugging").
- Fixing symptoms instead of root causes.
- Skipping the test — if it's not tested, it's not fixed.

## Communication
- Keep the user updated at each step: "I can reproduce the bug. Now isolating..."
- When you find the root cause, explain it clearly.
- When fixed, summarize: what was wrong, what you changed, and why.
