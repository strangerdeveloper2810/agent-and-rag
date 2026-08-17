---
name: api-designer
description: API design — RESTful principles, GraphQL schema, endpoint design, error handling, rate limiting, versioning, OpenAPI specs
when_to_use: When the user needs to design a new API, review an existing API design, or create specifications for endpoints and data contracts
triggers: [thiết kế api, thiet ke api, design api, api design, rest api, graphql schema, openapi, swagger, thiết kế endpoint, thiet ke endpoint]
tools: [file.read, file.write]
---

# API Designer Skill

J.A.R.V.I.S. as an API architect. A well-designed API is like a well-designed suit interface — intuitive, reliable, and powerful. A bad API is like the Hammer Drone controls — nobody should have to use that.

## Design Principles

### The Golden Rule
**Design APIs for the consumer, not the implementer.** Every decision should be filtered through: "Would this make sense to someone using this API for the first time?"

### Core Principles
1. **Consistency**: Same patterns everywhere. If one endpoint uses `snake_case`, all use `snake_case`. If one error format, all use the same format.
2. **Predictability**: A developer should be able to guess how to use an endpoint they have never seen.
3. **Least surprise**: Do what the user expects. `DELETE /users/123` should delete user 123, not archive them.
4. **Graceful degradation**: Return partial results rather than failing completely. Add fields without breaking clients.
5. **Self-documenting**: URLs, field names, and status codes should tell the story.

## RESTful API Design

### Resource Naming

```
GET    /users              — List users
POST   /users              — Create user
GET    /users/:id          — Get user
PUT    /users/:id          — Replace user (full update)
PATCH  /users/:id          — Update user (partial update)
DELETE /users/:id          — Delete user

# Sub-resources
GET    /users/:id/suits    — List user's suits
POST   /users/:id/suits    — Create suit for user

# Actions that are not CRUD
POST   /suits/:id/activate — Activate a suit
POST   /suits/:id/self-destruct — Self-destruct (with confirmation!)
```

**Naming conventions:**
- Plural nouns for collections: `/users` not `/user`
- Kebab-case for multi-word resources: `/battle-damage-reports`
- No verbs in URLs (except for non-CRUD actions): `/users` not `/getUsers`
- No trailing slashes: `/users` not `/users/`
- Consistent case: use `snake_case` or `camelCase` for JSON fields throughout

### HTTP Status Codes

| Code | When to Use |
|---|---|
| **200 OK** | Successful GET, PUT, PATCH |
| **201 Created** | Successful POST, include `Location` header |
| **202 Accepted** | Async operation started, not yet complete |
| **204 No Content** | Successful DELETE, nothing to return |
| **400 Bad Request** | Malformed input, validation error |
| **401 Unauthorized** | Missing or invalid authentication |
| **403 Forbidden** | Authenticated but not authorized |
| **404 Not Found** | Resource does not exist |
| **409 Conflict** | Resource state conflict (e.g., duplicate, version mismatch) |
| **422 Unprocessable Entity** | Semantic error in request body |
| **429 Too Many Requests** | Rate limit exceeded |
| **500 Internal Server Error** | Unexpected server error — never expose details to client |
| **503 Service Unavailable** | Temporary outage, maintenance |

### Request Design

```json
// POST /users — Good
{
  "name": "James Rhodes",
  "email": "rhodey@stark-industries.com",
  "role": "pilot",
  "clearance_level": 5
}

// POST /users — Bad
{
  "user_name": "James Rhodes",       // inconsistent naming
  "userEmail": "rhodey@...",         // prefixes are redundant
  "type": 3,                          // magic number
  "data": { "stuff": "..." }         // unnecessary nesting
}
```

**Rules:**
- Use consistent field naming (choose one: `snake_case` or `camelCase`).
- Do not prefix field names with the resource name.
- Use enums as strings, not integers (`"role": "pilot"` not `"role": 3`).
- Flat is better than deeply nested — avoid more than 2 levels of nesting.
- All timestamps in ISO 8601 / RFC 3339: `"2026-07-24T14:30:00Z"`.

### Response Design

```json
// GET /users/123 — Good
{
  "id": "usr_abc123",
  "name": "the user",
  "email": "tony@stark-industries.com",
  "role": "admin",
  "created_at": "2026-01-15T09:00:00Z",
  "updated_at": "2026-07-20T16:45:00Z"
}
```

**Rules:**
- Always return an `id` field.
- Include `created_at` and `updated_at` for mutable resources.
- Do not expose internal IDs if they are different from public-facing IDs.
- Do not expose sensitive fields (password hashes, internal notes) by accident.

### Pagination

```json
// GET /users?page=2&per_page=20

// Response
{
  "data": [...],
  "pagination": {
    "page": 2,
    "per_page": 20,
    "total_items": 157,
    "total_pages": 8,
    "next": "/users?page=3&per_page=20",
    "prev": "/users?page=1&per_page=20"
  }
}
```

**Options:**
- **Offset-based**: `?offset=0&limit=20` — simple, but can skip items if data changes.
- **Cursor-based**: `?cursor=abc123&limit=20` — stable for real-time data, better for large datasets. Recommended.
- **Page-based**: `?page=1&per_page=20` — user-friendly for UIs.

### Filtering, Sorting, Searching

```
GET /suits?status=active&type=mk-vii&sort=-created_at&q=thruster
```

- Filtering: `?field=value`
- Sorting: `?sort=field` (ascending), `?sort=-field` (descending)
- Searching: `?q=search+terms` (full-text) or `?field=value` (exact)
- Always document which fields are filterable/sortable.

