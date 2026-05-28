-- Dev seed data for gogomail (English locale)
-- Admin:  admin@gogomail.io / admin1234
-- Demo:   user@acme.io      / pass1234
-- Run:  docker exec -i gogomail-postgres-dev psql -U gogomail -d gogomail < scripts/seed_dev_data_en.sql

BEGIN;

-- ══════════════════════════════════════════════════════════════════════════════
-- 0. ADMIN TENANT  admin@gogomail.io / admin1234
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO companies (id, name, status)
VALUES ('10000000-0000-0000-0000-000000000001', 'GoGoMail', 'active')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status;

INSERT INTO domains (id, company_id, name, name_ace, status)
VALUES ('10000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001',
        'gogomail.io', 'gogomail.io', 'active')
ON CONFLICT (id) DO UPDATE SET company_id = EXCLUDED.company_id, name = EXCLUDED.name,
  name_ace = EXCLUDED.name_ace, status = EXCLUDED.status;

INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES ('10000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002',
        'admin', 'System Admin', 'plain:admin1234', 'local', 'system_admin', 'active')
ON CONFLICT (domain_id, username) DO UPDATE SET display_name = EXCLUDED.display_name,
  password_hash = EXCLUDED.password_hash, auth_source = EXCLUDED.auth_source,
  role = EXCLUDED.role, status = EXCLUDED.status;

INSERT INTO user_addresses (id, user_id, domain_id, local_part, local_part_ace, domain_ace,
  address, address_ace, is_primary)
VALUES ('10000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000003',
  '10000000-0000-0000-0000-000000000002',
  'admin', 'admin', 'gogomail.io', 'admin@gogomail.io', 'admin@gogomail.io', true)
ON CONFLICT (address) DO UPDATE SET user_id = EXCLUDED.user_id,
  domain_id = EXCLUDED.domain_id, is_primary = true;

INSERT INTO folders (id, user_id, name, full_path, type, system_type, order_index)
SELECT gen_random_uuid(), '10000000-0000-0000-0000-000000000003'::uuid,
  f.name, f.full_path, 'system', f.stype, f.ord
FROM (VALUES
  ('Inbox',  '/Inbox',  'inbox',  0),
  ('Drafts', '/Drafts', 'drafts', 1),
  ('Sent',   '/Sent',   'sent',   2),
  ('Trash',  '/Trash',  'trash',  3),
  ('Spam',   '/Spam',   'spam',   4)
) AS f(name, full_path, stype, ord)
WHERE NOT EXISTS (
  SELECT 1 FROM folders fo
  WHERE fo.user_id = '10000000-0000-0000-0000-000000000003'::uuid
    AND fo.system_type = f.stype
);

-- ══════════════════════════════════════════════════════════════════════════════
-- 1. DEMO TENANT  acme.io / Acme Corp
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO companies (id, name, status)
VALUES ('7206af4e-fc44-4a65-890d-55bb35741d6c', 'Acme Corp', 'active')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status;

INSERT INTO domains (id, company_id, name, name_ace, status)
VALUES ('7049fa6e-d649-44d3-83d2-b548c7e787d5', '7206af4e-fc44-4a65-890d-55bb35741d6c',
        'acme.io', 'acme.io', 'active')
ON CONFLICT (id) DO UPDATE SET company_id = EXCLUDED.company_id, name = EXCLUDED.name,
  name_ace = EXCLUDED.name_ace, status = EXCLUDED.status;

INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES ('30000000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
        'user', 'Jamie Park', 'plain:pass1234', 'local', 'user', 'active')
ON CONFLICT (domain_id, username) DO UPDATE SET display_name = EXCLUDED.display_name,
  password_hash = EXCLUDED.password_hash, auth_source = EXCLUDED.auth_source,
  role = EXCLUDED.role, status = EXCLUDED.status;

INSERT INTO user_addresses (id, user_id, domain_id, local_part, local_part_ace, domain_ace,
  address, address_ace, is_primary)
VALUES ('30000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001',
  '7049fa6e-d649-44d3-83d2-b548c7e787d5',
  'user', 'user', 'acme.io', 'user@acme.io', 'user@acme.io', true)
ON CONFLICT (address) DO UPDATE SET user_id = EXCLUDED.user_id,
  domain_id = EXCLUDED.domain_id, is_primary = true;

