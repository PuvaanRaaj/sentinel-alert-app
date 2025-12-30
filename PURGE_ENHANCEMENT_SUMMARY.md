# Purge Enhancement Summary

## Changes Made

I've enhanced the purge functionality to support purging alerts by specific chat. Here's what was updated:

---

## Backend Changes

### 1. **Store Interface** (`internal/store/store.go`)
- Added `PurgeAlertsByChat(ctx context.Context, chatID string) error` method to `AlertStore` interface

### 2. **Store Implementation** (`internal/store/store.go`)
- Implemented `PurgeAlertsByChat()` method that:
  - Fetches all alerts from Redis timeline
  - Filters alerts by matching `chat:{chatID}` in the source field
  - Deletes matching alerts and cleans up indexes (source, level, timeline)
  - Uses Redis pipelining for efficient batch deletion

### 3. **Handler** (`internal/handlers/purge.go`)
- Enhanced `PurgeAlertsHandler` to:
  - Accept optional `chat_id` parameter in request body
  - Purge all alerts when no `chat_id` provided
  - Purge only specific chat when `chat_id` is provided
  - Return scope info in response (`"all"` or `"chat-specific"`)
  - Log both actions separately in audit trail

---

## Frontend Changes

### 4. **Admin Dashboard UI** (`web/templates/admin/dashboard.html`)

#### UI Updates:
- Added **Purge Scope** dropdown with options:
  - "All Alerts (Entire System)"
  - "Specific Chat Only"
- Added **Select Chat** dropdown (shown when "Specific Chat" is selected)
- Dynamic button text changes based on selection
- Better confirmation messages showing which chat will be purged

#### JavaScript Updates:
- Enhanced `purgeAlerts()` function to:
  - Read scope selection
  - Send appropriate request body with `chat_id` if chat-specific
  - Show different confirmation messages
  - Display success message with chat name
- Added `updatePurgeUI()` function to:
  - Toggle chat selection visibility
  - Update button text dynamically
- Added `populatePurgeChatDropdown()` to populate chat options
- Added event listener for scope dropdown changes

---

## How to Use

### In Admin Dashboard:

1. Navigate to **System** tab
2. Scroll to **Alert Management**
3. Select purge scope:
   - **All Alerts (Entire System)**: Deletes everything
   - **Specific Chat Only**: Select a chat from dropdown
4. Click **Purge** button
5. Confirm the action

### Via API:

**Purge All:**
```bash
POST /api/admin/purge
Content-Type: application/json

{}
```

**Purge Specific Chat:**
```bash
POST /api/admin/purge
Content-Type: application/json

{
  "chat_id": "chat_2_1764240124958103481"
}
```

**Response:**
```json
{
  "success": true,
  "scope": "chat-specific"  // or "all"
}
```

---

## Audit Trail

Both operations are logged:
- **Purge all**: `action = "purge_alerts"`
- **Purge by chat**: `action = "purge_alerts_by_chat"` with metadata containing the `chat_id`

---

## Technical Details

### Alert Source Format
Alerts are stored with source format: `bot:{botname}:chat:{chatID}`

The chat-specific purge filters by checking if the source contains `chat:{chatID}`.

### Redis Cleanup
The implementation properly cleans up:
- ✅ Alert keys (`alert:*`)
- ✅ Timeline sorted set (`alerts:timeline`)
- ✅ Level indexes (`alerts:level:*`)
- ✅ Source indexes (`alerts:source:*`)

---

## Files Modified

1. `/internal/store/store.go` - Added interface method and implementation
2. `/internal/handlers/purge.go` - Enhanced handler
3. `/web/templates/admin/dashboard.html` - Updated UI and JavaScript

All changes compile successfully and are ready to deploy! 🚀
