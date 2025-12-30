# Purge Alerts API Examples

## Purge ALL Alerts

Deletes all alerts from the system.

```bash
curl -X POST "https://sentinel-alert-app.onrender.com/api/admin/purge" \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=YOUR_SESSION_TOKEN"
```

Response:
```json
{
  "success": true,
  "scope": "all"
}
```

---

## Purge Alerts by Specific Chat

Deletes only alerts for a specific chat ID.

```bash
# Purge alerts for chat_2_1764240124958103481
curl -X POST "https://sentinel-alert-app.onrender.com/api/admin/purge" \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=YOUR_SESSION_TOKEN" \
  -d '{
    "chat_id": "chat_2_1764240124958103481"
  }'
```

Response:
```json
{
  "success": true,
  "scope": "chat-specific"
}
```

---

## Usage Examples

### Purge "General" Chat Alerts
```bash
curl -X POST "https://sentinel-alert-app.onrender.com/api/admin/purge" \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=YOUR_SESSION_TOKEN" \
  -d '{
    "chat_id": "general"
  }'
```

### Purge Another Specific Chat
```bash
curl -X POST "https://sentinel-alert-app.onrender.com/api/admin/purge" \
  -H "Content-Type: application/json" \
  -H "Cookie: session_token=YOUR_SESSION_TOKEN" \
  -d '{
    "chat_id": "chat_production_alerts"
  }'
```

---

## How It Works

The purge function filters alerts based on the `source` field, which follows the format:
```
bot:{botname}:chat:{chatID}
```

When you provide a `chat_id`, it will delete all alerts where the source contains `chat:{your_chat_id}`.

### Audit Logs

Both operations are logged in the audit trail:
- Purge all: `action = "purge_alerts"`
- Purge by chat: `action = "purge_alerts_by_chat"` with metadata containing the chat_id
