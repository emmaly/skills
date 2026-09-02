# Manifest Format Reference

The manifest is a single JSON file. Top-level structure:

```json
{
  "manifestVersion": 2,
  "generatedAt": "ISO 8601 timestamp",
  "sourceSnapshot": "timestamp of raw snapshot used",
  "api": {
    "name": "string",
    "slug": "string",
    "version": "string",
    "description": "string",
    "baseUrls": [{"url": "string", "environment": "string"}],
    "specFormat": "openapi3 | openapi2 | asyncapi | graphql | grpc | raml | blueprint | html",
    "specUrl": "string"
  },
  "auth": {
    "mechanisms": [
      {
        "type": "oauth2 | apiKey | bearer | mutualTls | custom",
        "...": "mechanism-specific fields"
      }
    ],
    "requiredForAllEndpoints": "boolean",
    "notes": "string"
  },
  "conventions": {
    "pagination": {"style": "string", "params": {}, "responseFields": {}, "maxLimit": 0, "defaultLimit": 0},
    "rateLimits": {"global": "string", "headers": {}},
    "errors": {"format": "string", "structure": {}, "commonCodes": {}},
    "idFormat": "string",
    "timestamps": "string"
  },
  "types": {
    "TypeName": {
      "description": "string",
      "fields": {"fieldName": {"type": "string", "description": "string", "required": "boolean", "example": "any"}},
      "type": "enum (only if enum)",
      "values": ["only if enum"]
    }
  },
  "endpoints": [
    {
      "id": "string",
      "method": "GET | POST | PUT | PATCH | DELETE",
      "path": "string",
      "summary": "string",
      "tags": ["string"],
      "auth": {"required": "boolean", "scopes": ["string"]},
      "params": {
        "path": [{"name": "string", "type": "string", "required": "boolean"}],
        "query": [{"name": "string", "type": "string", "required": "boolean", "default": "any"}],
        "header": [{"name": "string", "type": "string", "required": "boolean"}]
      },
      "requestBody": {"type": "string", "required": "boolean", "contentType": "string"},
      "response": {
        "success": {"status": 0, "type": "string"},
        "errors": [0]
      },
      "dependsOn": ["endpoint IDs"]
    }
  ],
  "dependencyGraph": {
    "endpointId": ["prerequisite endpoint IDs"]
  }
}
```
