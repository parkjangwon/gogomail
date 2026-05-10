import fs from 'fs';

// Load all language files
const languages = ['ko', 'en', 'ja', 'zh-CN'];
const messageFiles = {};

languages.forEach(lang => {
  const filePath = `src/messages/${lang}.json`;
  messageFiles[lang] = JSON.parse(fs.readFileSync(filePath, 'utf8'));
});

// Ensure pages.roles section exists and has all required keys
function ensureTranslationKeys() {
  const requiredKeys = {
    ko: {
      'pages.roles.title': '역할',
      'pages.roles.description': '사용자 역할 및 권한 관리',
      'pages.roles.create_role': '새 역할 생성',
      'pages.roles.role_name': '역할명',
      'pages.roles.description_header': '설명',
      'pages.roles.permissions': '권한',
      'pages.roles.users': '사용자',
      'pages.roles.created': '생성 시간'
    },
    en: {
      'pages.roles.title': 'Roles',
      'pages.roles.description': 'Manage user roles and permissions',
      'pages.roles.create_role': 'Create Role',
      'pages.roles.role_name': 'Name',
      'pages.roles.description_header': 'Description',
      'pages.roles.permissions': 'Permissions',
      'pages.roles.users': 'Assigned Users',
      'pages.roles.created': 'Created'
    },
    ja: {
      'pages.roles.title': 'ロール',
      'pages.roles.description': 'ユーザーロールと権限を管理',
      'pages.roles.create_role': 'ロール作成',
      'pages.roles.role_name': '名前',
      'pages.roles.description_header': '説明',
      'pages.roles.permissions': '権限',
      'pages.roles.users': '割り当てられたユーザー',
      'pages.roles.created': '作成日時'
    },
    'zh-CN': {
      'pages.roles.title': '角色',
      'pages.roles.description': '管理用户角色和权限',
      'pages.roles.create_role': '创建新角色',
      'pages.roles.role_name': '名称',
      'pages.roles.description_header': '描述',
      'pages.roles.permissions': '权限',
      'pages.roles.users': '分配的用户',
      'pages.roles.created': '创建时间'
    }
  };

  languages.forEach(lang => {
    if (!messageFiles[lang].pages.roles) {
      messageFiles[lang].pages.roles = {};
    }
    Object.assign(messageFiles[lang].pages.roles, requiredKeys[lang]);
  });
}

ensureTranslationKeys();

// Save updated language files
languages.forEach(lang => {
  const filePath = `src/messages/${lang}.json`;
  fs.writeFileSync(filePath, JSON.stringify(messageFiles[lang], null, 2));
  console.log(`Updated ${filePath}`);
});

console.log('Translation keys added to all language files');
