---
name: api-explorer
description: This skill should be used when the user asks to "build out X API", "implement an X client", "integrate with X", or "call the X API". Discovers, fetches, caches, and normalizes third-party API documentation before implementing against any external API. Research only; generates no implementation code.
---

# API Explorer

Discovers, fetches, caches, and normalizes third-party API documentation so you can implement against it with full context. This skill is research only. It does not generate implementation code.

## When to Use

Invoke this skill when the task involves implementing against, integrating with, or building a client for an external API. Trigger phrases include: "build out X API", "implement X client", "integrate with X", "call the X API", or any task that requires understanding a third-party API's endpoints, types, and auth.

This skill runs first, before any implementation skill (`emmaly-skills:go`, `emmaly-skills:svelte`, etc.). Its output (a cached, normalized manifest) is what those skills consume.

## Reference Files

Detailed material is kept out of the always-loaded body. Consult as needed:

- **`references/manifest-format.md`**: full manifest JSON schema (top-level structure, auth, conventions, types, endpoints, dependency graph)
- **`references/scope-and-formats.md`**: scope include/exclude rules and the supported spec-format table with discovery methods and confidence
- **`references/edge-cases.md`**: private/large/HTML-only APIs, spec validation failures, and cache-management operations (list, purge, force refresh, show scopes)

## Cache Location

All cached API documentation lives centrally at `~/.cache/api-explorer/`, shared across projects. Never copy into a project directory unless the user explicitly requests it.

```
~/.cache/api-explorer/
  index.json                        # Global index of all cached APIs
  apis/{slug}/                      # One directory per API (e.g., "stripe", "acme-fishing")
    meta.json                       # API metadata, source URLs, fetch history
    raw/{timestamp}/                # Raw fetched artifacts, one snapshot per fetch
      README.md                     # What was fetched, from where, when
      ...                           # Spec files, HTML pages, proto files, etc.
    manifest.json                   # Latest normalized manifest
    manifest.{timestamp}.json       # Archived previous manifests
    scopes/{scope-slug}.json        # Filtered manifests for specific use cases
```

Create the directory structure with Bash (`mkdir -p`) on first use. Initialize `index.json` as `{"version": 1, "apis": {}}` if it does not exist.

### index.json

```json
{
  "version": 1,
  "apis": {
    "acme-fishing": {
      "name": "Acme Fishing Shop API",
      "path": "apis/acme-fishing",
      "lastFetched": "2026-03-26T14:15:00Z",
      "lastManifest": "2026-03-26T14:16:00Z",
      "sourceUrl": "https://api.acmefishing.com/docs/openapi.json",
      "formats": ["openapi3"]
    }
  }
}
```

### meta.json

```json
{
  "name": "Acme Fishing Shop API",
  "slug": "acme-fishing",
  "sources": [
    {
      "url": "https://api.acmefishing.com/docs/openapi.json",
      "format": "openapi3",
      "etag": "\"abc123\"",
      "lastModified": "2026-03-20T00:00:00Z"
    }
  ],
  "fetchHistory": [
    { "timestamp": "20260326T1415Z", "result": "success", "sources": 1 }
  ]
}
```

## Workflow

Follow these seven phases in order.

### Phase 1: Scope Negotiation

Parse the user's request into three components:

- **API name**: which API to research
- **Scope**: which area of the API (e.g., "order fulfillment", "user management", "payments")
- **Implementation language**: note for later, not relevant to this skill

Formulate a scope statement: what will be fetched and what will be excluded. Example:

> Scope: order endpoints, fulfillment endpoints, authentication, and all shared types these reference. Excluding: catalog, inventory, reporting, analytics.

If the scope is obvious from the user's request, proceed without asking. If ambiguous, confirm with the user. For very large APIs (AWS, GCP, Azure), always require an explicit scope qualifier. Never attempt to fetch the entire API.

### Phase 2: Check Cache

1. Read `~/.cache/api-explorer/index.json` (create the directory structure if it does not exist)
2. Derive the slug from the API name (lowercase, hyphenated, no special characters)
3. Check if the slug exists in the index
4. If cached:
   - Read `meta.json` and check `lastFetched`
   - If fresh (< 7 days old): tell the user "I have cached docs for {name} from {age}. Using cached version." Proceed to Phase 5 or 6.
   - If stale (>= 7 days): note the staleness and plan a refresh in Phase 4
5. If not cached: proceed to Phase 3

### Phase 3: Discovery

Find API documentation sources. Try these in order, stopping when you have a usable spec:

1. **User-provided URL.** If the user gave a doc URL or spec file path, start there.

2. **Well-known spec paths.** If you know the base URL, try fetching (use WebFetch):
   - `/openapi.json`, `/openapi.yaml`, `/openapi/v3`
   - `/swagger.json`, `/swagger.yaml`, `/v2/swagger.json`
   - `/api-docs`, `/docs/api`, `/.well-known/openapi`
   - `/graphql` (with introspection query)
   - `/api/graphql` (with introspection query)

