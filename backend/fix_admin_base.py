#!/usr/bin/env python3
"""
Fixes all struct literal sites in admin.go after the AdminBase refactor.

Each struct literal currently has:
    AdminUser: adminUser,
    AdminRole: adminRole,
    Stats:     stats,

which need to become:
    AdminBase: AdminBase{
        AdminUser:  adminUser,
        AdminRole:  adminRole,
        ActiveMenu: "<page>",
        Stats:      stats,
    },

This script:
1. Finds every handler function (pattern: func (h *AdminHandler) FuncName)
2. Determines the correct ActiveMenu value for that function
3. In the struct literal block for that function, replaces the old fields
   with the new AdminBase embedded literal.

The mapping of handler → ActiveMenu is defined below.
"""
import re
import sys

FILE = "/home/langit/Dev/event/gatherhub/backend/internal/handlers/admin.go"

# Maps handler function name → ActiveMenu value
ACTIVE_MENU_MAP = {
    "Dashboard":             "dashboard",
    "ParticipantList":       "participants",
    "ParticipantDetail":     "participants",
    "ParticipantQRPage":     "participants",
    "UpdateStatus":          "participants",
    "ExportParticipants":    "participants",
    "NotificationList":      "notifications",
    "AuditLogList":          "audit-logs",
    "EventList":             "events",
    "EventDetail":           "events",
    "EventCreatePage":       "events",
    "EventCreateSubmit":     "events",
    "EventEditPage":         "events",
    "EventEditSubmit":       "events",
    "AdminList":             "admins",
    "AdminCreatePage":       "admins",
    "AdminCreateSubmit":     "admins",
    "AdminEditPage":         "admins",
    "AdminEditSubmit":       "admins",
    "SystemSettingsPage":    "settings",
    "SystemSettingsSubmit":  "settings",
    "BackupsPage":           "backups",
    "CheckinPage":           "checkin",
    "CheckinSubmit":         "checkin",
    "AttendanceDashboard":   "attendance",
    "BroadcastList":         "broadcasts",
    "BroadcastCreatePage":   "broadcasts",
    "BroadcastCreateSubmit": "broadcasts",
    "BroadcastPreview":      "broadcasts",
    "BroadcastDetail":       "broadcasts",
    "SystemHealth":          "system",
}

with open(FILE, "r", encoding="utf-8") as f:
    content = f.read()

# Pattern that matches the old "flat" fields in a struct literal
# We look for blocks like:
#   AdminUser: adminUser,
#   AdminRole: adminRole,
#   Stats:     stats,
# possibly with other fields in between. We replace with the AdminBase block.
#
# Strategy: for each handler function, find its body, then replace all
# occurrences of the flat field pattern inside that body.

# Regex to find handler function boundaries
HANDLER_RE = re.compile(
    r'func \(h \*AdminHandler\) (\w+)\(c \*fiber\.Ctx\) error \{',
)

# Pattern to match the flat AdminUser/AdminRole/Stats lines (in any order,
# allowing other fields in between is too complex; they always appear together)
FLAT_FIELDS_RE = re.compile(
    r'(\t+)AdminUser:\s*adminUser,\n'
    r'\1AdminRole:\s*adminRole,\n'
    r'(\1Stats:\s*stats,\n)?',
)

FLAT_FIELDS_RE2 = re.compile(
    r'(\t+)AdminRole:\s*adminRole,\n'
    r'\1AdminUser:\s*adminUser,\n'
    r'(\1Stats:\s*stats,\n)?',
)

# Find all handler positions
handlers = list(HANDLER_RE.finditer(content))

changes = 0

# Process from last to first to preserve positions
for i in range(len(handlers) - 1, -1, -1):
    m = handlers[i]
    func_name = m.group(1)
    active_menu = ACTIVE_MENU_MAP.get(func_name, "")
    if not active_menu:
        continue

    # Body is from end of match to start of next handler (or EOF)
    body_start = m.end()
    body_end = handlers[i + 1].start() if i + 1 < len(handlers) else len(content)

    body = content[body_start:body_end]

    def make_replacement(m_inner, menu=active_menu):
        indent = m_inner.group(1)
        has_stats = bool(m_inner.group(2))
        stats_line = f"{indent}\tStats:      stats,\n" if has_stats else ""
        return (
            f"{indent}AdminBase: AdminBase{{\n"
            f"{indent}\tAdminUser:  adminUser,\n"
            f"{indent}\tAdminRole:  adminRole,\n"
            f"{indent}\tActiveMenu: \"{menu}\",\n"
            f"{stats_line}"
            f"{indent}}},\n"
        )

    new_body, n = FLAT_FIELDS_RE.subn(make_replacement, body)
    if n == 0:
        new_body, n = FLAT_FIELDS_RE2.subn(make_replacement, new_body)

    if n > 0:
        content = content[:body_start] + new_body + content[body_end:]
        changes += n
        print(f"  [OK] {func_name} ({n} replacement(s), menu={active_menu})")
    else:
        print(f"  [--] {func_name} — pattern not found in body")

with open(FILE, "w", encoding="utf-8") as f:
    f.write(content)

print(f"\nTotal replacements: {changes}")
