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

```text
ws://your-ha-host:8123/api/websocket
```

### Auth Handshake

1. Connect: server sends `{"type": "auth_required"}`
2. Send: `{"type": "auth", "access_token": "<HASS_API_KEY>"}`
3. Server responds `{"type": "auth_ok"}` on success

### Message IDs

Every command the client sends after authentication needs a unique integer `id`. Uniqueness is the whole requirement. Counting up from 1 is the usual client convention, not a rule, so do not build anything that depends on the server enforcing it.

The requirement covers commands the client sends, not everything on the socket. Server responses and subscription events carry the `id` of the command they answer, which is how a client correlates them. A repeated `id` arriving from the server is correct and must not be rejected.

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

## Go WebSocket Example

```bash
go mod init github.com/emmaly/ha-example
go get github.com/gorilla/websocket
```

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// message covers the handful of fields this exchange needs. Result stays raw
// because its shape depends on the command.
type message struct {
	ID          int             `json:"id,omitempty"`
	Type        string          `json:"type"`
	AccessToken string          `json:"access_token,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// websocketURL swaps the scheme and appends the socket path. Anchoring at the
// scheme leaves a host that happens to contain "http" intact.
func websocketURL(apiURL string) (string, error) {
	switch {
	case strings.HasPrefix(apiURL, "https://"):
		return "wss://" + strings.TrimPrefix(apiURL, "https://") + "/api/websocket", nil
	case strings.HasPrefix(apiURL, "http://"):
		return "ws://" + strings.TrimPrefix(apiURL, "http://") + "/api/websocket", nil
	default:
		return "", fmt.Errorf("no http scheme in %q", apiURL)
	}
}

// run authenticates, asks for entity states, and prints the result.
func run(ctx context.Context) error {
	url, err := websocketURL(os.Getenv("HASS_API_URL"))
	if err != nil {
		return err
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", url, err)
	}
	defer conn.Close()

	// gorilla reads and writes do not consult the context, so the dial timeout
	// would be the only thing the context covered. Carry it onto the socket.
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("set write deadline: %w", err)
		}
	}

	var required message
	if err := conn.ReadJSON(&required); err != nil {
		return fmt.Errorf("read auth_required: %w", err)
	}

	auth := message{Type: "auth", AccessToken: os.Getenv("HASS_API_KEY")}
	if err := conn.WriteJSON(auth); err != nil {
		return fmt.Errorf("send auth: %w", err)
	}

	var result message
	if err := conn.ReadJSON(&result); err != nil {
		return fmt.Errorf("read auth result: %w", err)
	}
	if result.Type != "auth_ok" {
		return fmt.Errorf("authentication rejected: %s", result.Type)
	}

	if err := conn.WriteJSON(message{ID: 1, Type: "get_states"}); err != nil {
		return fmt.Errorf("send get_states: %w", err)
	}

	var states message
	if err := conn.ReadJSON(&states); err != nil {
		return fmt.Errorf("read states: %w", err)
	}

	pretty, err := json.MarshalIndent(states.Result, "", "  ")
	if err != nil {
		return fmt.Errorf("format states: %w", err)
	}
	fmt.Println(string(pretty))
	return nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := run(ctx); err != nil {
		slog.Error("home assistant websocket", "err", err)
		os.Exit(1)
	}
}
```

## Tips

- REST API is good for simple state reads and service calls
- WebSocket is required for: dashboard configs, system logs, HACS, subscriptions, and any command not exposed via REST
- Entity IDs follow the pattern `<domain>.<object_id>` (e.g., `light.kitchen`, `sensor.temperature`)
- Use `/api/template` to test Jinja2 templates before putting them in automations
- Service calls return the resulting states of affected entities
