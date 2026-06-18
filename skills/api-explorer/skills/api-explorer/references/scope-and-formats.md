# Scope Rules and Supported Formats

## Scope Rules

**Always include:**
- Authentication mechanisms and details
- API conventions (pagination, errors, rate limits)
- API metadata (name, version, base URLs)

**Include by dependency walk:**
- Endpoints matching the user's scope (by path, tag, or keyword)
- All types referenced by included endpoints (transitive)
- All prerequisite endpoints from `dependsOn` chains (transitive)
- All types referenced by prerequisite endpoints (transitive)

**Exclude:**
- Endpoints outside the scope that are not dependencies
- Types only referenced by excluded endpoints
- Webhook/event definitions (unless the scope involves receiving events)
- Admin/management endpoints (unless explicitly requested)
- Deprecated endpoints (unless no non-deprecated alternative exists)

## Supported Formats

| Format | Discovery Method | Confidence |
|--------|-----------------|------------|
| OpenAPI 3.x | Well-known paths, web search, GitHub | High |
| Swagger / OpenAPI 2.0 | Well-known paths, web search, GitHub | High |
| AsyncAPI | Well-known paths, web search, GitHub | High |
| GraphQL | Introspection query at `/graphql` | High |
| gRPC / Protobuf | `.proto` file download, buf registry | High |
| RAML | Web search, GitHub | High |
| API Blueprint | Web search, GitHub | Medium |
| HTML documentation | Web scraping of doc pages | Low — flag in summary |
| Postman Collection v2.x | Postman search, web search, `{api}.postman.co` | Medium — covers endpoints and examples but may lack full type schemas |

When multiple formats are available, prefer machine-readable specs over HTML. Prefer OpenAPI 3.x over Swagger 2.0.
