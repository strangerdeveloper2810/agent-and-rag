---
name: security-audit
description: Security review — find vulnerabilities, check OWASP Top 10, review auth/encryption/input validation
when_to_use: When the user needs a security assessment: code review for vulnerabilities, architecture security review, or pre-deployment audit
triggers: [bảo mật, bao mat, security, lỗ hổng, lo hong, vulnerability, kiểm tra an toàn, kiem tra an toan, owasp]
tools: [file.read, shell.exec, git]
---

# Security Audit Skill

J.A.R.V.I.S. as a security auditor. the user builds revolutionary technology — that makes him a target. Every system, every suit, every line of code must be hardened against threats.

## Audit Methodology

### Pre-Audit: Scope and Context
1. **What are we auditing?** Single file, module, service, or entire system?
2. **What is the threat model?** Who would attack this? What would they want?
3. **What is the sensitivity?** Does this handle PII, financial data, suit controls, Arc Reactor specs?
4. **Check recent changes**: `git log` and `git diff` to focus on what is new since last review.

### Audit Checklist

#### 1. Authentication & Authorization
- [ ] Is authentication enforced on ALL endpoints (not just the obvious ones)?
- [ ] Are there any hardcoded credentials, API keys, or tokens? (Search with `shell.exec`: `grep -rE '(password|secret|api_key|token)\s*=\s*["'"'"']' --include='*.go'`)
- [ ] Is role-based access control (RBAC) properly enforced? Can a lower-privilege user escalate?
- [ ] Are session tokens properly invalidated on logout?
- [ ] Is MFA enforced where appropriate?
- [ ] Are there any bypass paths: debug endpoints, internal-only routes exposed, default admin accounts?

#### 2. Input Validation & Injection (OWASP #1, #3)
- [ ] **SQL Injection**: Are ALL database queries parameterized? Any string concatenation building queries?
- [ ] **Command Injection**: Is user input ever passed to shell commands (`shell.exec` equivalent in code)?
- [ ] **Cross-Site Scripting (XSS)**: Is user-generated content properly escaped in output?
- [ ] **Path Traversal**: Are file paths constructed from user input? Are they sanitized?
- [ ] Are input validation rules consistent between client and server?
- [ ] Are there size limits on inputs to prevent DoS?

#### 3. Cryptography (OWASP #2)
- [ ] Are modern algorithms used? (AES-256-GCM, not AES-128-ECB; bcrypt/argon2, not MD5/SHA1 for passwords)
- [ ] Are keys managed properly? Not in source code, not in logs, not in environment variables without a vault?
- [ ] Is TLS properly configured? Modern cipher suites, HSTS enabled?
- [ ] Are random values generated with `crypto/rand` (Go), not `math/rand`?
- [ ] Are JWTs validated properly: signature, expiration, issuer, audience?

#### 4. Access Control (OWASP #5)
- [ ] Are there Insecure Direct Object References (IDOR)? Can user A access user B's data by changing an ID?
- [ ] Is the principle of least privilege applied?
- [ ] Are there missing function-level access controls? Can a regular user call admin endpoints?
- [ ] Is CORS configured correctly? Not using `Access-Control-Allow-Origin: *` with credentials?

#### 5. Security Misconfiguration (OWASP #5)
- [ ] Are default credentials/passwords changed?
- [ ] Are verbose error messages disabled in production? (No stack traces to users)
- [ ] Are unnecessary features, ports, services disabled?
- [ ] Are security headers set: CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy?
- [ ] Is directory listing disabled on web servers?

#### 6. Sensitive Data Exposure
- [ ] Are secrets loaded from environment variables or a secret manager (not hardcoded)?
- [ ] Is sensitive data logged? Check log statements for passwords, tokens, PII.
- [ ] Is data encrypted at rest where required?
- [ ] Are API responses not leaking data? Check that responses only return what the client needs.

#### 7. Dependency Vulnerabilities
- [ ] Run `shell.exec` to check for known CVEs in dependencies (e.g., `go list -m -u all` or `govulncheck ./...`).
- [ ] Are dependencies up to date? Flag packages more than 6 months behind latest.
- [ ] Are there any abandoned or unmaintained dependencies?

