#!/usr/bin/env python3
import os
import re
from pathlib import Path

# Find all page.tsx files
pages_dir = Path('src/app/companies/[id]')
count = 0

for page_file in pages_dir.rglob('page.tsx'):
    content = page_file.read_text()
    original = content

    # Replace the _unused pattern
    content = content.replace(
        "const { t: _unused } = useI18n(); _unused;",
        "const { t } = useI18n();"
    )

    # Also handle case where it's on separate lines
    content = re.sub(
        r"const { t: _unused } = useI18n\(\);\s*_unused;",
        "const { t } = useI18n();",
        content
    )

    if content != original:
        page_file.write_text(content)
        print(f"FIXED: {page_file}")
        count += 1
    else:
        # Check if t is actually being used
        if "useI18n" in content and "t(" not in content:
            print(f"NEEDS TRANSLATION: {page_file}")

print(f"\nTotal fixed: {count}")
