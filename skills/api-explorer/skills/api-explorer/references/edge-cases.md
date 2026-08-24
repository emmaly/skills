# Edge Cases and Cache Management

## Edge Cases

**Private/authenticated doc sites:** If fetching returns 401 or 403, ask the user to either provide credentials, a pre-authenticated URL, or a locally downloaded copy of the spec file.

**Very large APIs (AWS, GCP, Azure, etc.):** These are mega-APIs with hundreds of services. Never attempt to fetch the whole thing. Always require a specific service name as part of the scope (e.g., "AWS S3" not "AWS").

**HTML-only APIs:** When no machine-readable spec exists, normalize from HTML. Flag the manifest with `"confidence": "low"` in the `api` section and include a note in the Phase 7 summary. Expect inaccuracies, and recommend the user verify critical types and auth details.

**Context window pressure:** Only load the scoped manifest into context, not the full manifest. If even the scoped manifest is very large, summarize the types section and keep the full endpoints list.

**Spec validation failures:** If a fetched spec is malformed (invalid JSON, broken YAML, non-conformant OpenAPI), note the specific issue, try alternate sources, and if no valid source exists, fall back to HTML doc scraping.

## Cache Management

These operations can be requested in natural language:

- **List cached APIs:** Read `index.json` and display all cached APIs with name, age, and scope count
- **Purge a cached API:** Remove the `apis/{slug}/` directory and its entry from `index.json`
- **Force refresh:** Re-run Phases 3-5 regardless of cache age, creating a new snapshot
- **Show cached scopes:** List all `scopes/*.json` files for a given API
