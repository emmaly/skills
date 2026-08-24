---
name: home-assistant
description: This skill should be used when interacting with Home Assistant. Covers REST and WebSocket API access patterns, Bearer-token auth, and common commands for states, services, dashboards, logs, and HACS.
---

## Credentials

API URL and long-lived access token are stored in `.secrets/.env` (or `~/.config/home-assistant/.env`):

```env
HASS_API_URL=http://your-ha-host:8123
HASS_API_KEY=your_long_lived_access_token
```

Load these into the environment before making requests. The user's `fish` shell cannot `source` a `.env` (it uses `export KEY=VALUE` syntax), so use `envwith`:

```
envwith -f .secrets/.env -- <command> [args...]
```

## REST API

### Authentication

All REST requests use a Bearer token:

```bash
curl -H "Authorization: Bearer $HASS_API_KEY" "$HASS_API_URL/api/..."
```

### Useful Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/` | GET | API running check (returns `{"message": "API running."}`) |
| `/api/config` | GET | Server configuration |
| `/api/states` | GET | All entity states |
| `/api/states/<entity_id>` | GET | Single entity state |
| `/api/states/<entity_id>` | POST | Update entity state |
| `/api/services` | GET | Available services by domain |
| `/api/services/<domain>/<service>` | POST | Call a service |
| `/api/events` | GET | List event types |
| `/api/events/<event_type>` | POST | Fire an event |
| `/api/history/period/<timestamp>` | GET | State history |
| `/api/template` | POST | Render a Jinja2 template |

### Calling a Service

```bash
curl -X POST \
  -H "Authorization: Bearer $HASS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"entity_id": "light.living_room"}' \
  "$HASS_API_URL/api/services/light/turn_on"
```

## WebSocket API

WebSocket gives full access to commands not available via REST (dashboards, logs, HACS, etc.).

### Connection URL

Derive from `HASS_API_URL` by replacing `http` with `ws` (or `https` with `wss`) and append `/api/websocket`:

```
ws://your-ha-host:8123/api/websocket
```

### Auth Handshake

1. Connect: server sends `{"type": "auth_required"}`
2. Send: `{"type": "auth", "access_token": "<HASS_API_KEY>"}`
3. Server responds `{"type": "auth_ok"}` on success

### Message IDs

Every command message requires an `id` field, a sequential integer starting from 1. Each message in a session must use a unique, incrementing ID.

### Useful Commands

```jsonc
// Lovelace dashboard config
{"id": 1, "type": "lovelace/config", "url_path": "lovelace"}

// Lovelace frontend resources (HACS-managed, etc.)
{"id": 2, "type": "lovelace/resources"}

// List all dashboards
{"id": 3, "type": "lovelace/dashboards/list"}

// System log entries
{"id": 4, "type": "system_log/list"}

// Persistent notifications
{"id": 5, "type": "persistent_notification/get"}

// Repair issues
{"id": 6, "type": "repairs/list_issues"}

// HACS info (if HACS installed)
{"id": 7, "type": "hacs/info"}

// Subscribe to state changes (streams events)
{"id": 8, "type": "subscribe_events", "event_type": "state_changed"}

// Get entity states (same as REST /api/states)
{"id": 9, "type": "get_states"}

// Call a service via WebSocket
{"id": 10, "type": "call_service", "domain": "light", "service": "turn_on", "service_data": {"entity_id": "light.living_room"}}
```

## Python WebSocket Example

Requires the `websockets` package. On modern Linux a bare `pip install` is blocked by PEP 668 (`externally-managed-environment`); install into a venv or with `uv`:

```bash
uv pip install websockets
# or: python -m venv .venv && .venv/bin/pip install websockets
```

```python
import asyncio
import json
import os
import websockets

async def ha_websocket():
    api_url = os.environ["HASS_API_URL"]
    api_key = os.environ["HASS_API_KEY"]
    # Anchor the swap at the scheme so hosts containing "http" aren't mangled
    ws_url = ("wss" + api_url[5:] if api_url.startswith("https") else "ws" + api_url[4:]) + "/api/websocket"

    async with websockets.connect(ws_url) as ws:
        # Auth
        await ws.recv()  # auth_required
        await ws.send(json.dumps({"type": "auth", "access_token": api_key}))
        auth_resp = json.loads(await ws.recv())
        assert auth_resp["type"] == "auth_ok"

        # Send a command
        await ws.send(json.dumps({"id": 1, "type": "get_states"}))
        result = json.loads(await ws.recv())
        print(json.dumps(result, indent=2))

asyncio.run(ha_websocket())
```

## Tips

- REST API is good for simple state reads and service calls
- WebSocket is required for: dashboard configs, system logs, HACS, subscriptions, and any command not exposed via REST
- Entity IDs follow the pattern `<domain>.<object_id>` (e.g., `light.kitchen`, `sensor.temperature`)
- Use `/api/template` to test Jinja2 templates before putting them in automations
- Service calls return the resulting states of affected entities
