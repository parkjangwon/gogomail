import fs from 'fs';

// Safe string replacement - no regex, just direct replacements
function translateFile(filePath, replacements) {
  if (!fs.existsSync(filePath)) {
    console.log(`SKIP: ${filePath}`);
    return false;
  }

  let content = fs.readFileSync(filePath, 'utf8');
  const original = content;

  // Apply all replacements
  replacements.forEach(([from, to]) => {
    // Simple direct replacement - no regex
    while (content.includes(from)) {
      content = content.replace(from, to);
    }
  });

  if (content !== original) {
    fs.writeFileSync(filePath, content, 'utf8');
    console.log(`TRANSLATED: ${filePath}`);
    return true;
  }
  return false;
}

// Translations for each file - only the most common patterns
const translations = {
  'src/app/companies/[id]/access/delegations/page.tsx': [
    ['Delegations', `{t('pages.delegations.title')}`],
    ['Manage user delegations and permissions', `{t('pages.delegations.description')}`],
    ['+ Create Delegation', `{t('pages.delegations.create_delegation')}`],
    ['Delegator', `{t('pages.delegations.delegator')}`],
    ['Delegate', `{t('pages.delegations.delegate')}`],
    ['Status', `{t('pages.delegations.status')}`],
    ['Created', `{t('pages.delegations.created')}`],
  ],
  'src/app/companies/[id]/access/groups/page.tsx': [
    ['Groups', `{t('pages.groups.title')}`],
    ['Manage user groups', `{t('pages.groups.description')}`],
    ['+ Create Group', `{t('pages.groups.create_group')}`],
    ['Group Name', `{t('pages.groups.group_name')}`],
    ['Members', `{t('pages.groups.members')}`],
    ['Created', `{t('pages.groups.created')}`],
  ],
};

let count = 0;
Object.entries(translations).forEach(([filePath, replacements]) => {
  if (translateFile(filePath, replacements)) {
    count++;
  }
});

console.log(`\nTotal translated: ${count}`);
