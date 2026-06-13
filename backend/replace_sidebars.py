#!/usr/bin/env python3
"""
Replace sidebar HTML blocks in all admin templates with:
  {{template "admin_sidebar" .}}

Strategy: find the <aside ...> block that contains the nav-item links
and replace the entire <aside>...</aside> with the template call.

Also removes active class hard-coding (the partial handles it via ActiveMenu).
"""
import os
import re

TEMPLATES_DIR = "/home/langit/Dev/event/gatherhub/backend/internal/templates"

# Regex: match <aside (anything) containing "sidebar" up to its closing </aside>
# We use a greedy match within the known sidebar block.
ASIDE_RE = re.compile(
    r'<aside\b[^>]*sidebar[^>]*>.*?</aside>',
    re.DOTALL
)

REPLACEMENT = '{{template "admin_sidebar" .}}'

skip = {
    'admin_login.html',   # no sidebar
    'admin_sidebar.html', # the partial itself
}

updated = []
not_found = []

for fname in sorted(os.listdir(TEMPLATES_DIR)):
    if not fname.startswith('admin_') or not fname.endswith('.html'):
        continue
    if fname in skip:
        print(f"  [--] {fname} (skipped)")
        continue

    path = os.path.join(TEMPLATES_DIR, fname)
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()

    m = ASIDE_RE.search(content)
    if not m:
        print(f"  [??] {fname} — no <aside> sidebar found")
        not_found.append(fname)
        continue

    new_content = content[:m.start()] + REPLACEMENT + content[m.end():]

    with open(path, 'w', encoding='utf-8') as f:
        f.write(new_content)
    updated.append(fname)
    print(f"  [OK] {fname}")

print(f"\nUpdated: {len(updated)}, Not found: {len(not_found)}")
if not_found:
    print("Manual check needed:", not_found)