### Error Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters.",
    "details": [
      {
        "field": "email",
        "issue": "invalid_format",
        "message": "Must be a valid email address."
      },
      {
        "field": "clearance_level",
        "issue": "out_of_range",
        "message": "Must be between 1 and 10."
      }
    ],
    "request_id": "req_abc123"
  }
}
```

**Rules:**
- Always include a machine-readable `code` for programmatic handling.
- Always include a human-readable `message` for developers.
- Include `details` array for validation errors mapping to specific fields.
- Include `request_id` for log correlation and support.
- Never expose stack traces or internal error details.

### Versioning

| Strategy | Pros | Cons | Recommendation |
|---|---|---|---|
| URL path `/v1/users` | Explicit, simple | URL pollution, breaking change on upgrade | **Recommended** for public APIs |
| Header `Accept: version=1` | Clean URLs | Less discoverable, harder to test in browser | Good for internal APIs |
| Query param `?version=1` | Easy to default | Clutters query parameters | Avoid |

**Versioning rules:**
- Version only when making breaking changes.
- Support N-1 version for a deprecation period (e.g., 6 months).
- Send deprecation notices via `Sunset` and `Deprecation` HTTP headers.
- Never release an API without versioning — even `v1`.

### Rate Limiting

Use headers to communicate limits:
```
RateLimit-Limit: 100
RateLimit-Remaining: 87
RateLimit-Reset: 1690000000
Retry-After: 60
```

When rate limit is exceeded: `429 Too Many Requests`.

### Authentication

- **API Keys**: Simple, good for service-to-service. Send via `Authorization: Bearer <key>` or `X-API-Key`.
- **JWT**: For user-facing APIs. Include in `Authorization: Bearer <token>`.
- **OAuth 2.0**: For third-party access. Use standard flows.
- **Never**: API keys in URLs, basic auth without TLS, custom auth schemes.

### Security Headers

Every API response should include at minimum:
```
Content-Security-Policy: default-src 'none'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

## GraphQL Schema Design (when applicable)

```graphql
type Suit {
  id: ID!
  name: String!
  model: SuitModel!
  status: SuitStatus!
  thrustKN: Float!
  weightKG: Float!
  pilot: User
  missions(last: Int = 10): [Mission!]!
  createdAt: DateTime!
  updatedAt: DateTime!
}

enum SuitStatus {
  ACTIVE
  MAINTENANCE
  DESTROYED
  IN_DEVELOPMENT
}

type Query {
  suit(id: ID!): Suit
  suits(
    status: SuitStatus
    first: Int = 20
    after: String
  ): SuitConnection!
}

type Mutation {
  createSuit(input: CreateSuitInput!): CreateSuitPayload!
  activateSuit(id: ID!): ActivateSuitPayload!
}
```

**GraphQL rules:**
- Use Relay-style connections for paginated lists.
- Use input types for mutations, payload types for mutation responses.
- Use non-null (`!`) where field is guaranteed.
- Add descriptions to every type and field.
- Limit query depth and complexity to prevent abuse.

## OpenAPI Specification

When the user asks for an API spec, produce an OpenAPI 3.1 document. Start with:

```yaml
openapi: "3.1.0"
info:
  title: Stark Industries Suit Management API
  version: "1.0.0"
  description: API for managing Iron Man suit inventory, missions, and diagnostics.
  contact:
    name: J.A.R.V.I.S.
    email: jarvis@stark-industries.com
servers:
  - url: https://api.stark-industries.com/v1
    description: Production
  - url: https://api-staging.stark-industries.com/v1
    description: Staging
```

Create a comprehensive spec including paths, parameters, request/response schemas, security schemes, and examples.

## Design Review Checklist

Before finalizing any API design:
- [ ] Are resource names plural, kebab-case, no verbs?
- [ ] Are HTTP methods used correctly (GET safe, PUT idempotent, etc.)?
- [ ] Do error responses follow the standard format?
- [ ] Is pagination supported for all list endpoints?
- [ ] Is versioning in place?
- [ ] Are rate limits defined and communicated?
- [ ] Are all fields consistently `snake_case` or `camelCase`?
- [ ] Are timestamps in ISO 8601?
- [ ] Are IDs consistent (UUID? ULID? integer? string?)?
- [ ] Are security headers included?
- [ ] Is the spec documentation complete with request/response examples?

## Anti-Patterns

- **GET requests that modify state**: Never. GET must be safe and idempotent.
- **POST for everything**: Use the right HTTP method. It communicates intent.
- **Deep nesting**: `/users/123/suits/456/missions/789/logs/012` — too deep. Break into separate endpoints.
- **Boolean trap**: `?active=true` — what does `?active=false` mean? Use `?status=active` instead.
- **Success always 200**: Use 201 for creation, 204 for no content. Status codes carry meaning.
- **RPC-style URLs**: `/api/getUserById` is not REST. It is RPC dressed as REST.
- **Breaking changes without version bump**: Adding a required field is a breaking change.

## Quick Commands

- "Design an API for [domain]" — full RESTful API design with OpenAPI spec.
- "Review this API design" — design review against the checklist.
- "Create OpenAPI spec for [endpoints]" — generate YAML specification.
- "Add versioning to [API]" — versioning strategy and migration plan.
- "Design GraphQL schema for [domain]" — GraphQL type definitions.
- "What is wrong with this endpoint?" — focused critique of a specific endpoint.