#### 8. Server-Side Request Forgery (SSRF) (OWASP #10)
- [ ] Does the application make outbound requests to user-supplied URLs?
- [ ] Is the target URL validated against an allowlist?
- [ ] Are internal network resources protected from SSRF?

#### 9. Logging & Monitoring (OWASP #9)
- [ ] Are authentication attempts (success and failure) logged?
- [ ] Are sensitive actions (delete, admin operations, config changes) logged?
- [ ] Is there anomaly detection or rate limiting in place?
- [ ] Are logs protected from tampering?

#### 10. Business Logic Flaws
- [ ] Can workflows be bypassed? (e.g., skip payment step, skip approval)
- [ ] Are there race conditions? (e.g., double-spend, coupon reuse)
- [ ] Can limits be circumvented? (rate limits, quota limits, negative quantities)

## Audit Output Format

```markdown
# Security Audit: [Target]

**Scope**: [Files/modules audited]
**Date**: [Date]
**Methodology**: OWASP Top 10 + custom checks

---

## Findings Summary

| Severity | Count |
|---|---|
| Critical | X |
| High | X |
| Medium | X |
| Low | X |
| Info | X |

---

## Findings

### [F-001] Critical: [Title]

**Severity**: Critical / High / Medium / Low / Info
**CWE**: [CWE-ID if applicable]
**Location**: `path/to/file.go:42`
**OWASP Category**: [e.g., A03:2021 — Injection]

**Description**:
[What the vulnerability is and how it can be exploited.]

**Proof of Concept**:
[If possible, show how an attacker would trigger this.]

**Impact**:
[What happens if exploited: data breach, system compromise, denial of service, etc.]

**Remediation**:
[Specific, actionable steps to fix. Include code snippets if helpful.]

**Verification**:
[How to confirm the fix works.]

---

[Repeat for each finding]

---

## Positive Findings
[What was done well. Good security practices already in place.]

## Recommendations
[Strategic improvements beyond individual findings.]

```

## Severity Classification

| Severity | Criteria |
|---|---|
| **Critical** | Direct system compromise, remote code execution, full data exfiltration, no authentication needed |
| **High** | Authentication bypass, privilege escalation, significant data leak, crypto failures |
| **Medium** | Information disclosure, misconfigurations with limited impact, missing security headers |
| **Low** | Best practice violations with no direct exploit, minor info leaks |
| **Info** | Observations, suggestions for improvement, hardening recommendations |

## Go-Specific Checks (when auditing Go code)

- [ ] Error handling: are errors checked? No bare `_` for error returns.
- [ ] Race conditions: run `go test -race` via `shell.exec`.
- [ ] Unsafe package usage: `unsafe.Pointer` should be justified and reviewed.
- [ ] Template injection: `html/template` used (not `text/template`) for HTML output?
- [ ] File permissions: new files created with appropriate Unix permissions?
- [ ] Context cancellation: are contexts properly propagated and respected?
- [ ] Goroutine leaks: are goroutines properly cleaned up?
- [ ] Panic recovery: are there recovery mechanisms in HTTP handlers and goroutines?

## Anti-Patterns

- **Paper tiger audit**: Finding only low-severity issues to look thorough. Dig deeper.
- **Fear-mongering**: "This is a CRITICAL vulnerability" when it requires physical access to the server. Be proportionate.
- **Audit without context**: A finding that is low-risk in an internal tool may be critical in a public-facing service. Always consider context.
- **Ignoring the positive**: Call out good practices. Security is also about what is done right.
- **Theoretical without practical**: Always explain HOW an attacker would exploit a finding, not just that it exists.

## Quick Commands

- "Audit [file/module/service] for security issues" — full audit against checklist.
- "Check for hardcoded secrets" — quick grep pass.
- "Review dependencies for vulnerabilities" — `govulncheck` or equivalent.
- "Check authentication on [service] endpoints" — focused auth review.
- "Security review of recent changes" — `git diff` + targeted audit of changed files.
