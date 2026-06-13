#!/usr/bin/env python3
"""
Moves outer-level 'Stats: stats,' into the AdminBase{} block.

Pattern to find:
    AdminBase: AdminBase{
        ...
        (no Stats here)
    },
    ...
    Stats:          stats,   ← wrong level

And move the Stats line inside the AdminBase block.
"""
import re

FILE = "/home/langit/Dev/event/gatherhub/backend/internal/handlers/admin.go"

with open(FILE, "r", encoding="utf-8") as f:
    content = f.read()

# Pattern: AdminBase block followed (anywhere within 20 lines) by outer Stats:
# We find AdminBase: AdminBase{ ... }, then Stats: stats, just outside

ADMIN_BASE_BLOCK_RE = re.compile(
    r'([ \t]+AdminBase: AdminBase\{[^}]*?\}),\n'  # AdminBase block ending
    r'((?:[ \t]+\w[^S\n][^\n]*\n)*?)'              # optional other fields (non-Stats)
    r'([ \t]+Stats:[ \t]+\w+,\n)',                  # the orphan Stats line
    re.DOTALL
)

def fix_match(m):
    admin_base_block = m.group(1)  # everything up to closing }
    between = m.group(2)
    stats_line = m.group(3).strip()  # "Stats: stats,"

    # Insert Stats before the closing brace in the AdminBase block
    # Find the closing } of AdminBase
    idx = admin_base_block.rfind('}')
    indent = re.match(r'([ \t]+)', m.group(3)).group(1)
    new_block = admin_base_block[:idx] + f"\t{stats_line}\n{indent}" + admin_base_block[idx:]
    return new_block + ',\n' + between

new_content, count = ADMIN_BASE_BLOCK_RE.subn(fix_match, content)

if count == 0:
    # Try simpler: just find "Stats:          stats," NOT inside an AdminBase block
    # and remove them (since Stats is now promoted via embedding)
    print("Pattern not found via complex regex — trying simple removal of orphan Stats lines")

    # Find all lines that are just \t+Stats:\t+stats,
    # but NOT inside an AdminBase{ block
    # The safest fix: move every orphan Stats into the preceding AdminBase block

    # Re-approach: find AdminBase blocks and the NEXT Stats: line after the closing }
    lines = content.splitlines(keepends=True)
    result = []
    i = 0
    changes = 0
    while i < len(lines):
        line = lines[i]
        # Detect start of AdminBase: AdminBase{
        if 'AdminBase: AdminBase{' in line:
            # Collect the AdminBase block
            block = [line]
            i += 1
            depth = line.count('{') - line.count('}')
            while i < len(lines) and depth > 0:
                block.append(lines[i])
                depth += lines[i].count('{') - lines[i].count('}')
                i += 1
            # Now check if the next non-empty line is Stats:
            j = i
            while j < len(lines) and lines[j].strip() == '':
                j += 1
            if j < len(lines) and re.match(r'[ \t]+Stats:\s+\w+,', lines[j]):
                stats_line = lines[j]
                # Insert Stats before the closing } of block
                # Find last line of block (the one with })
                closing = block[-1]
                indent_stats = re.match(r'([ \t]*)', stats_line).group(1)
                stats_stripped = stats_line.strip()
                block[-1] = closing.rstrip('\n') + '\n'
                # Insert stats before closing }
                block.insert(-1, f"\t{stats_stripped}\n")
                result.extend(block)
                # Skip the Stats line
                i = j + 1
                changes += 1
                print(f"  Moved Stats into AdminBase at ~line {i}")
                continue
            else:
                result.extend(block)
                continue
        result.append(line)
        i += 1

    new_content = ''.join(result)
    count = changes

with open(FILE, "w", encoding="utf-8") as f:
    f.write(new_content)

print(f"Total fixes: {count}")
