-- Dev seed data for gogomail
-- Admin:  admin@gogomail.io / admin1234
-- Demo:   user@parkjw.org   / pass1234
-- Run:  docker exec -i gogomail-postgres-dev psql -U gogomail -d gogomail < scripts/seed_dev_data.sql

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
        'admin', '시스템 관리자', 'plain:admin1234', 'local', 'admin', 'active')
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
-- 1. DEMO TENANT  parkjw.org
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO companies (id, name, status)
VALUES ('6106af4e-fc44-4a65-890d-55bb35741d6c', '고구마컴퍼니', 'active')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status;

INSERT INTO domains (id, company_id, name, name_ace, status)
VALUES ('6049fa6e-d649-44d3-83d2-b548c7e787d5', '6106af4e-fc44-4a65-890d-55bb35741d6c',
        'parkjw.org', 'parkjw.org', 'active')
ON CONFLICT (id) DO UPDATE SET company_id = EXCLUDED.company_id, name = EXCLUDED.name,
  name_ace = EXCLUDED.name_ace, status = EXCLUDED.status;

-- ── Demo user: user@parkjw.org ────────────────────────────────────────────────

INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES ('20000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
        'user', '박지원', 'plain:pass1234', 'local', 'user', 'active')
ON CONFLICT (domain_id, username) DO UPDATE SET display_name = EXCLUDED.display_name,
  password_hash = EXCLUDED.password_hash, auth_source = EXCLUDED.auth_source,
  role = EXCLUDED.role, status = EXCLUDED.status;

INSERT INTO user_addresses (id, user_id, domain_id, local_part, local_part_ace, domain_ace,
  address, address_ace, is_primary)
VALUES ('20000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001',
  '6049fa6e-d649-44d3-83d2-b548c7e787d5',
  'user', 'user', 'parkjw.org', 'user@parkjw.org', 'user@parkjw.org', true)
ON CONFLICT (address) DO UPDATE SET user_id = EXCLUDED.user_id,
  domain_id = EXCLUDED.domain_id, is_primary = true;

-- ── Supporting users (co-workers, senders) ────────────────────────────────────

INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES
  ('a1000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kim.chulsoo',  '김철수',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.younghee', '이영희',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'park.minjun',  '박민준',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jung.sooyeon', '정수연',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'choi.junho',   '최준호',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'han.jiyeon',   '한지연',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kang.hyunjae', '강현재',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000008', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'oh.seokmin',   '오석민',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000009', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'song.jiyul',   '송지율',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000010', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jang.inkyung', '장인경',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000011', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'baek.woojin',  '백우진',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000012', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'shim.dayoung', '심다영',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000013', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','홍승우',  'plain:pass1234', 'local', 'user', 'active')
ON CONFLICT (domain_id, username) DO NOTHING;

INSERT INTO user_addresses (id, user_id, domain_id, local_part, local_part_ace, domain_ace, address, address_ace, is_primary)
VALUES
  ('b1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kim.chulsoo',  'kim.chulsoo',  'parkjw.org', 'kim.chulsoo@parkjw.org',  'kim.chulsoo@parkjw.org',  true),
  ('b1000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.younghee', 'lee.younghee', 'parkjw.org', 'lee.younghee@parkjw.org', 'lee.younghee@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000003', 'a1000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'park.minjun',  'park.minjun',  'parkjw.org', 'park.minjun@parkjw.org',  'park.minjun@parkjw.org',  true),
  ('b1000000-0000-0000-0000-000000000004', 'a1000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jung.sooyeon', 'jung.sooyeon', 'parkjw.org', 'jung.sooyeon@parkjw.org', 'jung.sooyeon@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000005', 'a1000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'choi.junho',   'choi.junho',   'parkjw.org', 'choi.junho@parkjw.org',   'choi.junho@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000006', 'a1000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'han.jiyeon',   'han.jiyeon',   'parkjw.org', 'han.jiyeon@parkjw.org',   'han.jiyeon@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000007', 'a1000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kang.hyunjae', 'kang.hyunjae', 'parkjw.org', 'kang.hyunjae@parkjw.org', 'kang.hyunjae@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000008', 'a1000000-0000-0000-0000-000000000008', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'oh.seokmin',   'oh.seokmin',   'parkjw.org', 'oh.seokmin@parkjw.org',   'oh.seokmin@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000009', 'a1000000-0000-0000-0000-000000000009', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'song.jiyul',   'song.jiyul',   'parkjw.org', 'song.jiyul@parkjw.org',   'song.jiyul@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000010', 'a1000000-0000-0000-0000-000000000010', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jang.inkyung', 'jang.inkyung', 'parkjw.org', 'jang.inkyung@parkjw.org', 'jang.inkyung@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000011', 'a1000000-0000-0000-0000-000000000011', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'baek.woojin',  'baek.woojin',  'parkjw.org', 'baek.woojin@parkjw.org',  'baek.woojin@parkjw.org',  true),
  ('b1000000-0000-0000-0000-000000000012', 'a1000000-0000-0000-0000-000000000012', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'shim.dayoung', 'shim.dayoung', 'parkjw.org', 'shim.dayoung@parkjw.org', 'shim.dayoung@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000013', 'a1000000-0000-0000-0000-000000000013', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','hong.seungwoo','parkjw.org', 'hong.seungwoo@parkjw.org','hong.seungwoo@parkjw.org', true)
ON CONFLICT DO NOTHING;

INSERT INTO folders (id, user_id, name, full_path, type, system_type, order_index)
SELECT gen_random_uuid(), u.id::uuid, f.name, f.name, 'system', f.stype, f.ord
FROM (VALUES
  ('a1000000-0000-0000-0000-000000000001'), ('a1000000-0000-0000-0000-000000000002'),
  ('a1000000-0000-0000-0000-000000000003'), ('a1000000-0000-0000-0000-000000000004'),
  ('a1000000-0000-0000-0000-000000000005'), ('a1000000-0000-0000-0000-000000000006'),
  ('a1000000-0000-0000-0000-000000000007'), ('a1000000-0000-0000-0000-000000000008'),
  ('a1000000-0000-0000-0000-000000000009'), ('a1000000-0000-0000-0000-000000000010'),
  ('a1000000-0000-0000-0000-000000000011'), ('a1000000-0000-0000-0000-000000000012'),
  ('a1000000-0000-0000-0000-000000000013')
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
  ('f2000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'Inbox',  '/Inbox',  'system', 'inbox',  0),
  ('f2000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', 'Drafts', '/Drafts', 'system', 'drafts', 1),
  ('f2000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', 'Sent',   '/Sent',   'system', 'sent',   2),
  ('f2000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001', 'Trash',  '/Trash',  'system', 'trash',  3),
  ('f2000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000001', 'Spam',   '/Spam',   'system', 'spam',   4)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, full_path = EXCLUDED.full_path,
  type = EXCLUDED.type, system_type = EXCLUDED.system_type, order_index = EXCLUDED.order_index;

INSERT INTO folders (id, user_id, name, full_path, type, order_index)
VALUES
  ('f2000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000001', '프로젝트', '/프로젝트', 'custom', 10),
  ('f2000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000001', '뉴스레터', '/뉴스레터', 'custom', 11),
  ('f2000000-0000-0000-0000-000000000012', '20000000-0000-0000-0000-000000000001', '청구서',   '/청구서',   'custom', 12),
  ('f2000000-0000-0000-0000-000000000013', '20000000-0000-0000-0000-000000000001', '업무',     '/업무',     'custom', 13)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, full_path = EXCLUDED.full_path,
  order_index = EXCLUDED.order_index;

-- ══════════════════════════════════════════════════════════════════════════════
-- 3. DEMO USER INBOX — 15 messages, varied flags/senders/scenarios
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO messages (
  id, tenant_id, domain_id, user_id, folder_id,
  rfc_message_id, thread_id, subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  received_at, sent_at, size, has_attachment,
  flags, status, storage_path, draft_text_body
) VALUES
  -- 1. 안읽음·별표 — 스프린트 킥오프 (김철수)
  ('f2100000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg001@parkjw.org>', 'f2100000-0000-0000-0000-000000000001',
   '[개발팀] 5월 스프린트 킥오프 일정 공유',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 1200, false,
   '{"read":false,"starred":true,"answered":false}'::jsonb, 'active', '',
   '안녕하세요! 이번 주 스프린트 킥오프 일정을 공유드립니다. 수요일 오전 10시 회의실 A입니다. 참석 부탁드립니다.'),

  -- 2. 안읽음 — PR 코드 리뷰 요청 (이영희)
  ('f2100000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg002@parkjw.org>', 'f2100000-0000-0000-0000-000000000002',
   'Re: PR #312 코드 리뷰 요청 - 인증 미들웨어',
   'lee.younghee@parkjw.org', '이영희',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', 980, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'PR #312 검토해 주셨나요? 인증 미들웨어 부분에 제안 드린 변경사항 확인 부탁드립니다. 오늘 머지 예정이라 빠른 리뷰 부탁드려요.'),

  -- 3. 읽음 — Q2 캠페인 협업 요청 CC 포함 (박민준)
  ('f2100000-0000-0000-0000-000000000003',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg003@parkjw.org>', 'f2100000-0000-0000-0000-000000000003',
   'Q2 마케팅 캠페인 랜딩페이지 협업 요청',
   'park.minjun@parkjw.org', '박민준',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb,
   '[{"address":"jung.sooyeon@parkjw.org","name":"정수연"},{"address":"han.jiyeon@parkjw.org","name":"한지연"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours', 1540, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요, 지원님. Q2 캠페인 랜딩페이지 개발 협업 관련하여 연락드립니다. 다음 주 중 미팅 가능하실까요?'),

  -- 4. 읽음·답장완료 — 인사평가 안내 첨부 (최준호)
  ('f2100000-0000-0000-0000-000000000004',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg004@parkjw.org>', 'f2100000-0000-0000-0000-000000000004',
   '5월 인사평가 일정 및 자가평가 제출 안내',
   'choi.junho@parkjw.org', '최준호',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', 2100, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   '5월 정기 인사평가 일정을 안내드립니다. 자가평가서를 5월 15일까지 HR 포털에 제출해 주시기 바랍니다.'),

  -- 5. 안읽음 — 전체 타운홀 미팅 안내 (정수연)
  ('f2100000-0000-0000-0000-000000000005',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg005@parkjw.org>', 'f2100000-0000-0000-0000-000000000005',
   '[전체] 5월 타운홀 미팅 일정 안내',
   'jung.sooyeon@parkjw.org', '정수연',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', 870, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   '이번 달 타운홀 미팅을 5월 22일 오후 2시 대회의실에서 진행합니다. CEO 발표 및 Q&A 세션이 포함되어 있습니다.'),

  -- 6. 읽음·별표 — 클라우드 비용 절감 첨부 (한지연)
  ('f2100000-0000-0000-0000-000000000006',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg006@parkjw.org>', 'f2100000-0000-0000-0000-000000000006',
   '클라우드 인프라 비용 최적화 제안서',
   'han.jiyeon@parkjw.org', '한지연',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days' - INTERVAL '4 hours', NOW() - INTERVAL '2 days' - INTERVAL '4 hours', 3600, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'AWS 비용 분석 결과를 공유드립니다. Reserved Instance 전환과 S3 스토리지 티어 조정으로 월 약 30% 절감 가능합니다.'),

  -- 7. 안읽음·별표 — 서비스 런칭 최종 검토 CC 포함 (김철수)
  ('f2100000-0000-0000-0000-000000000007',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg007@parkjw.org>', 'f2100000-0000-0000-0000-000000000007',
   '[긴급] 신규 서비스 런칭 계획 최종 검토 요청',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb,
   '[{"address":"park.minjun@parkjw.org","name":"박민준"},{"address":"choi.junho@parkjw.org","name":"최준호"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', 4800, true,
   '{"read":false,"starred":true,"answered":false}'::jsonb, 'active', '',
   '지원님, 내달 런칭 예정인 gogomail 서비스의 최종 계획서를 공유드립니다. 기술 검토 완료 후 사인오프 부탁드립니다.'),

  -- 8. 읽음 — DB 쿼리 성능 이슈 (오석민)
  ('f2100000-0000-0000-0000-000000000008',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg008@parkjw.org>', 'f2100000-0000-0000-0000-000000000008',
   'Re: 메일함 목록 API 응답 지연 분석 결과',
   'oh.seokmin@parkjw.org', '오석민',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 days' - INTERVAL '2 hours', NOW() - INTERVAL '3 days' - INTERVAL '2 hours', 1780, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'messages 테이블에 누락된 복합 인덱스를 추가한 후 p99 지연이 820ms → 45ms로 줄었습니다. 마이그레이션 PR 올렸으니 확인 부탁드립니다.'),

  -- 9. 안읽음·첨부 — 보안 취약점 리포트 (강현재)
  ('f2100000-0000-0000-0000-000000000009',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg009@parkjw.org>', 'f2100000-0000-0000-0000-000000000009',
   '[보안] 월간 취약점 스캐닝 리포트 - 2026년 5월',
   'kang.hyunjae@parkjw.org', '강현재',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', 5200, true,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   '5월 보안 스캐닝 결과를 첨부드립니다. CVE-2026-1234 대응 패치가 필요합니다. 이번 주 내로 배포 일정 조율 부탁드립니다.'),

  -- 10. 읽음·답장 — 팀 점심 초대 (백우진)
  ('f2100000-0000-0000-0000-000000000010',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg010@parkjw.org>', 'f2100000-0000-0000-0000-000000000010',
   '이번 주 금요일 팀 점심 같이 하실 분?',
   'baek.woojin@parkjw.org', '백우진',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb,
   '[{"address":"kang.hyunjae@parkjw.org","name":"강현재"},{"address":"oh.seokmin@parkjw.org","name":"오석민"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '4 days' - INTERVAL '3 hours', NOW() - INTERVAL '4 days' - INTERVAL '3 hours', 520, false,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   '이번 주 금요일 12시 30분에 근처 한식집 가려는데 참여 가능하신 분 댓글 달아주세요!'),

  -- 11. 안읽음 — 데브옵스 파이프라인 빌드 실패 알림 (심다영)
  ('f2100000-0000-0000-0000-000000000011',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg011@parkjw.org>', 'f2100000-0000-0000-0000-000000000011',
   '[CI/CD] main 브랜치 빌드 실패 — gogomail-backend',
   'shim.dayoung@parkjw.org', '심다영',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 1100, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'main 브랜치 빌드가 실패했습니다. TestIMAPSearchFastPath 테스트에서 타임아웃이 발생했습니다. 확인 부탁드립니다.'),

  -- 12. 읽음·별표 — 외부 파트너 계약 검토 첨부 (외부)
  ('f2100000-0000-0000-0000-000000000012',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg012@parkjw.org>', 'f2100000-0000-0000-0000-000000000012',
   '[계약] 넥스트웨이브 스튜디오 협업 계약서 검토 요청',
   'minji.kwon@example-partner.test', '권민지',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days' - INTERVAL '6 hours', NOW() - INTERVAL '5 days' - INTERVAL '6 hours', 8500, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   '안녕하세요 박지원님, 런칭 캠페인 협업 계약서 초안을 첨부드립니다. 법무팀 검토 후 회신 부탁드립니다.'),

  -- 13. 읽음 — 홍보 디자인 시안 공유 (홍승우)
  ('f2100000-0000-0000-0000-000000000013',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg013@parkjw.org>', 'f2100000-0000-0000-0000-000000000013',
   'gogomail 서비스 홍보 배너 디자인 시안 v2 공유',
   'hong.seungwoo@parkjw.org', '홍승우',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb,
   '[{"address":"park.minjun@parkjw.org","name":"박민준"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days', 2300, true,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '지원님 피드백 주신 사항 반영해서 시안 v2 첨부드립니다. 색상 팔레트와 타이포그래피를 수정했습니다.'),

  -- 14. 안읽음 — 전사 공지 (외부 도메인 뉴스레터 느낌)
  ('f2100000-0000-0000-0000-000000000014',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg014@parkjw.org>', 'f2100000-0000-0000-0000-000000000014',
   '[전사] 사내 IT 보안 정책 개정 안내 (필독)',
   'choi.junho@parkjw.org', '최준호',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days', 3100, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   '개정된 사내 IT 보안 정책을 안내드립니다. 개인 기기 업무 연결 시 MDM 등록이 의무화됩니다. 6월 1일부터 시행됩니다.'),

  -- 15. 읽음·별표 — 분기 OKR 리뷰 초대 (김철수)
  ('f2100000-0000-0000-0000-000000000015',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000001',
   '<msg015@parkjw.org>', 'f2100000-0000-0000-0000-000000000015',
   'Q2 OKR 중간 리뷰 — 개발본부 결과 공유',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb,
   '[{"address":"lee.younghee@parkjw.org","name":"이영희"},{"address":"oh.seokmin@parkjw.org","name":"오석민"}]'::jsonb,
   '[]'::jsonb,
   NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', 2650, false,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'Q2 개발본부 OKR 중간 리뷰 결과를 공유드립니다. Key Result 3개 중 2개 달성, 1개 70% 진행 중입니다.')

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
  -- ── 프로젝트 폴더 (2개) ──────────────────────────────────────────────────────
  ('f2200000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000010',
   '<proj001@parkjw.org>', 'f2200000-0000-0000-0000-000000000001',
   '[프로젝트] gogomail v2.0 기술 스펙 문서',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days', 12000, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   'gogomail v2.0 기술 스펙 문서 초안을 공유드립니다. IMAP/SMTP 레이어 리팩토링 및 OpenSearch 통합 설계가 포함되어 있습니다.'),

  ('f2200000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000010',
   '<proj002@parkjw.org>', 'f2200000-0000-0000-0000-000000000002',
   '[프로젝트] 마이그레이션 플랜 — Postgres → TimescaleDB 검토',
   'oh.seokmin@parkjw.org', '오석민',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '12 days', NOW() - INTERVAL '12 days', 5400, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'TimescaleDB 전환 검토 결과 공유드립니다. 현재 메일 이벤트 로그 볼륨 기준으로 압축률 약 8배 개선이 예상됩니다.'),

  -- ── 뉴스레터 폴더 (3개) ──────────────────────────────────────────────────────
  ('f2300000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000011',
   '<nl001@devnews.example.test>', 'f2300000-0000-0000-0000-000000000001',
   'Go Weekly #598 — 1.25 릴리즈 노트와 새로운 range-over-func',
   'newsletter@devnews.example.test', 'Go Weekly',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', 8900, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Go 1.25가 릴리즈되었습니다. 이번 버전의 핵심 기능인 range-over-func와 새로운 math/rand/v2 API를 소개합니다.'),

  ('f2300000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000011',
   '<nl002@techdigest.example.test>', 'f2300000-0000-0000-0000-000000000002',
   'Tech Digest: OpenSearch 3.0 주요 변경 사항 요약',
   'digest@techdigest.example.test', 'Tech Digest',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days', 6200, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'OpenSearch 3.0의 주요 변경사항을 정리했습니다. 새로운 k-NN 엔진과 AI/ML 기능 강화가 눈에 띕니다.'),

  ('f2300000-0000-0000-0000-000000000003',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000011',
   '<nl003@cloudnews.example.test>', 'f2300000-0000-0000-0000-000000000003',
   'Cloud Cost Report: AWS vs GCP 2026 Q1 비교 분석',
   'report@cloudnews.example.test', 'Cloud Cost Report',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '9 days', NOW() - INTERVAL '9 days', 7100, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '2026 Q1 AWS vs GCP 실사용 비용 비교 분석입니다. 컨테이너 워크로드 기준 GCP가 평균 12% 저렴한 것으로 나타났습니다.'),

  -- ── 청구서 폴더 (3개) ──────────────────────────────────────────────────────
  ('f2400000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000012',
   '<inv001@aws.example.test>', 'f2400000-0000-0000-0000-000000000001',
   'AWS 청구서 — 2026년 4월 (₩1,842,300)',
   'billing@aws.example.test', 'AWS Billing',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '14 days', NOW() - INTERVAL '14 days', 4200, true,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '2026년 4월 AWS 사용 요금 청구서를 첨부드립니다. EC2(₩890k), RDS(₩520k), S3(₩432k) 순입니다.'),

  ('f2400000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000012',
   '<inv002@github.example.test>', 'f2400000-0000-0000-0000-000000000002',
   'GitHub Enterprise 갱신 청구서 — 연간 구독',
   'billing@github.example.test', 'GitHub Billing',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days', 2800, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   'GitHub Enterprise 연간 구독 갱신 청구서입니다. 30인 기준 $4,200/년입니다. 결제 확인 후 영수증 발송됩니다.'),

  ('f2400000-0000-0000-0000-000000000003',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000012',
   '<inv003@figma.example.test>', 'f2400000-0000-0000-0000-000000000003',
   'Figma 팀 플랜 결제 완료 — 2026년 5월',
   'billing@figma.example.test', 'Figma',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', 1900, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   'Figma 팀 플랜 ($15/editor×5명) 5월분 결제가 완료되었습니다. 영수증은 청구 포털에서 확인하실 수 있습니다.'),

  -- ── 업무 폴더 (2개) ──────────────────────────────────────────────────────────
  ('f2500000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000013',
   '<work001@parkjw.org>', 'f2500000-0000-0000-0000-000000000001',
   '연차 신청 승인 완료 — 2026-06-02 ~ 06-06 (5일)',
   'choi.junho@parkjw.org', '최준호',
   '[{"address":"user@parkjw.org","name":"박지원"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 680, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '박지원님의 연차 신청이 승인되었습니다. 기간: 2026-06-02(월) ~ 06-06(금), 총 5일. 즐거운 휴가 되세요!'),

  ('f2500000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5',
   '20000000-0000-0000-0000-000000000001', 'f2000000-0000-0000-0000-000000000013',
   '<work002@parkjw.org>', 'f2500000-0000-0000-0000-000000000002',
   '[업무] 재택근무 신청서 — 2026년 6월',
   'user@parkjw.org', '박지원',
   '[{"address":"choi.junho@parkjw.org","name":"최준호"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', 920, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요 최준호 팀장님, 6월 재택근무 신청서를 제출합니다. 주 2회(월,수) 재택 희망드립니다. 검토 부탁드립니다.')

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

-- Populate html_body from draft_text_body for all seed messages that lack a
-- storage_path body.  This lets the API return readable body content without
-- needing MinIO-stored MIME emails.
UPDATE messages
SET html_body = '<p>' || draft_text_body || '</p>'
WHERE user_id = '20000000-0000-0000-0000-000000000001'
  AND domain_id = '6049fa6e-d649-44d3-83d2-b548c7e787d5'
  AND storage_path = ''
  AND draft_text_body IS NOT NULL
  AND draft_text_body != ''
  AND (html_body IS NULL OR html_body = '');

-- ══════════════════════════════════════════════════════════════════════════════
-- 5. ORGANIZATION STRUCTURE (parkjw.org)
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO organization_units (id, company_id, name, name_normalized, type, display_name, status)
VALUES
  ('c1000000-0000-0000-0000-000000000001', '6106af4e-fc44-4a65-890d-55bb35741d6c', '개발팀',   '개발팀',   'team',       '개발팀',   'active'),
  ('c1000000-0000-0000-0000-000000000002', '6106af4e-fc44-4a65-890d-55bb35741d6c', '마케팅팀', '마케팅팀', 'team',       '마케팅팀', 'active'),
  ('c1000000-0000-0000-0000-000000000003', '6106af4e-fc44-4a65-890d-55bb35741d6c', '인사팀',   '인사팀',   'department', '인사팀',   'active')
ON CONFLICT DO NOTHING;

INSERT INTO organizations (id, domain_id, name, code, depth, order_index, status)
VALUES
  ('c2000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '개발본부',   'dev', 0, 1, 'active'),
  ('c2000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '마케팅본부', 'mkt', 0, 2, 'active'),
  ('c2000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '경영지원부', 'biz', 0, 3, 'active')
ON CONFLICT (id) DO UPDATE SET domain_id=EXCLUDED.domain_id, name=EXCLUDED.name,
  code=EXCLUDED.code, depth=EXCLUDED.depth, order_index=EXCLUDED.order_index, status=EXCLUDED.status;

INSERT INTO organizations (id, domain_id, parent_id, name, code, depth, order_index, status)
VALUES
  ('c3000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '백엔드팀',     'be',    1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '프론트엔드팀', 'fe',    1, 2, 'active'),
  ('c3000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '인프라팀',     'infra', 1, 3, 'active'),
  ('c3000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000002', '브랜드팀',     'brand', 1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000002', '퍼포먼스팀',   'perf',  1, 2, 'active'),
  ('c3000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000003', '인사팀',       'hr',    1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000003', '재무팀',       'fin',   1, 2, 'active')
ON CONFLICT (id) DO UPDATE SET domain_id=EXCLUDED.domain_id, parent_id=EXCLUDED.parent_id,
  name=EXCLUDED.name, code=EXCLUDED.code, depth=EXCLUDED.depth,
  order_index=EXCLUDED.order_index, status=EXCLUDED.status;

UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000001'
WHERE id IN ('20000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000002'
WHERE id = 'a1000000-0000-0000-0000-000000000002';
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000004'
WHERE id = 'a1000000-0000-0000-0000-000000000003';
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000005'
WHERE id IN ('a1000000-0000-0000-0000-000000000004', 'a1000000-0000-0000-0000-000000000006');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000006'
WHERE id = 'a1000000-0000-0000-0000-000000000005';
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000003'
WHERE id IN ('a1000000-0000-0000-0000-000000000007', 'a1000000-0000-0000-0000-000000000011');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000001'
WHERE id IN ('a1000000-0000-0000-0000-000000000008', 'a1000000-0000-0000-0000-000000000009', 'a1000000-0000-0000-0000-000000000010');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000004'
WHERE id IN ('a1000000-0000-0000-0000-000000000012', 'a1000000-0000-0000-0000-000000000013');

-- ══════════════════════════════════════════════════════════════════════════════
-- 6. DEMO USER CONTACTS (주소록)
-- ══════════════════════════════════════════════════════════════════════════════

-- 주소록 1: 사내 동료
INSERT INTO carddav_addressbooks (id, company_id, domain_id, user_id, name, normalized_name, sync_token, status)
VALUES ('d2000000-0000-0000-0000-000000000001', '6106af4e-fc44-4a65-890d-55bb35741d6c',
  '6049fa6e-d649-44d3-83d2-b548c7e787d5', '20000000-0000-0000-0000-000000000001',
  '사내 동료', 'colleagues', '1', 'active')
ON CONFLICT DO NOTHING;

-- 주소록 2: 외부 연락처
INSERT INTO carddav_addressbooks (id, company_id, domain_id, user_id, name, normalized_name, sync_token, status)
VALUES ('d2000000-0000-0000-0000-000000000002', '6106af4e-fc44-4a65-890d-55bb35741d6c',
  '6049fa6e-d649-44d3-83d2-b548c7e787d5', '20000000-0000-0000-0000-000000000001',
  '외부 연락처', 'external', '1', 'active')
ON CONFLICT DO NOTHING;

-- 사내 동료 연락처 (12명)
INSERT INTO carddav_contact_objects (id, user_id, addressbook_id, object_name, uid, etag, size, vcard, status)
VALUES
  ('e3000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'kim.chulsoo.vcf', 'col-kim-chulsoo', '"0000000000000000000000000000000000000000000000000000000000000001"', 280,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:김철수\r\nN:김;철수;;;\r\nEMAIL;TYPE=WORK:kim.chulsoo@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0001\r\nORG:고구마컴퍼니;백엔드팀\r\nTITLE:팀장\r\nNOTE:개발본부 백엔드팀 팀장. 고고메일 아키텍처 총괄.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'lee.younghee.vcf', 'col-lee-younghee', '"0000000000000000000000000000000000000000000000000000000000000002"', 270,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:이영희\r\nN:이;영희;;;\r\nEMAIL;TYPE=WORK:lee.younghee@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0002\r\nORG:고구마컴퍼니;프론트엔드팀\r\nTITLE:시니어 개발자\r\nNOTE:React/TypeScript 전문. 주요 UI 컴포넌트 개발 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'park.minjun.vcf', 'col-park-minjun', '"0000000000000000000000000000000000000000000000000000000000000003"', 265,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:박민준\r\nN:박;민준;;;\r\nEMAIL;TYPE=WORK:park.minjun@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0003\r\nORG:고구마컴퍼니;브랜드팀\r\nTITLE:팀장\r\nNOTE:마케팅본부 브랜드팀 팀장. 캠페인 기획 총괄.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'jung.sooyeon.vcf', 'col-jung-sooyeon', '"0000000000000000000000000000000000000000000000000000000000000004"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:정수연\r\nN:정;수연;;;\r\nEMAIL;TYPE=WORK:jung.sooyeon@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0004\r\nORG:고구마컴퍼니;퍼포먼스팀\r\nTITLE:마케터\r\nNOTE:퍼포먼스 마케팅 전문. 전사 이메일 채널 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'choi.junho.vcf', 'col-choi-junho', '"0000000000000000000000000000000000000000000000000000000000000005"', 260,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:최준호\r\nN:최;준호;;;\r\nEMAIL;TYPE=WORK:choi.junho@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0005\r\nORG:고구마컴퍼니;인사팀\r\nTITLE:팀장\r\nNOTE:경영지원부 인사팀장. 평가/보상 총괄.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000006', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'han.jiyeon.vcf', 'col-han-jiyeon', '"0000000000000000000000000000000000000000000000000000000000000006"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:한지연\r\nN:한;지연;;;\r\nEMAIL;TYPE=WORK:han.jiyeon@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0006\r\nTEL;TYPE=CELL:+82-10-9876-0006\r\nORG:고구마컴퍼니;퍼포먼스팀\r\nTITLE:클라우드 비용 최적화 담당\r\nNOTE:AWS 아키텍트 자격증 보유. FinOps 전문.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000007', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'kang.hyunjae.vcf', 'col-kang-hyunjae', '"0000000000000000000000000000000000000000000000000000000000000007"', 275,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:강현재\r\nN:강;현재;;;\r\nEMAIL;TYPE=WORK:kang.hyunjae@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0007\r\nORG:고구마컴퍼니;인프라팀\r\nTITLE:보안 엔지니어\r\nNOTE:취약점 스캐닝 및 침투 테스트 담당. CISSP 보유.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000008', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'oh.seokmin.vcf', 'col-oh-seokmin', '"0000000000000000000000000000000000000000000000000000000000000008"', 268,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:오석민\r\nN:오;석민;;;\r\nEMAIL;TYPE=WORK:oh.seokmin@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0008\r\nORG:고구마컴퍼니;백엔드팀\r\nTITLE:DBA\r\nNOTE:PostgreSQL 전문. 쿼리 튜닝 및 마이그레이션 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000009', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'baek.woojin.vcf', 'col-baek-woojin', '"0000000000000000000000000000000000000000000000000000000000000009"', 258,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:백우진\r\nN:백;우진;;;\r\nEMAIL;TYPE=WORK:baek.woojin@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0009\r\nTEL;TYPE=CELL:+82-10-5555-0009\r\nORG:고구마컴퍼니;인프라팀\r\nTITLE:클라우드 엔지니어\r\nNOTE:Kubernetes/Terraform 전문. 인프라 자동화 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'shim.dayoung.vcf', 'col-shim-dayoung', '"000000000000000000000000000000000000000000000000000000000000000a"', 270,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:심다영\r\nN:심;다영;;;\r\nEMAIL;TYPE=WORK:shim.dayoung@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0010\r\nORG:고구마컴퍼니;인프라팀\r\nTITLE:데브옵스 엔지니어\r\nNOTE:CI/CD 파이프라인 관리. GitHub Actions/ArgoCD 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'hong.seungwoo.vcf', 'col-hong-seungwoo', '"000000000000000000000000000000000000000000000000000000000000000b"', 265,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:홍승우\r\nN:홍;승우;;;\r\nEMAIL;TYPE=WORK:hong.seungwoo@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0011\r\nORG:고구마컴퍼니;브랜드팀\r\nTITLE:UI/UX 디자이너\r\nNOTE:Figma 전문. 서비스 디자인 시스템 담당.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000012', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000001',
   'song.jiyul.vcf', 'col-song-jiyul', '"000000000000000000000000000000000000000000000000000000000000000c"', 255,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:송지율\r\nN:송;지율;;;\r\nEMAIL;TYPE=WORK:song.jiyul@parkjw.org\r\nTEL;TYPE=WORK,VOICE:+82-2-1234-0012\r\nORG:고구마컴퍼니;프론트엔드팀\r\nTITLE:프론트엔드 개발자\r\nNOTE:Next.js/React 전문. 메일 클라이언트 UI 개발 담당.\r\nEND:VCARD',
   'active')
ON CONFLICT DO NOTHING;

-- 외부 연락처 (10명, 풍부한 vcard)
INSERT INTO carddav_contact_objects (id, user_id, addressbook_id, object_name, uid, etag, size, vcard, status)
VALUES
  ('e3000000-0000-0000-0000-000000000021', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'minji.kwon.vcf', 'ext-minji-kwon', '"000000000000000000000000000000000000000000000000000000000000000d"', 380,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:권민지\r\nN:권;민지;;;\r\nEMAIL;TYPE=WORK:minji.kwon@example-partner.test\r\nEMAIL;TYPE=HOME:minji.personal@gmail.example.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0101\r\nTEL;TYPE=CELL:+82-10-7777-0101\r\nORG:넥스트웨이브 스튜디오\r\nTITLE:프로젝트 매니저\r\nADR;TYPE=WORK:;;서울시 강남구 테헤란로 123;서울;;06234;대한민국\r\nNOTE:런칭 캠페인 협력사 PM. 계약 검토 중.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000022', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'taeho.seo.vcf', 'ext-taeho-seo', '"000000000000000000000000000000000000000000000000000000000000000e"', 360,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:서태호\r\nN:서;태호;;;\r\nEMAIL;TYPE=WORK:taeho.seo@example-cloud.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0102\r\nTEL;TYPE=CELL:+82-10-8888-0102\r\nORG:클라우드브릿지\r\nTITLE:솔루션 아키텍트\r\nADR;TYPE=WORK:;;서울시 서초구 서초대로 456;서울;;06560;대한민국\r\nNOTE:인프라 비용 최적화 컨설팅 담당. AWS/GCP 전문.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000023', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'sarah.lee.vcf', 'ext-sarah-lee', '"000000000000000000000000000000000000000000000000000000000000000f"', 350,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Sarah Lee\r\nN:Lee;Sarah;;;\r\nEMAIL;TYPE=WORK:sarah.lee@example-legal.test\r\nTEL;TYPE=WORK,VOICE:+1-415-555-0103\r\nTEL;TYPE=CELL:+1-415-555-9103\r\nORG:Open Standard Legal\r\nTITLE:Senior Counsel\r\nADR;TYPE=WORK:;;101 Market St;San Francisco;CA;94105;USA\r\nNOTE:오픈소스 라이선스 및 계약 검토 전문 변호사.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000024', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'jisu.nam.vcf', 'ext-jisu-nam', '"0000000000000000000000000000000000000000000000000000000000000010"', 355,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:남지수\r\nN:남;지수;;;\r\nEMAIL;TYPE=WORK:jisu.nam@example-design.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0104\r\nORG:프레이머스 랩\r\nTITLE:브랜드 디자이너\r\nADR;TYPE=WORK:;;서울시 마포구 월드컵북로 789;서울;;03938;대한민국\r\nNOTE:제품 웹사이트 및 브랜드 시스템 협업 파트너.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000025', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'jason.kim.vcf', 'ext-jason-kim', '"0000000000000000000000000000000000000000000000000000000000000011"', 345,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Jason Kim\r\nN:Kim;Jason;;;\r\nEMAIL;TYPE=WORK:jason.kim@example-vc.test\r\nEMAIL;TYPE=HOME:jasonkim@gmail.example.test\r\nTEL;TYPE=WORK,VOICE:+1-650-555-0105\r\nTEL;TYPE=CELL:+1-650-555-8105\r\nORG:Horizon Ventures\r\nTITLE:Partner\r\nADR;TYPE=WORK:;;3000 Sand Hill Rd;Menlo Park;CA;94025;USA\r\nNOTE:시리즈 A 투자 검토 중. 월 1회 업데이트 공유.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000026', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'yumi.tanaka.vcf', 'ext-yumi-tanaka', '"0000000000000000000000000000000000000000000000000000000000000012"', 340,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Yumi Tanaka\r\nN:Tanaka;Yumi;;;\r\nEMAIL;TYPE=WORK:yumi.tanaka@example-jp.test\r\nTEL;TYPE=WORK,VOICE:+81-3-5555-0106\r\nTEL;TYPE=CELL:+81-90-1234-0106\r\nORG:Future Stack Japan\r\nTITLE:Business Development Manager\r\nADR;TYPE=WORK:;;2-3-1 Marunouchi;Chiyoda-ku;Tokyo;100-0005;Japan\r\nNOTE:일본 파트너십 담당. 분기별 비즈니스 미팅.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000027', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'dongwoo.lim.vcf', 'ext-dongwoo-lim', '"0000000000000000000000000000000000000000000000000000000000000013"', 330,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:임동우\r\nN:임;동우;;;\r\nEMAIL;TYPE=WORK:dongwoo.lim@example-pr.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0107\r\nTEL;TYPE=CELL:+82-10-3333-0107\r\nORG:플래티넘 PR\r\nTITLE:PR 디렉터\r\nNOTE:미디어 릴레이션 및 보도자료 담당. 런칭 PR 협업.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000028', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'hyunwoo.ahn.vcf', 'ext-hyunwoo-ahn', '"0000000000000000000000000000000000000000000000000000000000000014"', 325,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:안현우\r\nN:안;현우;;;\r\nEMAIL;TYPE=WORK:hyunwoo.ahn@example-audit.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0108\r\nORG:한국 IT 감사법인\r\nTITLE:선임 감사역\r\nADR;TYPE=WORK:;;서울시 영등포구 여의대로 77;서울;;07326;대한민국\r\nNOTE:연간 보안 감사 담당. SOC2 Type II 준비 지원.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000029', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'claire.martin.vcf', 'ext-claire-martin', '"0000000000000000000000000000000000000000000000000000000000000015"', 335,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Claire Martin\r\nN:Martin;Claire;;;\r\nEMAIL;TYPE=WORK:claire.martin@example-eu.test\r\nTEL;TYPE=WORK,VOICE:+33-1-5555-0109\r\nTEL;TYPE=CELL:+33-6-1234-0109\r\nORG:DigitalBridge EU\r\nTITLE:Head of Partnerships\r\nADR;TYPE=WORK:;;25 Rue de la Paix;Paris;;75002;France\r\nNOTE:유럽 파트너십 및 GDPR 컴플라이언스 협력.\r\nEND:VCARD',
   'active'),
  ('e3000000-0000-0000-0000-000000000030', '20000000-0000-0000-0000-000000000001', 'd2000000-0000-0000-0000-000000000002',
   'sehoon.cho.vcf', 'ext-sehoon-cho', '"0000000000000000000000000000000000000000000000000000000000000016"', 320,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:조세훈\r\nN:조;세훈;;;\r\nEMAIL;TYPE=WORK:sehoon.cho@example-telecom.test\r\nTEL;TYPE=WORK,VOICE:+82-2-555-0110\r\nTEL;TYPE=CELL:+82-10-2222-0110\r\nORG:KR Telecom Enterprise\r\nTITLE:기업고객 솔루션 담당\r\nNOTE:기업 메일 서비스 도입 검토 고객. 견적 발송 완료.\r\nEND:VCARD',
   'active')
ON CONFLICT DO NOTHING;

-- ══════════════════════════════════════════════════════════════════════════════
-- 7. DEMO USER CALENDARS + EVENTS
-- ══════════════════════════════════════════════════════════════════════════════

INSERT INTO caldav_calendars (id, company_id, domain_id, user_id, name, normalized_name,
  color, description, sync_token, status, slug, timezone)
VALUES
  ('ca000000-0000-0000-0000-000000000001', '6106af4e-fc44-4a65-890d-55bb35741d6c',
   '6049fa6e-d649-44d3-83d2-b548c7e787d5', '20000000-0000-0000-0000-000000000001',
   '업무 캘린더', 'work-calendar', '#4285F4', '업무 일정', 'sync-work-1', 'active',
   'work', 'Asia/Seoul'),
  ('ca000000-0000-0000-0000-000000000002', '6106af4e-fc44-4a65-890d-55bb35741d6c',
   '6049fa6e-d649-44d3-83d2-b548c7e787d5', '20000000-0000-0000-0000-000000000001',
   '개인 캘린더', 'personal-calendar', '#34A853', '개인 일정', 'sync-personal-1', 'active',
   'personal', 'Asia/Seoul')
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, color=EXCLUDED.color,
  description=EXCLUDED.description, timezone=EXCLUDED.timezone;

INSERT INTO caldav_calendar_objects (id, user_id, calendar_id, object_name, uid,
  component_type, etag, size, ics, status)
VALUES
  -- 업무 캘린더 이벤트 7개
  ('cb000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'sprint-kickoff-2026-05-28.ics', 'evt-sprint-kickoff-20260528@parkjw.org',
   'VEVENT', '"0000000000000000000000000000000000000000000000000000000000000017"', 420,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-sprint-kickoff-20260528@parkjw.org\r\nSUMMARY:스프린트 킥오프 회의\r\nDTSTART;TZID=Asia/Seoul:20260528T100000\r\nDTEND;TZID=Asia/Seoul:20260528T110000\r\nLOCATION:회의실 A\r\nDESCRIPTION:Q2 3차 스프린트 킥오프. 개발팀 전원 참석 필수.\r\nORGANIZER:mailto:kim.chulsoo@parkjw.org\r\nATTENDEE;CN=박지원:mailto:user@parkjw.org\r\nATTENDEE;CN=이영희:mailto:lee.younghee@parkjw.org\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'okr-review-2026-05-30.ics', 'evt-okr-review-20260530@parkjw.org',
   'VEVENT', '"0000000000000000000000000000000000000000000000000000000000000018"', 450,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-okr-review-20260530@parkjw.org\r\nSUMMARY:Q2 OKR 중간 리뷰\r\nDTSTART;TZID=Asia/Seoul:20260530T140000\r\nDTEND;TZID=Asia/Seoul:20260530T160000\r\nLOCATION:대회의실\r\nDESCRIPTION:Q2 OKR 중간 점검. 본부별 달성 현황 발표 및 하반기 조정 논의.\r\nORGANIZER:mailto:kim.chulsoo@parkjw.org\r\nATTENDEE;CN=박지원:mailto:user@parkjw.org\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'townhall-2026-05-22.ics', 'evt-townhall-20260522@parkjw.org',
   'VEVENT', '"0000000000000000000000000000000000000000000000000000000000000019"', 440,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-townhall-20260522@parkjw.org\r\nSUMMARY:전사 타운홀 미팅\r\nDTSTART;TZID=Asia/Seoul:20260522T140000\r\nDTEND;TZID=Asia/Seoul:20260522T160000\r\nLOCATION:대강당\r\nDESCRIPTION:CEO 발표 및 전사 Q&A 세션. 전 직원 참석.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'sec-patch-deploy-2026-06-03.ics', 'evt-sec-patch-20260603@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001a"', 430,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-sec-patch-20260603@parkjw.org\r\nSUMMARY:[보안] CVE 패치 배포 — gogomail-backend\r\nDTSTART;TZID=Asia/Seoul:20260603T020000\r\nDTEND;TZID=Asia/Seoul:20260603T040000\r\nLOCATION:원격\r\nDESCRIPTION:CVE-2026-1234 보안 패치 배포. 새벽 2-4시 점검 시간.\r\nORGANIZER:mailto:user@parkjw.org\r\nATTENDEE;CN=강현재:mailto:kang.hyunjae@parkjw.org\r\nATTENDEE;CN=심다영:mailto:shim.dayoung@parkjw.org\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'launch-planning-2026-06-10.ics', 'evt-launch-planning-20260610@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001b"', 460,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-launch-planning-20260610@parkjw.org\r\nSUMMARY:gogomail 서비스 런칭 기획 회의\r\nDTSTART;TZID=Asia/Seoul:20260610T100000\r\nDTEND;TZID=Asia/Seoul:20260610T120000\r\nLOCATION:회의실 B\r\nDESCRIPTION:7월 런칭 최종 계획 확정. 마케팅/개발/디자인 팀 조율.\r\nORGANIZER:mailto:kim.chulsoo@parkjw.org\r\nATTENDEE;CN=박지원:mailto:user@parkjw.org\r\nATTENDEE;CN=박민준:mailto:park.minjun@parkjw.org\r\nATTENDEE;CN=홍승우:mailto:hong.seungwoo@parkjw.org\r\nSTATUS:TENTATIVE\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000006', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'hr-eval-deadline-2026-05-15.ics', 'evt-hr-eval-20260515@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001c"', 380,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-hr-eval-20260515@parkjw.org\r\nSUMMARY:자가평가서 제출 마감\r\nDTSTART;TZID=Asia/Seoul:20260515T180000\r\nDTEND;TZID=Asia/Seoul:20260515T183000\r\nDESCRIPTION:5월 인사평가 자가평가서 HR 포털 제출 마감.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000007', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000001',
   'partner-meeting-2026-06-05.ics', 'evt-partner-mtg-20260605@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001d"', 420,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-partner-mtg-20260605@parkjw.org\r\nSUMMARY:넥스트웨이브 계약 미팅\r\nDTSTART;TZID=Asia/Seoul:20260605T150000\r\nDTEND;TZID=Asia/Seoul:20260605T160000\r\nLOCATION:강남구 테헤란로 123 (넥스트웨이브 스튜디오)\r\nDESCRIPTION:런칭 캠페인 협업 계약 최종 협의. 권민지 PM과 대면 미팅.\r\nORGANIZER:mailto:user@parkjw.org\r\nATTENDEE;CN=권민지:mailto:minji.kwon@example-partner.test\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  -- 개인 캘린더 이벤트 3개
  ('cb000000-0000-0000-0000-000000000008', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000002',
   'vacation-2026-06-02.ics', 'evt-vacation-20260602@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001e"', 380,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-vacation-20260602@parkjw.org\r\nSUMMARY:연차 휴가 🌴\r\nDTSTART;VALUE=DATE:20260602\r\nDTEND;VALUE=DATE:20260607\r\nDESCRIPTION:2026-06-02(월) ~ 06-06(금) 5일 연차 승인 완료.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000009', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000002',
   'dentist-2026-05-27.ics', 'evt-dentist-20260527@parkjw.org',
   'VEVENT', '"000000000000000000000000000000000000000000000000000000000000001f"', 350,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-dentist-20260527@parkjw.org\r\nSUMMARY:치과 정기검진\r\nDTSTART;TZID=Asia/Seoul:20260527T190000\r\nDTEND;TZID=Asia/Seoul:20260527T200000\r\nLOCATION:강남역 스마일 치과\r\nDESCRIPTION:6개월 정기 스케일링 예약.\r\nSTATUS:CONFIRMED\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active'),

  ('cb000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000001',
   'ca000000-0000-0000-0000-000000000002',
   'birthday-minjun-2026-06-15.ics', 'evt-birthday-20260615@parkjw.org',
   'VEVENT', '"0000000000000000000000000000000000000000000000000000000000000020"', 340,
   E'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//GoGoMail//CalDAV//EN\r\nBEGIN:VEVENT\r\nUID:evt-birthday-20260615@parkjw.org\r\nSUMMARY:박민준 생일 🎂\r\nDTSTART;VALUE=DATE:20260615\r\nDTEND;VALUE=DATE:20260616\r\nDESCRIPTION:민준이 생일. 팀원들과 케이크 준비할 것.\r\nSTATUS:CONFIRMED\r\nRRULE:FREQ=YEARLY\r\nEND:VEVENT\r\nEND:VCALENDAR',
   'active')

ON CONFLICT (id) DO UPDATE SET
  object_name=EXCLUDED.object_name, uid=EXCLUDED.uid,
  component_type=EXCLUDED.component_type, etag=EXCLUDED.etag,
  size=EXCLUDED.size, ics=EXCLUDED.ics, status=EXCLUDED.status,
  updated_at=now();

COMMIT;

-- 결과 확인
SELECT 'admin_user'   AS tbl, COUNT(*) FROM users WHERE domain_id='10000000-0000-0000-0000-000000000002'
UNION ALL SELECT 'demo_user',      COUNT(*) FROM users WHERE domain_id='6049fa6e-d649-44d3-83d2-b548c7e787d5' AND username='user'
UNION ALL SELECT 'coworkers',      COUNT(*) FROM users WHERE domain_id='6049fa6e-d649-44d3-83d2-b548c7e787d5' AND username != 'user'
UNION ALL SELECT 'demo_folders',   COUNT(*) FROM folders WHERE user_id='20000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'inbox_msgs',     COUNT(*) FROM messages WHERE user_id='20000000-0000-0000-0000-000000000001' AND folder_id='f2000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'custom_msgs',    COUNT(*) FROM messages WHERE user_id='20000000-0000-0000-0000-000000000001' AND folder_id NOT IN ('f2000000-0000-0000-0000-000000000001','f2000000-0000-0000-0000-000000000002','f2000000-0000-0000-0000-000000000003','f2000000-0000-0000-0000-000000000004','f2000000-0000-0000-0000-000000000005')
UNION ALL SELECT 'contacts',       COUNT(*) FROM carddav_contact_objects WHERE user_id='20000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'calendars',      COUNT(*) FROM caldav_calendars WHERE user_id='20000000-0000-0000-0000-000000000001'
UNION ALL SELECT 'cal_events',     COUNT(*) FROM caldav_calendar_objects WHERE user_id='20000000-0000-0000-0000-000000000001';
