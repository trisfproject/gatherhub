#!/usr/bin/env python3
"""
Injects the "Kesehatan Sistem" sidebar link into all admin templates
that have a Pusat Broadcast nav-item but no Kesehatan Sistem link yet.
"""
import os
import re

TEMPLATES_DIR = "/home/langit/Dev/event/gatherhub/backend/internal/templates"

ANCHOR_SNIPPET = 'Pusat Broadcast'

# The nav-item HTML to insert AFTER the Pusat Broadcast closing </a>
INJECT_AFTER = '''    </a>
    <a href="/admin/system" class="nav-item">
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3H5a2 2 0 00-2 2v4m6-6h10a2 2 0 012 2v4M9 3v18m0 0h10a2 2 0 002-2V9M9 21H5a2 2 0 01-2-2V9m0 0h18"/>
      </svg>
      Kesehatan Sistem'''

# Pattern: closing </a> that follows "Pusat Broadcast"
# We find the Pusat Broadcast block then the immediately following </a>
def inject(content):
    if 'Kesehatan Sistem' in content:
        return content, False  # already injected

    if ANCHOR_SNIPPET not in content:
        return content, False  # no broadcast link, skip

    # Find position of ANCHOR_SNIPPET text then locate the next closing </a>
    idx = content.find(ANCHOR_SNIPPET)
    close_tag_pos = content.find('</a>', idx)
    if close_tag_pos == -1:
        return content, False

    # Replace JUST that closing </a> with our inject block
    before = content[:close_tag_pos]
    after  = content[close_tag_pos + len('</a>'):]
    new_content = before + INJECT_AFTER + '\n    </a>' + after
    return new_content, True

skip = {'admin_login.html'}  # login page has no sidebar

updated = []
for fname in sorted(os.listdir(TEMPLATES_DIR)):
    if not fname.endswith('.html'):
        continue
    if fname in skip:
        continue
    path = os.path.join(TEMPLATES_DIR, fname)
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    new_content, changed = inject(content)
    if changed:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(new_content)
        updated.append(fname)
        print(f"  [OK] {fname}")
    else:
        print(f"  [--] {fname} (skipped)")

print(f"\nTotal updated: {len(updated)}")