-- Co-workers
INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES
  ('a2000000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'kim.chulsoo',  'Chris Kim',  'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000002', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.younghee', 'Yuna Lee',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000003', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'park.minjun',  'Min Park',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000004', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'jung.sooyeon', 'Sue Jung',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000005', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'choi.junho',   'Jun Choi',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000006', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'han.jiyeon',   'Jiyeon Han', 'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000007', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'kang.hyunjae', 'Henry Kang', 'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000008', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'oh.seokmin',   'Seok Oh',    'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000009', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'song.jiyul',   'Jay Song',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000010', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'jang.inkyung', 'Ina Jang',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000011', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'baek.woojin',  'Woo Baek',   'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000012', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'shim.dayoung', 'Dora Shim',  'plain:pass1234', 'local', 'user', 'active'),
  ('a2000000-0000-0000-0000-000000000013', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','Sean Hong',  'plain:pass1234', 'local', 'user', 'active')
ON CONFLICT (domain_id, username) DO NOTHING;

INSERT INTO user_addresses (id, user_id, domain_id, local_part, local_part_ace, domain_ace, address, address_ace, is_primary)
VALUES
  ('b2000000-0000-0000-0000-000000000001', 'a2000000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'kim.chulsoo',  'kim.chulsoo',  'acme.io', 'kim.chulsoo@acme.io',  'kim.chulsoo@acme.io',  true),
  ('b2000000-0000-0000-0000-000000000002', 'a2000000-0000-0000-0000-000000000002', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.younghee', 'lee.younghee', 'acme.io', 'lee.younghee@acme.io', 'lee.younghee@acme.io', true),
  ('b2000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000003', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'park.minjun',  'park.minjun',  'acme.io', 'park.minjun@acme.io',  'park.minjun@acme.io',  true),
  ('b2000000-0000-0000-0000-000000000004', 'a2000000-0000-0000-0000-000000000004', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'jung.sooyeon', 'jung.sooyeon', 'acme.io', 'jung.sooyeon@acme.io', 'jung.sooyeon@acme.io', true),
  ('b2000000-0000-0000-0000-000000000005', 'a2000000-0000-0000-0000-000000000005', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'choi.junho',   'choi.junho',   'acme.io', 'choi.junho@acme.io',   'choi.junho@acme.io',   true),
  ('b2000000-0000-0000-0000-000000000006', 'a2000000-0000-0000-0000-000000000006', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'han.jiyeon',   'han.jiyeon',   'acme.io', 'han.jiyeon@acme.io',   'han.jiyeon@acme.io',   true),
  ('b2000000-0000-0000-0000-000000000007', 'a2000000-0000-0000-0000-000000000007', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'kang.hyunjae', 'kang.hyunjae', 'acme.io', 'kang.hyunjae@acme.io', 'kang.hyunjae@acme.io', true),
  ('b2000000-0000-0000-0000-000000000008', 'a2000000-0000-0000-0000-000000000008', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'oh.seokmin',   'oh.seokmin',   'acme.io', 'oh.seokmin@acme.io',   'oh.seokmin@acme.io',   true),
  ('b2000000-0000-0000-0000-000000000009', 'a2000000-0000-0000-0000-000000000009', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'song.jiyul',   'song.jiyul',   'acme.io', 'song.jiyul@acme.io',   'song.jiyul@acme.io',   true),
  ('b2000000-0000-0000-0000-000000000010', 'a2000000-0000-0000-0000-000000000010', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'jang.inkyung', 'jang.inkyung', 'acme.io', 'jang.inkyung@acme.io', 'jang.inkyung@acme.io', true),
  ('b2000000-0000-0000-0000-000000000011', 'a2000000-0000-0000-0000-000000000011', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'baek.woojin',  'baek.woojin',  'acme.io', 'baek.woojin@acme.io',  'baek.woojin@acme.io',  true),
  ('b2000000-0000-0000-0000-000000000012', 'a2000000-0000-0000-0000-000000000012', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'shim.dayoung', 'shim.dayoung', 'acme.io', 'shim.dayoung@acme.io', 'shim.dayoung@acme.io', true),
  ('b2000000-0000-0000-0000-000000000013', 'a2000000-0000-0000-0000-000000000013', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','hong.seungwoo','acme.io', 'hong.seungwoo@acme.io','hong.seungwoo@acme.io', true)
ON CONFLICT DO NOTHING;

-- System folders for co-workers
INSERT INTO folders (id, user_id, name, full_path, type, system_type, order_index)
SELECT gen_random_uuid(), u.id::uuid, f.name, f.name, 'system', f.stype, f.ord
FROM (VALUES
  ('a2000000-0000-0000-0000-000000000001'), ('a2000000-0000-0000-0000-000000000002'),
  ('a2000000-0000-0000-0000-000000000003'), ('a2000000-0000-0000-0000-000000000004'),
  ('a2000000-0000-0000-0000-000000000005'), ('a2000000-0000-0000-0000-000000000006'),
  ('a2000000-0000-0000-0000-000000000007'), ('a2000000-0000-0000-0000-000000000008'),
  ('a2000000-0000-0000-0000-000000000009'), ('a2000000-0000-0000-0000-000000000010'),
  ('a2000000-0000-0000-0000-000000000011'), ('a2000000-0000-0000-0000-000000000012'),
  ('a2000000-0000-0000-0000-000000000013')
) AS u(id)
CROSS JOIN (VALUES
  ('Inbox', 'inbox', 1), ('Sent', 'sent', 2), ('Drafts', 'drafts', 3), ('Trash', 'trash', 4)
) AS f(name, stype, ord)
WHERE NOT EXISTS (
  SELECT 1 FROM folders fo WHERE fo.user_id = u.id::uuid AND fo.system_type = f.stype
);


-- ══════════════════════════════════════════════════════════════════════════════
-- 2. DEMO USER FOLDERS
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO folders (id, user_id, name, full_path, type, system_type, order_index)
VALUES
  ('f3000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Inbox',  '/Inbox',  'system', 'inbox',  0),
  ('f3000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'Drafts', '/Drafts', 'system', 'drafts', 1),
  ('f3000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000001', 'Sent',   '/Sent',   'system', 'sent',   2),
  ('f3000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000001', 'Trash',  '/Trash',  'system', 'trash',  3),
  ('f3000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000001', 'Spam',   '/Spam',   'system', 'spam',   4)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, full_path = EXCLUDED.full_path,
  type = EXCLUDED.type, system_type = EXCLUDED.system_type, order_index = EXCLUDED.order_index;

INSERT INTO folders (id, user_id, name, full_path, type, order_index)
VALUES
  ('f3000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000001', 'Projects',    '/Projects',    'custom', 10),
  ('f3000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000001', 'Newsletters', '/Newsletters', 'custom', 11),
  ('f3000000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000001', 'Invoices',    '/Invoices',    'custom', 12),
  ('f3000000-0000-0000-0000-000000000013', '30000000-0000-0000-0000-000000000001', 'Work',        '/Work',        'custom', 13)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, full_path = EXCLUDED.full_path,
  order_index = EXCLUDED.order_index;


-- ══════════════════════════════════════════════════════════════════════════════
-- 3. DEMO USER INBOX — 15 messages
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO messages (
  id, tenant_id, domain_id, user_id, folder_id,
  rfc_message_id, thread_id, subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  received_at, sent_at, size, has_attachment,
  flags, status, storage_path, draft_text_body
) VALUES
  ('f3100000-0000-0000-0000-000000000001',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg001en@acme.io>', 'f3100000-0000-0000-0000-000000000001',
   '[Dev Team] May Sprint Kickoff Schedule',
   'kim.chulsoo@acme.io', 'Chris Kim',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 1200, false,
   '{"read":false,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Hi team, sharing the sprint kickoff schedule. Wednesday at 10am in Meeting Room A. Please join.'),

  ('f3100000-0000-0000-0000-000000000002',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg002en@acme.io>', 'f3100000-0000-0000-0000-000000000002',
   'Re: PR #312 Code Review Request - Auth Middleware',
   'lee.younghee@acme.io', 'Yuna Lee',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', 980, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Have you reviewed PR #312? Please check the suggested changes in the auth middleware section. Planning to merge today so a quick review would be appreciated.'),

  ('f3100000-0000-0000-0000-000000000003',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg003en@acme.io>', 'f3100000-0000-0000-0000-000000000003',
   'Q2 Marketing Campaign Landing Page Collaboration',
   'park.minjun@acme.io', 'Min Park',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb,
   '[{"address":"jung.sooyeon@acme.io","name":"Sue Jung"},{"address":"han.jiyeon@acme.io","name":"Jiyeon Han"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours', 1540, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Hi Jamie, reaching out about the landing page development for the Q2 campaign. Would you be available for a meeting next week?'),

  ('f3100000-0000-0000-0000-000000000004',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg004en@acme.io>', 'f3100000-0000-0000-0000-000000000004',
   'May Performance Review — Self-Evaluation Submission Guide',
   'choi.junho@acme.io', 'Jun Choi',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', 2100, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'Please submit your self-evaluation form to the HR portal by May 15. The performance review cycle covers Q1 and Q2 objectives.'),

  ('f3100000-0000-0000-0000-000000000005',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg005en@acme.io>', 'f3100000-0000-0000-0000-000000000005',
   '[All Staff] May Town Hall Meeting Notice',
   'jung.sooyeon@acme.io', 'Sue Jung',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', 870, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'The monthly town hall is scheduled for May 22 at 2pm in the Main Conference Room. Includes CEO presentation and Q&A session.'),

  ('f3100000-0000-0000-0000-000000000006',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg006en@acme.io>', 'f3100000-0000-0000-0000-000000000006',
   'Cloud Infrastructure Cost Optimization Proposal',
   'han.jiyeon@acme.io', 'Jiyeon Han',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days' - INTERVAL '4 hours', NOW() - INTERVAL '2 days' - INTERVAL '4 hours', 3600, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Sharing AWS cost analysis results. Switching to Reserved Instances and adjusting S3 storage tiers could reduce monthly costs by approximately 30%.'),

  ('f3100000-0000-0000-0000-000000000007',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg007en@acme.io>', 'f3100000-0000-0000-0000-000000000007',
   '[URGENT] New Service Launch Plan — Final Review Request',
   'kim.chulsoo@acme.io', 'Chris Kim',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb,
   '[{"address":"park.minjun@acme.io","name":"Min Park"},{"address":"choi.junho@acme.io","name":"Jun Choi"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', 4800, true,
   '{"read":false,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Jamie, sharing the final plan for the gogomail service launch next month. Please complete the technical review and sign off.'),

  ('f3100000-0000-0000-0000-000000000008',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg008en@acme.io>', 'f3100000-0000-0000-0000-000000000008',
   'Re: Mail List API Response Latency Analysis',
   'oh.seokmin@acme.io', 'Seok Oh',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 days' - INTERVAL '2 hours', NOW() - INTERVAL '3 days' - INTERVAL '2 hours', 1780, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'After adding the missing composite index on the messages table, p99 latency dropped from 820ms to 45ms. Opened the migration PR for your review.'),

  ('f3100000-0000-0000-0000-000000000009',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg009en@acme.io>', 'f3100000-0000-0000-0000-000000000009',
   '[Security] Monthly Vulnerability Scan Report - May 2026',
   'kang.hyunjae@acme.io', 'Henry Kang',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', 5200, true,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Attaching the May security scan results. A patch for CVE-2026-1234 is required. Please coordinate a deployment schedule this week.'),

  ('f3100000-0000-0000-0000-000000000010',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg010en@acme.io>', 'f3100000-0000-0000-0000-000000000010',
   'Team lunch this Friday?',
   'baek.woojin@acme.io', 'Woo Baek',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb,
   '[{"address":"kang.hyunjae@acme.io","name":"Henry Kang"},{"address":"oh.seokmin@acme.io","name":"Seok Oh"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '4 days' - INTERVAL '3 hours', NOW() - INTERVAL '4 days' - INTERVAL '3 hours', 520, false,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'Heading to a nearby restaurant at 12:30pm Friday. Reply if you can join!'),

  ('f3100000-0000-0000-0000-000000000011',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg011en@acme.io>', 'f3100000-0000-0000-0000-000000000011',
   '[CI/CD] main branch build failed — gogomail-backend',
   'shim.dayoung@acme.io', 'Dora Shim',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 1100, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'The main branch build failed. A timeout occurred in the TestIMAPSearchFastPath test. Please investigate.'),

  ('f3100000-0000-0000-0000-000000000012',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg012en@acme.io>', 'f3100000-0000-0000-0000-000000000012',
   '[Contract] NextWave Studio Collaboration Agreement Review',
   'minji.kwon@example-partner.test', 'Minji Kwon',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days' - INTERVAL '6 hours', NOW() - INTERVAL '5 days' - INTERVAL '6 hours', 8500, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Hi Jamie, attaching the draft collaboration agreement for the launch campaign. Please review with legal and respond.'),

  ('f3100000-0000-0000-0000-000000000013',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg013en@acme.io>', 'f3100000-0000-0000-0000-000000000013',
   'GoGoMail Service Banner Design Draft v2',
   'hong.seungwoo@acme.io', 'Sean Hong',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb,
   '[{"address":"park.minjun@acme.io","name":"Min Park"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days', 2300, true,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Attaching draft v2 incorporating your feedback. Updated the color palette and typography.'),

  ('f3100000-0000-0000-0000-000000000014',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg014en@acme.io>', 'f3100000-0000-0000-0000-000000000014',
   '[Company-Wide] IT Security Policy Update (Required Reading)',
   'choi.junho@acme.io', 'Jun Choi',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days', 3100, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Announcing updates to the company IT security policy. MDM enrollment will be mandatory for personal devices connected to work networks, effective June 1.'),

  ('f3100000-0000-0000-0000-000000000015',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000001',
   '<msg015en@acme.io>', 'f3100000-0000-0000-0000-000000000015',
   'Q2 OKR Mid-Review — Engineering Division Results',
   'kim.chulsoo@acme.io', 'Chris Kim',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb,
   '[{"address":"lee.younghee@acme.io","name":"Yuna Lee"},{"address":"oh.seokmin@acme.io","name":"Seok Oh"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', 2650, false,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Sharing Q2 OKR mid-review results for the Engineering Division. 2 of 3 Key Results achieved, 1 at 70% progress.')

ON CONFLICT (id) DO UPDATE SET
  folder_id = EXCLUDED.folder_id, rfc_message_id = EXCLUDED.rfc_message_id,
  thread_id = EXCLUDED.thread_id, subject = EXCLUDED.subject,
  from_addr = EXCLUDED.from_addr, from_name = EXCLUDED.from_name,
  to_addrs = EXCLUDED.to_addrs, cc_addrs = EXCLUDED.cc_addrs, bcc_addrs = EXCLUDED.bcc_addrs,
  received_at = EXCLUDED.received_at, sent_at = EXCLUDED.sent_at,
  size = EXCLUDED.size, has_attachment = EXCLUDED.has_attachment,
  flags = EXCLUDED.flags, status = EXCLUDED.status,
  storage_path = EXCLUDED.storage_path, draft_text_body = EXCLUDED.draft_text_body,
  updated_at = now();


-- ══════════════════════════════════════════════════════════════════════════════
-- 4. DEMO USER CUSTOM FOLDER MESSAGES
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO messages (
  id, tenant_id, domain_id, user_id, folder_id,
  rfc_message_id, thread_id, subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  received_at, sent_at, size, has_attachment,
  flags, status, storage_path, draft_text_body
) VALUES
  -- Projects (2)
  ('f3200000-0000-0000-0000-000000000001',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000010',
   '<proj001en@acme.io>', 'f3200000-0000-0000-0000-000000000001',
   '[Project] GoGoMail v2.0 Technical Spec Document',
   'kim.chulsoo@acme.io', 'Chris Kim',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days', 12000, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Sharing the GoGoMail v2.0 technical spec draft. Covers IMAP/SMTP layer refactoring and OpenSearch integration design.'),

  ('f3200000-0000-0000-0000-000000000002',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000010',
   '<proj002en@acme.io>', 'f3200000-0000-0000-0000-000000000002',
   '[Project] Migration Plan — Postgres to TimescaleDB Evaluation',
   'oh.seokmin@acme.io', 'Seok Oh',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '12 days', NOW() - INTERVAL '12 days', 5400, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'Sharing TimescaleDB migration evaluation results. Approximately 8x compression improvement expected based on current mail event log volume.'),

  -- Newsletters (3)
  ('f3300000-0000-0000-0000-000000000001',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000011',
   '<nl001en@devnews.example.test>', 'f3300000-0000-0000-0000-000000000001',
   'Go Weekly #598 — Go 1.25 Release Notes and range-over-func',
   'newsletter@devnews.example.test', 'Go Weekly',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', 8900, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Go 1.25 has been released. This issue covers the new range-over-func feature and the math/rand/v2 API.'),

  ('f3300000-0000-0000-0000-000000000002',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000011',
   '<nl002en@techdigest.example.test>', 'f3300000-0000-0000-0000-000000000002',
   'Tech Digest: OpenSearch 3.0 Major Changes Summary',
   'digest@techdigest.example.test', 'Tech Digest',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days', 6200, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Summarizing major changes in OpenSearch 3.0. Notable improvements include a new k-NN engine and enhanced AI/ML capabilities.'),

  ('f3300000-0000-0000-0000-000000000003',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000011',
   '<nl003en@cloudnews.example.test>', 'f3300000-0000-0000-0000-000000000003',
   'Cloud Cost Report: AWS vs GCP 2026 Q1 Comparison',
   'report@cloudnews.example.test', 'Cloud Cost Report',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '9 days', NOW() - INTERVAL '9 days', 7100, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '2026 Q1 real-world cost comparison between AWS and GCP. GCP was approximately 12% cheaper for container workloads on average.'),

  -- Invoices (3)
  ('f3400000-0000-0000-0000-000000000001',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000012',
   '<inv001en@aws.example.test>', 'f3400000-0000-0000-0000-000000000001',
   'AWS Invoice — April 2026 ($1,842)',
   'billing@aws.example.test', 'AWS Billing',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '14 days', NOW() - INTERVAL '14 days', 4200, true,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'April 2026 AWS charges attached. EC2($890), RDS($520), S3($432).'),

  ('f3400000-0000-0000-0000-000000000002',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000012',
   '<inv002en@github.example.test>', 'f3400000-0000-0000-0000-000000000002',
   'GitHub Enterprise Renewal Invoice — Annual Subscription',
   'billing@github.example.test', 'GitHub Billing',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days', 2800, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'Annual GitHub Enterprise subscription renewal for 30 users at $4,200/year. Receipt will be issued upon payment.'),

  ('f3400000-0000-0000-0000-000000000003',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000012',
   '<inv003en@figma.example.test>', 'f3400000-0000-0000-0000-000000000003',
   'Figma Team Plan Payment Confirmed — May 2026',
   'billing@figma.example.test', 'Figma',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', 1900, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Figma Team Plan ($15/editor x 5) May payment confirmed. Receipt available in the billing portal.'),

  -- Work (2)
  ('f3500000-0000-0000-0000-000000000001',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000013',
   '<work001en@acme.io>', 'f3500000-0000-0000-0000-000000000001',
   'PTO Approved — 2026-06-02 to 06-06 (5 days)',
   'choi.junho@acme.io', 'Jun Choi',
   '[{"address":"user@acme.io","name":"Jamie Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 680, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Your PTO request has been approved. Period: 2026-06-02(Mon) through 06-06(Fri), 5 days total. Enjoy your vacation!'),

  ('f3500000-0000-0000-0000-000000000002',
   '7206af4e-fc44-4a65-890d-55bb35741d6c', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   '30000000-0000-0000-0000-000000000001', 'f3000000-0000-0000-0000-000000000013',
   '<work002en@acme.io>', 'f3500000-0000-0000-0000-000000000002',
   '[Work] Remote Work Request — June 2026',
   'user@acme.io', 'Jamie Park',
   '[{"address":"choi.junho@acme.io","name":"Jun Choi"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', 920, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Hi Jun, submitting my remote work request for June. Requesting 2 days per week (Mon, Wed). Please review.')

ON CONFLICT (id) DO UPDATE SET
  folder_id = EXCLUDED.folder_id, rfc_message_id = EXCLUDED.rfc_message_id,
  thread_id = EXCLUDED.thread_id, subject = EXCLUDED.subject,
  from_addr = EXCLUDED.from_addr, from_name = EXCLUDED.from_name,
  to_addrs = EXCLUDED.to_addrs, cc_addrs = EXCLUDED.cc_addrs, bcc_addrs = EXCLUDED.bcc_addrs,
  received_at = EXCLUDED.received_at, sent_at = EXCLUDED.sent_at,
  size = EXCLUDED.size, has_attachment = EXCLUDED.has_attachment,
  flags = EXCLUDED.flags, status = EXCLUDED.status,
  storage_path = EXCLUDED.storage_path, draft_text_body = EXCLUDED.draft_text_body,
  updated_at = now();

UPDATE messages
SET html_body = '<p>' || draft_text_body || '</p>'
WHERE user_id = '30000000-0000-0000-0000-000000000001'
  AND domain_id = '7049fa6e-d649-44d3-83d2-b548c7e787d5'
  AND storage_path = ''
  AND draft_text_body IS NOT NULL
  AND draft_text_body != ''
  AND (html_body IS NULL OR html_body = '');


-- ══════════════════════════════════════════════════════════════════════════════
-- 5. ORGANIZATION STRUCTURE (acme.io / Acme Corp)
--
--   Acme Corp
--   ├── Engineering Division
--   │   ├── Product Business Unit
--   │   │   ├── Backend Team   — lead: Chris Kim, members: Jamie Park, Seok Oh, Ina Jang
--   │   │   └── Frontend Team  — lead: Yuna Lee, member: Jay Song
--   │   └── Technology Business Unit
--   │       └── Infrastructure Team — lead: Henry Kang, member: Woo Baek
--   ├── Marketing Division
--   │   ├── Brand Business Unit
--   │   │   └── Brand Team     — lead: Min Park, members: Dora Shim, Sean Hong
--   │   └── Growth Business Unit
--   │       └── Performance Team — lead: Sue Jung, member: Jiyeon Han
--   └── Business Operations (department, direct teams)
--       ├── HR Team   — lead: Jun Choi
--       └── Finance Team (no members)
-- ══════════════════════════════════════════════════════════════════════════════

-- 5-A. organizations tree (for directory)

INSERT INTO organizations (id, domain_id, name, code, depth, order_index, status)
VALUES
  ('e2000000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'Engineering Division', 'eng', 0, 1, 'active'),
  ('e2000000-0000-0000-0000-000000000002', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'Marketing Division',   'mkt', 0, 2, 'active'),
  ('e2000000-0000-0000-0000-000000000003', '7049fa6e-d649-44d3-83d2-b548c7e787d5', 'Business Operations',  'biz', 0, 3, 'active')
ON CONFLICT (id) DO UPDATE SET domain_id=EXCLUDED.domain_id, name=EXCLUDED.name,
  code=EXCLUDED.code, depth=EXCLUDED.depth, order_index=EXCLUDED.order_index, status=EXCLUDED.status;

INSERT INTO organizations (id, domain_id, parent_id, name, code, depth, order_index, status)
VALUES
  ('e2100000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000001', 'Product Business Unit',    'pbu',    1, 1, 'active'),
  ('e2100000-0000-0000-0000-000000000002', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000001', 'Technology Business Unit', 'tbu',    1, 2, 'active'),
  ('e2100000-0000-0000-0000-000000000003', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000002', 'Brand Business Unit',      'bbu',    1, 1, 'active'),
  ('e2100000-0000-0000-0000-000000000004', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000002', 'Growth Business Unit',     'gbu',    1, 2, 'active')
ON CONFLICT (id) DO UPDATE SET domain_id=EXCLUDED.domain_id, parent_id=EXCLUDED.parent_id,
  name=EXCLUDED.name, code=EXCLUDED.code, depth=EXCLUDED.depth,
  order_index=EXCLUDED.order_index, status=EXCLUDED.status;

INSERT INTO organizations (id, domain_id, parent_id, name, code, depth, order_index, status)
VALUES
  ('e3000000-0000-0000-0000-000000000001', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2100000-0000-0000-0000-000000000001', 'Backend Team',      'be',    2, 1, 'active'),
  ('e3000000-0000-0000-0000-000000000002', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2100000-0000-0000-0000-000000000001', 'Frontend Team',     'fe',    2, 2, 'active'),
  ('e3000000-0000-0000-0000-000000000003', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2100000-0000-0000-0000-000000000002', 'Infrastructure Team','infra', 2, 1, 'active'),
  ('e3000000-0000-0000-0000-000000000004', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2100000-0000-0000-0000-000000000003', 'Brand Team',        'brand', 2, 1, 'active'),
  ('e3000000-0000-0000-0000-000000000005', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2100000-0000-0000-0000-000000000004', 'Performance Team',  'perf',  2, 1, 'active'),
  ('e3000000-0000-0000-0000-000000000006', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000003', 'HR Team',           'hr',    1, 1, 'active'),
  ('e3000000-0000-0000-0000-000000000007', '7049fa6e-d649-44d3-83d2-b548c7e787d5',
   'e2000000-0000-0000-0000-000000000003', 'Finance Team',      'fin',   1, 2, 'active')
ON CONFLICT (id) DO UPDATE SET domain_id=EXCLUDED.domain_id, parent_id=EXCLUDED.parent_id,
  name=EXCLUDED.name, code=EXCLUDED.code, depth=EXCLUDED.depth,
  order_index=EXCLUDED.order_index, status=EXCLUDED.status;

-- Assign org_id on users
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000001'
WHERE id IN ('30000000-0000-0000-0000-000000000001',
             'a2000000-0000-0000-0000-000000000001',
             'a2000000-0000-0000-0000-000000000008',
             'a2000000-0000-0000-0000-000000000010');
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000002'
WHERE id IN ('a2000000-0000-0000-0000-000000000002',
             'a2000000-0000-0000-0000-000000000009');
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000003'
WHERE id IN ('a2000000-0000-0000-0000-000000000007',
             'a2000000-0000-0000-0000-000000000011');
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000004'
WHERE id IN ('a2000000-0000-0000-0000-000000000003',
             'a2000000-0000-0000-0000-000000000012',
             'a2000000-0000-0000-0000-000000000013');
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000005'
WHERE id IN ('a2000000-0000-0000-0000-000000000004',
             'a2000000-0000-0000-0000-000000000006');
UPDATE users SET org_id = 'e3000000-0000-0000-0000-000000000006'
WHERE id = 'a2000000-0000-0000-0000-000000000005';

-- 5-B. organization_units (orgchart)

INSERT INTO organization_units
  (id, company_id, name, name_normalized, type, display_name, manager_user_id, status)
VALUES
  ('e1000000-0000-0000-0000-000000000001', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'Engineering Division', 'Engineering Division', 'division', 'Engineering Division',
   'a2000000-0000-0000-0000-000000000001', 'active'),
  ('e1000000-0000-0000-0000-000000000002', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'Marketing Division', 'Marketing Division', 'division', 'Marketing Division',
   'a2000000-0000-0000-0000-000000000003', 'active'),
  ('e1000000-0000-0000-0000-000000000003', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'Business Operations', 'Business Operations', 'department', 'Business Operations',
   'a2000000-0000-0000-0000-000000000005', 'active')
ON CONFLICT (id) DO UPDATE SET
  name=EXCLUDED.name, name_normalized=EXCLUDED.name_normalized,
  type=EXCLUDED.type, display_name=EXCLUDED.display_name,
  manager_user_id=EXCLUDED.manager_user_id, status=EXCLUDED.status;

INSERT INTO organization_units
  (id, company_id, parent_id, name, name_normalized, type, display_name, manager_user_id, status)
VALUES
  ('e1000000-0000-0000-0000-000000000004', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000001',
   'Product Business Unit', 'Product Business Unit', 'business_unit', 'Product Business Unit',
   'a2000000-0000-0000-0000-000000000001', 'active'),
  ('e1000000-0000-0000-0000-000000000005', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000001',
   'Technology Business Unit', 'Technology Business Unit', 'business_unit', 'Technology Business Unit',
   'a2000000-0000-0000-0000-000000000007', 'active'),
  ('e1000000-0000-0000-0000-000000000006', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000002',
   'Brand Business Unit', 'Brand Business Unit', 'business_unit', 'Brand Business Unit',
   'a2000000-0000-0000-0000-000000000003', 'active'),
  ('e1000000-0000-0000-0000-000000000007', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000002',
   'Growth Business Unit', 'Growth Business Unit', 'business_unit', 'Growth Business Unit',
   'a2000000-0000-0000-0000-000000000004', 'active')
ON CONFLICT (id) DO UPDATE SET
  parent_id=EXCLUDED.parent_id, name=EXCLUDED.name,
  name_normalized=EXCLUDED.name_normalized, type=EXCLUDED.type,
  display_name=EXCLUDED.display_name,
  manager_user_id=EXCLUDED.manager_user_id, status=EXCLUDED.status;

INSERT INTO organization_units
  (id, company_id, parent_id, name, name_normalized, type, display_name, manager_user_id, status)
VALUES
  ('e1000000-0000-0000-0000-000000000011', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000004',
   'Backend Team', 'Backend Team', 'team', 'Backend Team',
   'a2000000-0000-0000-0000-000000000001', 'active'),
  ('e1000000-0000-0000-0000-000000000012', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000004',
   'Frontend Team', 'Frontend Team', 'team', 'Frontend Team',
   'a2000000-0000-0000-0000-000000000002', 'active'),
  ('e1000000-0000-0000-0000-000000000013', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000005',
   'Infrastructure Team', 'Infrastructure Team', 'team', 'Infrastructure Team',
   'a2000000-0000-0000-0000-000000000007', 'active'),
  ('e1000000-0000-0000-0000-000000000021', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000006',
   'Brand Team', 'Brand Team', 'team', 'Brand Team',
   'a2000000-0000-0000-0000-000000000003', 'active'),
  ('e1000000-0000-0000-0000-000000000022', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000007',
   'Performance Team', 'Performance Team', 'team', 'Performance Team',
   'a2000000-0000-0000-0000-000000000004', 'active'),
  ('e1000000-0000-0000-0000-000000000031', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000003',
   'HR Team', 'HR Team', 'team', 'HR Team',
   'a2000000-0000-0000-0000-000000000005', 'active'),
  ('e1000000-0000-0000-0000-000000000032', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   'e1000000-0000-0000-0000-000000000003',
   'Finance Team', 'Finance Team', 'team', 'Finance Team',
   NULL, 'active')
ON CONFLICT (id) DO UPDATE SET
  parent_id=EXCLUDED.parent_id, name=EXCLUDED.name,
  name_normalized=EXCLUDED.name_normalized, type=EXCLUDED.type,
  display_name=EXCLUDED.display_name,
  manager_user_id=EXCLUDED.manager_user_id, status=EXCLUDED.status;

-- 5-C. organization_members

DELETE FROM organization_members
WHERE user_id IN (
  '30000000-0000-0000-0000-000000000001',
  'a2000000-0000-0000-0000-000000000001', 'a2000000-0000-0000-0000-000000000002',
  'a2000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000004',
  'a2000000-0000-0000-0000-000000000005', 'a2000000-0000-0000-0000-000000000006',
  'a2000000-0000-0000-0000-000000000007', 'a2000000-0000-0000-0000-000000000008',
  'a2000000-0000-0000-0000-000000000009', 'a2000000-0000-0000-0000-000000000010',
  'a2000000-0000-0000-0000-000000000011', 'a2000000-0000-0000-0000-000000000012',
  'a2000000-0000-0000-0000-000000000013'
);

-- Chris Kim: Backend Team lead (P) / Product BU head / Engineering Division head
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000001', 'e1000000-0000-0000-0000-000000000011', 'a2000000-0000-0000-0000-000000000001', 'manager', 'Tech Lead · Senior Engineer', true),
  ('e0000000-0000-0000-0000-000000000002', 'e1000000-0000-0000-0000-000000000004', 'a2000000-0000-0000-0000-000000000001', 'manager', 'BU Head · Senior Engineer', false),
  ('e0000000-0000-0000-0000-000000000003', 'e1000000-0000-0000-0000-000000000001', 'a2000000-0000-0000-0000-000000000001', 'manager', 'Division Head · Senior Engineer', false);

-- Jamie Park: Backend Team senior engineer (demo user)
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000010', 'e1000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000001', 'member', 'Senior Backend Engineer', true);

-- Seok Oh: Backend Team DBA
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000011', 'e1000000-0000-0000-0000-000000000011', 'a2000000-0000-0000-0000-000000000008', 'member', 'Database Engineer', true);

-- Ina Jang: Backend Team junior engineer
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000012', 'e1000000-0000-0000-0000-000000000011', 'a2000000-0000-0000-0000-000000000010', 'member', 'Backend Engineer', true);

-- Yuna Lee: Frontend Team lead
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000013', 'e1000000-0000-0000-0000-000000000012', 'a2000000-0000-0000-0000-000000000002', 'manager', 'Tech Lead · Engineer', true);

-- Jay Song: Frontend Team
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000014', 'e1000000-0000-0000-0000-000000000012', 'a2000000-0000-0000-0000-000000000009', 'member', 'Frontend Engineer', true);

-- Henry Kang: Infrastructure Team lead (P) / Technology BU head
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000015', 'e1000000-0000-0000-0000-000000000013', 'a2000000-0000-0000-0000-000000000007', 'manager', 'Tech Lead · Engineer', true),
  ('e0000000-0000-0000-0000-000000000016', 'e1000000-0000-0000-0000-000000000005', 'a2000000-0000-0000-0000-000000000007', 'manager', 'BU Head · Engineer', false);

-- Woo Baek: Infrastructure Team
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000017', 'e1000000-0000-0000-0000-000000000013', 'a2000000-0000-0000-0000-000000000011', 'member', 'Cloud Engineer', true);

-- Min Park: Brand Team lead (P) / Brand BU head / Marketing Division head
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000020', 'e1000000-0000-0000-0000-000000000021', 'a2000000-0000-0000-0000-000000000003', 'manager', 'Team Lead · Senior Engineer', true),
  ('e0000000-0000-0000-0000-000000000021', 'e1000000-0000-0000-0000-000000000006', 'a2000000-0000-0000-0000-000000000003', 'manager', 'BU Head · Senior Engineer', false),
  ('e0000000-0000-0000-0000-000000000022', 'e1000000-0000-0000-0000-000000000002', 'a2000000-0000-0000-0000-000000000003', 'manager', 'Division Head · Senior Engineer', false);

-- Dora Shim: Brand Team designer
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000023', 'e1000000-0000-0000-0000-000000000021', 'a2000000-0000-0000-0000-000000000012', 'member', 'UI/UX Designer', true);

-- Sean Hong: Brand Team designer
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000024', 'e1000000-0000-0000-0000-000000000021', 'a2000000-0000-0000-0000-000000000013', 'member', 'UI/UX Designer', true);

-- Sue Jung: Performance Team lead (P) / Growth BU head
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000025', 'e1000000-0000-0000-0000-000000000022', 'a2000000-0000-0000-0000-000000000004', 'manager', 'Team Lead · Engineer', true),
  ('e0000000-0000-0000-0000-000000000026', 'e1000000-0000-0000-0000-000000000007', 'a2000000-0000-0000-0000-000000000004', 'manager', 'BU Head · Engineer', false);

-- Jiyeon Han: Performance Team
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000027', 'e1000000-0000-0000-0000-000000000022', 'a2000000-0000-0000-0000-000000000006', 'member', 'Cloud Cost Engineer', true);

-- Jun Choi: HR Team lead (P) / Business Operations head
INSERT INTO organization_members (id, organization_unit_id, user_id, role, title, is_primary) VALUES
  ('e0000000-0000-0000-0000-000000000030', 'e1000000-0000-0000-0000-000000000031', 'a2000000-0000-0000-0000-000000000005', 'manager', 'Team Lead · Director', true),
  ('e0000000-0000-0000-0000-000000000031', 'e1000000-0000-0000-0000-000000000003', 'a2000000-0000-0000-0000-000000000005', 'manager', 'Department Head · Director', false);


-- ══════════════════════════════════════════════════════════════════════════════
-- 6. DEMO USER CONTACTS
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO carddav_addressbooks (id, company_id, domain_id, user_id, name, normalized_name, sync_token, status)
VALUES ('f4000000-0000-0000-0000-000000000001', '7206af4e-fc44-4a65-890d-55bb35741d6c',
  '7049fa6e-d649-44d3-83d2-b548c7e787d5', '30000000-0000-0000-0000-000000000001',
  'Internal Colleagues', 'colleagues', '1', 'active')
ON CONFLICT DO NOTHING;

INSERT INTO carddav_addressbooks (id, company_id, domain_id, user_id, name, normalized_name, sync_token, status)
VALUES ('f4000000-0000-0000-0000-000000000002', '7206af4e-fc44-4a65-890d-55bb35741d6c',
  '7049fa6e-d649-44d3-83d2-b548c7e787d5', '30000000-0000-0000-0000-000000000001',
  'External Contacts', 'external', '1', 'active')
ON CONFLICT DO NOTHING;

-- Internal colleagues (12)
INSERT INTO carddav_contact_objects (id, user_id, addressbook_id, object_name, uid, etag, size, vcard, status)
VALUES
  ('f4100000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'kim.chulsoo.vcf', 'en-col-kim-chulsoo', '"e000000000000000000000000000000000000000000000000000000000000001"', 260,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Chris Kim\r\nN:Kim;Chris;;;\r\nEMAIL;TYPE=WORK:kim.chulsoo@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0001\r\nORG:Acme Corp;Backend Team\r\nTITLE:Tech Lead\r\nNOTE:Backend Team lead. Oversees GoGoMail architecture.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'lee.younghee.vcf', 'en-col-lee-younghee', '"e000000000000000000000000000000000000000000000000000000000000002"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Yuna Lee\r\nN:Lee;Yuna;;;\r\nEMAIL;TYPE=WORK:lee.younghee@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0002\r\nORG:Acme Corp;Frontend Team\r\nTITLE:Tech Lead\r\nNOTE:Frontend Team lead. React/TypeScript expert. Oversees UI components.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'park.minjun.vcf', 'en-col-park-minjun', '"e000000000000000000000000000000000000000000000000000000000000003"', 250,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Min Park\r\nN:Park;Min;;;\r\nEMAIL;TYPE=WORK:park.minjun@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0003\r\nORG:Acme Corp;Brand Team\r\nTITLE:Team Lead\r\nNOTE:Marketing Division Brand Team lead. Campaign strategy.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'jung.sooyeon.vcf', 'en-col-jung-sooyeon', '"e000000000000000000000000000000000000000000000000000000000000004"', 245,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Sue Jung\r\nN:Jung;Sue;;;\r\nEMAIL;TYPE=WORK:jung.sooyeon@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0004\r\nORG:Acme Corp;Performance Team\r\nTITLE:Marketing Specialist\r\nNOTE:Performance marketing expert. Manages company-wide email channel.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'choi.junho.vcf', 'en-col-choi-junho', '"e000000000000000000000000000000000000000000000000000000000000005"', 248,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jun Choi\r\nN:Choi;Jun;;;\r\nEMAIL;TYPE=WORK:choi.junho@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0005\r\nORG:Acme Corp;HR Team\r\nTITLE:Director\r\nNOTE:HR Team lead. Manages performance reviews and compensation.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'han.jiyeon.vcf', 'en-col-han-jiyeon', '"e000000000000000000000000000000000000000000000000000000000000006"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jiyeon Han\r\nN:Han;Jiyeon;;;\r\nEMAIL;TYPE=WORK:han.jiyeon@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0006\r\nTEL;TYPE=CELL:+1-415-555-9006\r\nORG:Acme Corp;Performance Team\r\nTITLE:Cloud Cost Specialist\r\nNOTE:AWS Architect certified. FinOps expert.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'kang.hyunjae.vcf', 'en-col-kang-hyunjae', '"e000000000000000000000000000000000000000000000000000000000000007"', 262,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Henry Kang\r\nN:Kang;Henry;;;\r\nEMAIL;TYPE=WORK:kang.hyunjae@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0007\r\nORG:Acme Corp;Infrastructure Team\r\nTITLE:Security Engineer\r\nNOTE:Vulnerability scanning and penetration testing. CISSP certified.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'oh.seokmin.vcf', 'en-col-oh-seokmin', '"e000000000000000000000000000000000000000000000000000000000000008"', 250,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Seok Oh\r\nN:Oh;Seok;;;\r\nEMAIL;TYPE=WORK:oh.seokmin@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0008\r\nORG:Acme Corp;Backend Team\r\nTITLE:Database Engineer\r\nNOTE:PostgreSQL expert. Query tuning and migration specialist.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'baek.woojin.vcf', 'en-col-baek-woojin', '"e000000000000000000000000000000000000000000000000000000000000009"', 252,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Woo Baek\r\nN:Baek;Woo;;;\r\nEMAIL;TYPE=WORK:baek.woojin@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0009\r\nTEL;TYPE=CELL:+1-415-555-9009\r\nORG:Acme Corp;Infrastructure Team\r\nTITLE:Cloud Engineer\r\nNOTE:Kubernetes/Terraform expert. Infrastructure automation.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'shim.dayoung.vcf', 'en-col-shim-dayoung', '"e00000000000000000000000000000000000000000000000000000000000000a"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Dora Shim\r\nN:Shim;Dora;;;\r\nEMAIL;TYPE=WORK:shim.dayoung@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0010\r\nORG:Acme Corp;Brand Team\r\nTITLE:UI/UX Designer\r\nNOTE:Figma/Zeplin expert. Brand team service UI design.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'hong.seungwoo.vcf', 'en-col-hong-seungwoo', '"e00000000000000000000000000000000000000000000000000000000000000b"', 248,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Sean Hong\r\nN:Hong;Sean;;;\r\nEMAIL;TYPE=WORK:hong.seungwoo@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0011\r\nORG:Acme Corp;Brand Team\r\nTITLE:UI/UX Designer\r\nNOTE:Figma expert. Design system owner.\r\nEND:VCARD',
   'active'),
  ('f4100000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000001',
   'song.jiyul.vcf', 'en-col-song-jiyul', '"e00000000000000000000000000000000000000000000000000000000000000c"', 245,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jay Song\r\nN:Song;Jay;;;\r\nEMAIL;TYPE=WORK:song.jiyul@acme.io\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0012\r\nORG:Acme Corp;Frontend Team\r\nTITLE:Frontend Engineer\r\nNOTE:Next.js/React expert. Mail client UI development.\r\nEND:VCARD',
   'active')
ON CONFLICT DO NOTHING;

-- External contacts (10)
INSERT INTO carddav_contact_objects (id, user_id, addressbook_id, object_name, uid, etag, size, vcard, status)
VALUES
  ('f4200000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'minji.kwon.vcf', 'en-ext-minji-kwon', '"e00000000000000000000000000000000000000000000000000000000000000d"', 370,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Minji Kwon\r\nN:Kwon;Minji;;;\r\nEMAIL;TYPE=WORK:minji.kwon@example-partner.test\r\nEMAIL;TYPE=HOME:minji.personal@gmail.example.test\r\nTEL;TYPE=WORK,VOICE:+1-650-555-0101\r\nTEL;TYPE=CELL:+1-650-555-9101\r\nORG:NextWave Studio\r\nTITLE:Project Manager\r\nADR;TYPE=WORK:;;101 Innovation Dr;San Jose;CA;95110;USA\r\nNOTE:Launch campaign partner PM. Contract under review.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'taeho.seo.vcf', 'en-ext-taeho-seo', '"e00000000000000000000000000000000000000000000000000000000000000e"', 350,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Taeho Seo\r\nN:Seo;Taeho;;;\r\nEMAIL;TYPE=WORK:taeho.seo@example-cloud.test\r\nTEL;TYPE=WORK,VOICE:+1-650-555-0102\r\nTEL;TYPE=CELL:+1-650-555-9102\r\nORG:CloudBridge\r\nTITLE:Solutions Architect\r\nADR;TYPE=WORK:;;200 Cloud Ave;Sunnyvale;CA;94086;USA\r\nNOTE:Infrastructure cost optimization consulting. AWS/GCP expert.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'sarah.lee.vcf', 'en-ext-sarah-lee', '"e00000000000000000000000000000000000000000000000000000000000000f"', 350,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Sarah Lee\r\nN:Lee;Sarah;;;\r\nEMAIL;TYPE=WORK:sarah.lee@example-legal.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0103\r\nTEL;TYPE=CELL:+1-415-555-9103\r\nORG:Open Standard Legal\r\nTITLE:Senior Counsel\r\nADR;TYPE=WORK:;;101 Market St;San Francisco;CA;94105;USA\r\nNOTE:Open source licensing and contract review specialist.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'jisu.nam.vcf', 'en-ext-jisu-nam', '"e000000000000000000000000000000000000000000000000000000000000010"', 340,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jisu Nam\r\nN:Nam;Jisu;;;\r\nEMAIL;TYPE=WORK:jisu.nam@example-design.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0104\r\nORG:Framers Lab\r\nTITLE:Brand Designer\r\nADR;TYPE=WORK:;;789 Design Blvd;San Francisco;CA;94107;USA\r\nNOTE:Product website and brand system collaboration partner.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'jason.kim.vcf', 'en-ext-jason-kim', '"e000000000000000000000000000000000000000000000000000000000000011"', 345,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jason Kim\r\nN:Kim;Jason;;;\r\nEMAIL;TYPE=WORK:jason.kim@example-vc.test\r\nEMAIL;TYPE=HOME:jasonkim@gmail.example.test\r\nTEL;TYPE=WORK,VOICE:+1-650-555-0105\r\nTEL;TYPE=CELL:+1-650-555-8105\r\nORG:Horizon Ventures\r\nTITLE:Partner\r\nADR;TYPE=WORK:;;3000 Sand Hill Rd;Menlo Park;CA;94025;USA\r\nNOTE:Reviewing Series A. Monthly updates shared.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'yumi.tanaka.vcf', 'en-ext-yumi-tanaka', '"e000000000000000000000000000000000000000000000000000000000000012"', 340,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Yumi Tanaka\r\nN:Tanaka;Yumi;;;\r\nEMAIL;TYPE=WORK:yumi.tanaka@example-jp.test\r\nTEL;TYPE=WORK,VOICE:+81-3-5555-0106\r\nTEL;TYPE=CELL:+81-90-1234-0106\r\nORG:Future Stack Japan\r\nTITLE:Business Development Manager\r\nADR;TYPE=WORK:;;2-3-1 Marunouchi;Chiyoda-ku;Tokyo;100-0005;Japan\r\nNOTE:Japan partnership. Quarterly business meetings.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'dongwoo.lim.vcf', 'en-ext-dongwoo-lim', '"e000000000000000000000000000000000000000000000000000000000000013"', 325,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Dongwoo Lim\r\nN:Lim;Dongwoo;;;\r\nEMAIL;TYPE=WORK:dongwoo.lim@example-pr.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0107\r\nTEL;TYPE=CELL:+1-415-555-9107\r\nORG:Platinum PR\r\nTITLE:PR Director\r\nNOTE:Media relations and press releases. Launch PR collaboration.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'hyunwoo.ahn.vcf', 'en-ext-hyunwoo-ahn', '"e000000000000000000000000000000000000000000000000000000000000014"', 318,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Hyunwoo Ahn\r\nN:Ahn;Hyunwoo;;;\r\nEMAIL;TYPE=WORK:hyunwoo.ahn@example-audit.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0108\r\nORG:KR IT Audit Corp\r\nTITLE:Senior Auditor\r\nADR;TYPE=WORK:;;77 Financial Row;New York;NY;10005;USA\r\nNOTE:Annual security audit. SOC2 Type II preparation support.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'claire.martin.vcf', 'en-ext-claire-martin', '"e000000000000000000000000000000000000000000000000000000000000015"', 335,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Claire Martin\r\nN:Martin;Claire;;;\r\nEMAIL;TYPE=WORK:claire.martin@example-eu.test\r\nTEL;TYPE=WORK,VOICE:+33-1-5555-0109\r\nTEL;TYPE=CELL:+33-6-1234-0109\r\nORG:DigitalBridge EU\r\nTITLE:Head of Partnerships\r\nADR;TYPE=WORK:;;25 Rue de la Paix;Paris;;75002;France\r\nNOTE:European partnerships and GDPR compliance.\r\nEND:VCARD',
   'active'),
  ('f4200000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000001', 'f4000000-0000-0000-0000-000000000002',
   'sehoon.cho.vcf', 'en-ext-sehoon-cho', '"e000000000000000000000000000000000000000000000000000000000000016"', 315,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Sehoon Cho\r\nN:Cho;Sehoon;;;\r\nEMAIL;TYPE=WORK:sehoon.cho@example-telecom.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0110\r\nTEL;TYPE=CELL:+1-415-555-9110\r\nORG:KR Telecom Enterprise\r\nTITLE:Enterprise Solutions Manager\r\nNOTE:Prospective enterprise mail customer. Quote sent.\r\nEND:VCARD',
   'active')
ON CONFLICT DO NOTHING;


-- ══════════════════════════════════════════════════════════════════════════════
-- 7. DEMO USER CALENDARS + EVENTS
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO caldav_calendars (id, company_id, domain_id, user_id, name, normalized_name,
  color, description, sync_token, status, slug, timezone)
VALUES
  ('a1000000-0000-0000-0000-000000000001', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   '7049fa6e-d649-44d3-83d2-b548c7e787d5', '30000000-0000-0000-0000-000000000001',
   'Work Calendar', 'work-calendar', '#4285F4', 'Work schedule', 'sync-work-1', 'active',
   'work', 'America/New_York'),
  ('a1000000-0000-0000-0000-000000000002', '7206af4e-fc44-4a65-890d-55bb35741d6c',
   '7049fa6e-d649-44d3-83d2-b548c7e787d5', '30000000-0000-0000-0000-000000000001',
   'Personal Calendar', 'personal-calendar', '#34A853', 'Personal schedule', 'sync-personal-1', 'active',
   'personal', 'America/New_York')
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, color=EXCLUDED.color,
  description=EXCLUDED.description, timezone=EXCLUDED.timezone;

INSERT INTO caldav_calendar_objects (id, user_id, calendar_id, object_name, uid,
  component_type, etag, size, ics, status)
VALUES
  ('a2000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'sprint-kickoff-2026-05-28.ics', 'en-evt-sprint-kickoff-20260528@acme.io',
   'VEVENT', '"0e00000000000000000000000000000000000000000000000000000000000017"', 420,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-sprint-kickoff-20260528@acme.io\r\nSUMMARY:Sprint Kickoff Meeting\r\nDTSTART;TZID=America/New_York:20260528T100000\r\nDTEND;TZID=America/New_York:20260528T110000\r\nLOCATION:Meeting Room A\r\nDESCRIPTION:Q2 Sprint 3 kickoff. All engineering team members required.\r\nORGANIZER:mailto:kim.chulsoo@acme.io\r\nATTENDEE;CN=Jamie Park:mailto:user@acme.io\r\nATTENDEE;CN=Yuna Lee:mailto:lee.younghee@acme.io\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'okr-review-2026-05-30.ics', 'en-evt-okr-review-20260530@acme.io',
   'VEVENT', '"0e00000000000000000000000000000000000000000000000000000000000018"', 440,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-okr-review-20260530@acme.io\r\nSUMMARY:Q2 OKR Mid-Review\r\nDTSTART;TZID=America/New_York:20260530T140000\r\nDTEND;TZID=America/New_York:20260530T160000\r\nLOCATION:Main Conference Room\r\nDESCRIPTION:Q2 OKR check-in. Division results presentation and H2 adjustment discussion.\r\nORGANIZER:mailto:kim.chulsoo@acme.io\r\nATTENDEE;CN=Jamie Park:mailto:user@acme.io\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'townhall-2026-05-22.ics', 'en-evt-townhall-20260522@acme.io',
   'VEVENT', '"0e00000000000000000000000000000000000000000000000000000000000019"', 430,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-townhall-20260522@acme.io\r\nSUMMARY:Company Town Hall\r\nDTSTART;TZID=America/New_York:20260522T140000\r\nDTEND;TZID=America/New_York:20260522T160000\r\nLOCATION:Main Auditorium\r\nDESCRIPTION:CEO presentation and company-wide Q&A session. All staff attendance required.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'sec-patch-deploy-2026-06-03.ics', 'en-evt-sec-patch-20260603@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001a"', 425,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-sec-patch-20260603@acme.io\r\nSUMMARY:[Security] CVE Patch Deployment — gogomail-backend\r\nDTSTART;TZID=America/New_York:20260603T020000\r\nDTEND;TZID=America/New_York:20260603T040000\r\nLOCATION:Remote\r\nDESCRIPTION:CVE-2026-1234 patch deployment. Maintenance window 2-4am EST.\r\nORGANIZER:mailto:user@acme.io\r\nATTENDEE;CN=Henry Kang:mailto:kang.hyunjae@acme.io\r\nATTENDEE;CN=Dora Shim:mailto:shim.dayoung@acme.io\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'launch-planning-2026-06-10.ics', 'en-evt-launch-planning-20260610@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001b"', 455,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-launch-planning-20260610@acme.io\r\nSUMMARY:GoGoMail Service Launch Planning\r\nDTSTART;TZID=America/New_York:20260610T100000\r\nDTEND;TZID=America/New_York:20260610T120000\r\nLOCATION:Meeting Room B\r\nDESCRIPTION:July launch final plan. Cross-team coordination for marketing, engineering, and design.\r\nORGANIZER:mailto:kim.chulsoo@acme.io\r\nATTENDEE;CN=Jamie Park:mailto:user@acme.io\r\nATTENDEE;CN=Min Park:mailto:park.minjun@acme.io\r\nATTENDEE;CN=Sean Hong:mailto:hong.seungwoo@acme.io\r\nSTATUS:TENTATIVE\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'hr-eval-deadline-2026-05-15.ics', 'en-evt-hr-eval-20260515@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001c"', 375,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-hr-eval-20260515@acme.io\r\nSUMMARY:Self-Evaluation Submission Deadline\r\nDTSTART;TZID=America/New_York:20260515T180000\r\nDTEND;TZID=America/New_York:20260515T183000\r\nDESCRIPTION:May performance review self-evaluation due in HR portal.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000001',
   'partner-meeting-2026-06-05.ics', 'en-evt-partner-mtg-20260605@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001d"', 415,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-partner-mtg-20260605@acme.io\r\nSUMMARY:NextWave Studio Contract Meeting\r\nDTSTART;TZID=America/New_York:20260605T150000\r\nDTEND;TZID=America/New_York:20260605T160000\r\nLOCATION:NextWave Studio, 101 Innovation Dr, San Jose CA\r\nDESCRIPTION:Launch campaign agreement final discussion with Minji Kwon (PM).\r\nORGANIZER:mailto:user@acme.io\r\nATTENDEE;CN=Minji Kwon:mailto:minji.kwon@example-partner.test\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  -- Personal calendar (3 events)
  ('a2000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000002',
   'vacation-2026-06-02.ics', 'en-evt-vacation-20260602@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001e"', 375,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-vacation-20260602@acme.io\r\nSUMMARY:Vacation \U0001F334\r\nDTSTART;VALUE=DATE:20260602\r\nDTEND;VALUE=DATE:20260607\r\nDESCRIPTION:Approved PTO 2026-06-02(Mon) through 06-06(Fri), 5 days.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000002',
   'dentist-2026-05-27.ics', 'en-evt-dentist-20260527@acme.io',
   'VEVENT', '"0e0000000000000000000000000000000000000000000000000000000000001f"', 345,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-dentist-20260527@acme.io\r\nSUMMARY:Dentist Appointment\r\nDTSTART;TZID=America/New_York:20260527T190000\r\nDTEND;TZID=America/New_York:20260527T200000\r\nLOCATION:Smile Dental, 500 Main St\r\nDESCRIPTION:Bi-annual checkup and cleaning.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('a2000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000001',
   'a1000000-0000-0000-0000-000000000002',
   'birthday-min-2026-06-15.ics', 'en-evt-birthday-20260615@acme.io',
   'VEVENT', '"e000000000000000000000000000000000000000000000000000000000000020"', 335,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:en-evt-birthday-20260615@acme.io\r\nSUMMARY:Min''s Birthday \U0001F382\r\nDTSTART;VALUE=DATE:20260615\r\nDTEND;VALUE=DATE:20260616\r\nDESCRIPTION:Min Park''s birthday. Prepare cake with the team.\r\nSTATUS:CONFIRMED\r\nRRULE:FREQ=YEARLY\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active')

ON CONFLICT (id) DO UPDATE SET
  object_name=EXCLUDED.object_name, uid=EXCLUDED.uid,
  component_type=EXCLUDED.component_type, etag=EXCLUDED.etag,
  size=EXCLUDED.size, ics=EXCLUDED.ics, status=EXCLUDED.status,
  updated_at=now();

COMMIT;

-- Result check
SELECT 'admin_user'   AS tbl, COUNT(*) FROM users WHERE domain_id='10000000-0000-0000-0000-000000000002'
UNION ALL SELECT 'demo_user',      COUNT(*) FROM users WHERE domain_id='7049fa6e-d649-44d3-83d2-b548c7e787d5' AND username='user'
UNION ALL SELECT 'coworkers',      COUNT(*) FROM users WHERE domain_id='7049fa6e-d649-44d3-83d2-b548c7e787d5' AND username != 'user'
UNION ALL SELECT 'demo_folders',   COUNT(*) FROM folders WHERE user_id='30000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'inbox_msgs',     COUNT(*) FROM messages WHERE user_id='30000000-0000-0000-0000-000000000001' AND folder_id='f3000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'custom_msgs',    COUNT(*) FROM messages WHERE user_id='30000000-0000-0000-0000-000000000001' AND folder_id NOT IN ('f3000000-0000-0000-0000-000000000001','f3000000-0000-0000-0000-000000000002','f3000000-0000-0000-0000-000000000003','f3000000-0000-0000-0000-000000000004','f3000000-0000-0000-0000-000000000005')
UNION ALL SELECT 'contacts',       COUNT(*) FROM carddav_contact_objects WHERE user_id='30000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'calendars',      COUNT(*) FROM caldav_calendars WHERE user_id='30000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'cal_events',     COUNT(*) FROM caldav_calendar_objects WHERE user_id='30000000-0000-0000-0000-000000000001';

