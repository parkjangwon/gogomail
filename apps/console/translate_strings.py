#!/usr/bin/env python3
import re
from pathlib import Path

# Mapping of common English strings to i18n keys
STRING_MAPPINGS = {
    # Generic patterns that appear in many pages
    ('title', "'>([^<{]+)</Header>", 1, ".title')}}"): (
        r">([\w\s]+)</Header>$", r">{{t('{}.title')}}}</Header>"
    ),
}

# Specific mappings for each page
SPECIFIC_MAPPINGS = {
    'delegations': [
        ('+ Create Delegation', 't(\'pages.delegations.create_delegation\')'),
        ('Delegations', 't(\'pages.delegations.title\')'),
        ('Manage delegation permissions', 't(\'pages.delegations.description\')'),
        ('Delegator', 't(\'pages.delegations.delegator\')'),
        ('Delegate', 't(\'pages.delegations.delegate\')'),
        ('Created', 't(\'pages.delegations.created\')'),
        ('Status', 't(\'pages.delegations.status\')'),
    ],
    'groups': [
        ('+ Create Group', 't(\'pages.groups.create_group\')'),
        ('Groups', 't(\'pages.groups.title\')'),
        ('Manage user groups', 't(\'pages.groups.description\')'),
        ('Group Name', 't(\'pages.groups.group_name\')'),
        ('Members', 't(\'pages.groups.members\')'),
        ('Created', 't(\'pages.groups.created\')'),
    ],
    'api-keys': [
        ('+ Create Key', 't(\'pages.api_keys.create_key\')'),
        ('API Keys', 't(\'pages.api_keys.title\')'),
        ('Manage API keys', 't(\'pages.api_keys.description\')'),
        ('Key', 't(\'pages.api_keys.key\')'),
        ('Secret', 't(\'pages.api_keys.secret\')'),
        ('Created', 't(\'pages.api_keys.created\')'),
    ],
}

def update_file_with_translations(filepath, page_name):
    """Update a file with specific string translations."""
    if not filepath.exists():
        return False

    content = filepath.read_text()
    original = content

    # Get mappings for this page if they exist
    if page_name in SPECIFIC_MAPPINGS:
        for english_str, trans_key in SPECIFIC_MAPPINGS[page_name]:
            # Replace in various JSX contexts
            # 1. In button/header text: >text</button> or >text</Header>
            content = content.replace(f'>{english_str}</', f'>{{{{{trans_key}}}}}</')
            # 2. In attributes: "text"
            content = content.replace(f'"{english_str}"', f'{{{{{trans_key}}}}}')
            # 3. In strings with spaces
            content = content.replace(english_str, trans_key)

    return content != original and filepath.write_text(content) is None

# List of pages to update
PAGES = [
    ('delegations', 'src/app/companies/[id]/access/delegations/page.tsx'),
    ('groups', 'src/app/companies/[id]/access/groups/page.tsx'),
    ('api-keys', 'src/app/companies/[id]/security/api-keys/page.tsx'),
]

count = 0
for page_name, page_path in PAGES:
    p = Path(page_path)
    if update_file_with_translations(p, page_name):
        print(f"TRANSLATED: {page_path}")
        count += 1
    else:
        print(f"NO CHANGES: {page_path}")

print(f"\nTotal translated: {count}")