3. **Web search.** Use WebSearch for:
   - `"{api name}" openapi OR swagger OR "api reference" OR "api documentation"`
   - `"{api name}" API site:github.com openapi filetype:json OR filetype:yaml`
   - `"{api name}" developer docs OR "api docs"`

4. **GitHub/repo search.** Many APIs publish specs in public repos. Search for official SDK repos or API spec repos.

5. **HTML doc sites.** If no machine-readable spec is found, fall back to the HTML documentation site. This is the least reliable path, so flag it.

6. **Postman collections.** Many APIs publish official Postman collections. Search for `"{api name}" postman collection` or check `www.postman.com/explore`. If found, curl the collection JSON directly to disk (`~/.cache/api-explorer/apis/{slug}/raw/{timestamp}/postman-collection.json`) then parse it locally. This is the first step that writes into a snapshot, so create the `{timestamp}` directory here using the `YYYYMMDDTHHMMZ` format Phase 4 uses, and let Phase 4 reuse it rather than making a second one. Postman collections contain endpoints, auth config, example requests/responses, and environment variables, which makes them rich source material.

7. **Community-maintained spec registries.** Check [APIs.guru](https://apis.guru/) and their [GitHub repo](https://github.com/APIs-guru/openapi-directory) which aggregates thousands of OpenAPI specs. Also check [SwaggerHub](https://app.swaggerhub.com/search).

8. **SDK source code.** Official SDKs (Go, Python, JS/TS) are often the most accurate documentation. Clone or browse the SDK repo. Type definitions, method signatures, and inline comments reveal endpoints, params, and auth requirements. Look for generated clients (openapi-generator output) which may contain the original spec.

9. **Package registry metadata.** `npm info`, `pip show`, or `go doc` on official SDK packages often link to API docs or source repos.

10. **README and changelog files.** Official API repos frequently document endpoints, auth, and usage examples in their README. Don't overlook these.

11. **Wayback Machine.** If a doc URL is dead or returning errors, try `web.archive.org/web/{url}` for archived versions.

Record all discovered sources and their format type.

> **Resourcefulness principle:** Do everything you can to get the documentation, even if it means unconventional approaches. If a Postman collection exists, curl it directly to disk and parse the file locally. If the only spec lives inside an SDK's generated client, read those source files. If the HTML docs are behind a SPA that WebFetch can't render, check if there's a static build or an API powering the docs page. If a spec URL returns a massive file, download it to the raw cache first rather than trying to hold it in context. The goal is a complete, accurate manifest, so be creative about how you get there.

### Phase 4: Fetch and Store Raw

1. Generate a timestamp: `YYYYMMDDTHHMMZ` (UTC). If Phase 3 already created a snapshot directory for this run, reuse that timestamp instead of generating a second one.
2. Create `apis/{slug}/raw/{timestamp}/` if it does not already exist
3. Fetch each discovered source:

| Format | Action |
|--------|--------|
| OpenAPI/Swagger (JSON/YAML) | Download the spec file directly |
| AsyncAPI | Download the spec file directly |
| GraphQL | Run introspection query, save result as `schema.graphql` or `introspection.json` |
| gRPC/protobuf | Download `.proto` files if available; note if only runtime reflection is possible |
| RAML / API Blueprint | Download the spec file directly |
| HTML doc site | Fetch relevant pages (scoped to user's requested area), save as HTML files |
| Postman Collection | Download JSON, convert to OpenAPI-compatible structures during normalization |
| SDK source code | Clone/download relevant type definition files and client methods |
| APIs.guru / community spec | Download the spec file, then treat as OpenAPI but verify version and accuracy |

> **Large files:** When a spec or collection is large (>1MB), always download it to disk first (`WebFetch` → `Write` to `raw/{timestamp}/`), then read and parse from disk. Never try to hold a massive spec entirely in context. Read it in sections during normalization.

4. Write a `README.md` in the snapshot directory documenting what was fetched and from where
5. Validate: check that specs are parseable (valid JSON/YAML, valid OpenAPI structure, etc.). If broken, note the issue and try alternate sources
6. On refresh: use conditional HTTP requests (`If-None-Match` with ETag, `If-Modified-Since`) when previous values are available in `meta.json`
7. Update `meta.json` with the new fetch entry and source ETags/Last-Modified values
8. Update `index.json` with the new `lastFetched` timestamp

### Phase 5: Parse and Normalize

Transform raw documentation into a structured manifest. Extract these sections:

**API metadata:**
- Name, version, description
- Base URL(s) with environment labels (production, sandbox, staging)
- Spec format and source URL

**Authentication:**
Capture every auth mechanism the API supports. For each:

- **OAuth2**: all supported flows (authorization code, client credentials, etc.), authorization URL, token URL, refresh URL, all available scopes with descriptions
- **API Key**: location (header, query, cookie), parameter name, how to obtain
- **Bearer token**: format (JWT, opaque), how to obtain
- **Mutual TLS**: certificate requirements
- **Other**: any custom auth schemes

Note which mechanism applies to which endpoints (or if one applies globally). Record any notes about auth (e.g., "OAuth2 for user-context, API key for server-to-server").

**Conventions** (cross-cutting patterns, found by examining multiple endpoints):
- Pagination: style (cursor, offset, page), parameter names, response field names, max/default limits
- Rate limits: global limit, per-endpoint limits, header names for limit/remaining/reset
- Error responses: format, structure (fields), common HTTP status codes and their meanings
- ID format: prefixed IDs, UUIDs, numeric, etc.
- Timestamp format: ISO 8601, Unix epoch, etc.
- Common headers: required request headers, useful response headers

**Types:**
Build a flat dictionary of named types. For each type:
- Description
- Fields with type, description, required flag, example values
- Enum values (if enum type)
- Type references use the type name as a string (e.g., `"type": "OrderStatus"`)

**Endpoints:**
For each operation:
- Unique ID (operationId or generated)
- HTTP method and path
- Summary/description
- Auth requirements: required flag, specific scopes needed
- Parameters: path, query, header params with name, type, required flag, default value
- Request body: type reference, required flag, content type
- Response: success status code and type, error status codes
- `dependsOn`: list of endpoint IDs that are operational prerequisites (e.g., "createFulfillment depends on getOrder because it needs an order_id")

**Dependency graph:**
A map of endpoint ID to its prerequisite endpoint IDs. Built from `dependsOn` fields but also from type analysis (if an endpoint's path parameter references an ID that only another endpoint can produce).

Save the manifest as `apis/{slug}/manifest.json`. If a previous manifest exists, archive it as `manifest.{previous-timestamp}.json`. See `references/manifest-format.md` for the full schema.

### Phase 6: Scope Filtering

If the user requested a subset of the API (which is the common case):

1. **Identify matching endpoints**: match on path prefixes, tags, operation IDs, or keywords in summaries/descriptions
2. **Walk transitive dependencies:**
   - For each matched endpoint, include all types it references (params, request body, response)
   - For each included type, include all types it references (field types, array items)
   - For each matched endpoint, include all endpoints in its `dependsOn` chain
   - For each newly included endpoint, repeat the type and dependency walk
3. **Always include:** auth section, conventions section, API metadata
4. **Boundary check:** if the scope pulls in more than 50% of the total endpoints, flag this to the user: "The {scope} scope pulls in {N} of {total} endpoints due to shared dependencies. Proceed with this scope, or use the full API?" For very large APIs (AWS, GCP, Azure), do not offer the full API here. Ask for a narrower scope instead, because the Phase 1 rule against fetching the entire API still applies at this point.
5. Save the filtered manifest as `apis/{slug}/scopes/{scope-slug}.json`

See `references/scope-and-formats.md` for the full include/exclude rules.

### Phase 7: Present Summary

Before handing off to implementation, present a summary:

```
API: {name} v{version}
Base URL: {url}
Auth: {auth methods summary}
Scope: {scope name}
Endpoints: {N in scope} of {total} total
  - {Group 1}: {endpoint list with methods}
  - {Group 2}: {endpoint list with methods}
Key types: {list of main types that will become structs/interfaces}
Conventions: {pagination style}, {error format}, {id format}
Confidence: {high if from machine-readable spec, medium/low if from HTML}
```

If there are gaps or concerns, list them:
- "No machine-readable spec found. Normalized from HTML docs, so it may have inaccuracies"
- "Auth documentation is sparse. May need to consult {link} during implementation"
- "Some response types are undocumented, and are marked as `unknown` in the manifest"

Wait for user confirmation before proceeding. Once confirmed, the scoped manifest is ready for consumption by implementation skills.

## Freshness and Re-use

- **Staleness threshold:** 7 days by default. Mention the age when reusing cached docs.
- **Conditional fetch:** Use `If-None-Match` (ETag) and `If-Modified-Since` headers when refreshing, if previous values are stored in `meta.json`.
- **Snapshots are immutable:** never overwrite a `raw/{timestamp}/` directory. Each fetch creates a new snapshot.
- **Manifest diffing:** when refreshing, compare the new manifest against the previous one and report changes: new endpoints, removed endpoints, changed types, etc.
- **Cross-project:** the cache is global. Any project can use any cached API. Never duplicate into a project unless the user requests it.

## Tool Usage

| Phase | Tools |
|-------|-------|
| Cache check | Read (for index.json, meta.json), Bash (mkdir -p for first-time setup) |
| Discovery | WebSearch, WebFetch (for well-known paths) |
| Fetch | WebFetch (download specs/pages), Write (save to cache) |
| Normalize | Read (raw files), Write (manifest.json) |
| Scope filter | Read (manifest.json), Write (scopes/{slug}.json) |
| Present | Direct text output to user |
