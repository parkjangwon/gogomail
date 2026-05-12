-- Dev seed data for gogomail
-- Company: 고구마컴퍼니 (6106af4e-fc44-4a65-890d-55bb35741d6c)
-- Domain:  parkjw.org    (6049fa6e-d649-44d3-83d2-b548c7e787d5)
-- Run:  docker exec -i gogomail-postgres-dev psql -U gogomail -d gogomail < scripts/seed_dev_data.sql

BEGIN;

-- ── 1. Users ──────────────────────────────────────────────────────────────────

INSERT INTO users (id, domain_id, username, display_name, password_hash, auth_source, role, status)
VALUES
  ('a1000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kim.chulsoo',  '김철수',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.younghee', '이영희',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'park.minjun',  '박민준',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jung.sooyeon', '정수연',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'choi.junho',   '최준호',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'han.jiyeon',   '한지연',  'plain:pass1234', 'local', 'user', 'active'),
  -- Additional users for depth-2 teams
  ('a1000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'kang.hyunjae', '강현재',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000008', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'oh.seokmin',   '오석민',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000009', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'song.jiyul',   '송지율',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000010', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'jang.inkyung', '장인경',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000011', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'baek.woojin',  '백우진',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000012', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'shim.dayoung', '심다영',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000013', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','홍승우',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000014', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'yoon.jieun',   '윤지은',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000015', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'bae.jinhun',   '배진흔',  'plain:pass1234', 'local', 'user', 'active'),
  ('a1000000-0000-0000-0000-000000000016', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.hyejin',   '이혜진',  'plain:pass1234', 'local', 'user', 'active')
ON CONFLICT (domain_id, username) DO NOTHING;

-- ── 2. User email addresses ────────────────────────────────────────────────────

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
  ('b1000000-0000-0000-0000-000000000013', 'a1000000-0000-0000-0000-000000000013', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'hong.seungwoo','hong.seungwoo', 'parkjw.org', 'hong.seungwoo@parkjw.org','hong.seungwoo@parkjw.org', true),
  ('b1000000-0000-0000-0000-000000000014', 'a1000000-0000-0000-0000-000000000014', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'yoon.jieun',   'yoon.jieun',   'parkjw.org', 'yoon.jieun@parkjw.org',   'yoon.jieun@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000015', 'a1000000-0000-0000-0000-000000000015', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'bae.jinhun',   'bae.jinhun',   'parkjw.org', 'bae.jinhun@parkjw.org',   'bae.jinhun@parkjw.org',   true),
  ('b1000000-0000-0000-0000-000000000016', 'a1000000-0000-0000-0000-000000000016', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'lee.hyejin',   'lee.hyejin',   'parkjw.org', 'lee.hyejin@parkjw.org',   'lee.hyejin@parkjw.org',   true)
ON CONFLICT DO NOTHING;

-- ── 3. Folders for new users ──────────────────────────────────────────────────

INSERT INTO folders (id, user_id, name, full_path, type, system_type, order_index)
SELECT gen_random_uuid(), u.id::uuid, f.name, f.name, 'system', f.stype, f.ord
FROM (VALUES
  ('a1000000-0000-0000-0000-000000000001'),
  ('a1000000-0000-0000-0000-000000000002'),
  ('a1000000-0000-0000-0000-000000000003'),
  ('a1000000-0000-0000-0000-000000000004'),
  ('a1000000-0000-0000-0000-000000000005'),
  ('a1000000-0000-0000-0000-000000000006')
) AS u(id)
CROSS JOIN (VALUES
  ('Inbox',  'inbox',  1),
  ('Sent',   'sent',   2),
  ('Drafts', 'drafts', 3),
  ('Trash',  'trash',  4)
) AS f(name, stype, ord)
WHERE NOT EXISTS (
  SELECT 1 FROM folders fo WHERE fo.user_id = u.id::uuid AND fo.system_type = f.stype
);

-- ── 4. Organisation units (legacy table, kept for reference) ──────────────────

INSERT INTO organization_units (id, company_id, name, name_normalized, type, display_name, status)
VALUES
  ('c1000000-0000-0000-0000-000000000001', '6106af4e-fc44-4a65-890d-55bb35741d6c', '개발팀',   '개발팀',   'team',       '개발팀',   'active'),
  ('c1000000-0000-0000-0000-000000000002', '6106af4e-fc44-4a65-890d-55bb35741d6c', '마케팅팀', '마케팅팀', 'team',       '마케팅팀', 'active'),
  ('c1000000-0000-0000-0000-000000000003', '6106af4e-fc44-4a65-890d-55bb35741d6c', '인사팀',   '인사팀',   'department', '인사팀',   'active')
ON CONFLICT DO NOTHING;

-- ── 5. Organisation members (legacy) ──────────────────────────────────────────

INSERT INTO organization_members (id, organization_unit_id, user_id, role, is_primary)
VALUES
  -- 개발팀: pjw (manager), 김철수 (manager), 이영희 (member)
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000001', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'manager', true),
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000001', 'manager', true),
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000001', 'a1000000-0000-0000-0000-000000000002', 'member',  true),
  -- 마케팅팀: 박민준 (manager), 정수연 (member), 한지연 (member)
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000003', 'manager', true),
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000004', 'member',  true),
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000002', 'a1000000-0000-0000-0000-000000000006', 'member',  true),
  -- 인사팀: 최준호 (manager)
  (gen_random_uuid(), 'c1000000-0000-0000-0000-000000000003', 'a1000000-0000-0000-0000-000000000005', 'manager', true)
ON CONFLICT DO NOTHING;

-- ── 5b. Organizations (directory table used by SearchPrincipals / org-tree) ───
-- Level 0: 본부/부서 (root)
INSERT INTO organizations (id, domain_id, name, code, depth, order_index, status)
VALUES
  ('c2000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '개발본부',   'dev',  0, 1, 'active'),
  ('c2000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '마케팅본부', 'mkt',  0, 2, 'active'),
  ('c2000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', '경영지원부', 'biz',  0, 3, 'active')
ON CONFLICT DO NOTHING;

-- Level 1: 팀 (under 개발본부)
INSERT INTO organizations (id, domain_id, parent_id, name, code, depth, order_index, status)
VALUES
  ('c3000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '백엔드팀',     'be',   1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '프론트엔드팀', 'fe',   1, 2, 'active'),
  ('c3000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000001', '인프라팀',     'infra',1, 3, 'active'),
  -- under 마케팅본부
  ('c3000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000002', '브랜드팀',     'brand',1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000002', '퍼포먼스팀',   'perf', 1, 2, 'active'),
  -- under 경영지원부
  ('c3000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000003', '인사팀',       'hr',   1, 1, 'active'),
  ('c3000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c2000000-0000-0000-0000-000000000003', '재무팀',       'fin',  1, 2, 'active')
ON CONFLICT DO NOTHING;

-- Level 2: 그룹/팀 (under 백엔드팀)
INSERT INTO organizations (id, domain_id, parent_id, name, code, depth, order_index, status)
VALUES
  ('c4000000-0000-0000-0000-000000000001', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000001', '인프라 그룹',   'be-infra', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000002', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000001', '데이터베이스 그룹', 'be-db', 2, 2, 'active'),
  -- under 프론트엔드팀
  ('c4000000-0000-0000-0000-000000000003', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000002', '웹 개발 그룹', 'fe-web', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000004', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000002', '모바일 개발 그룹', 'fe-mobile', 2, 2, 'active'),
  -- under 인프라팀
  ('c4000000-0000-0000-0000-000000000005', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000003', '클라우드 운영 그룹', 'infra-cloud', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000006', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000003', '데브옵스 그룹', 'infra-devops', 2, 2, 'active'),
  -- under 브랜드팀
  ('c4000000-0000-0000-0000-000000000007', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000004', '크리에이티브 그룹', 'brand-creative', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000008', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000004', '컨텐츠 그룹', 'brand-content', 2, 2, 'active'),
  -- under 퍼포먼스팀
  ('c4000000-0000-0000-0000-000000000009', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000005', '광고 운영 그룹', 'perf-ads', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000010', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000005', '분석 그룹', 'perf-analytics', 2, 2, 'active'),
  -- under 인사팀
  ('c4000000-0000-0000-0000-000000000011', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000006', '채용 그룹', 'hr-recruitment', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000012', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000006', '보상 그룹', 'hr-compensation', 2, 2, 'active'),
  -- under 재무팀
  ('c4000000-0000-0000-0000-000000000013', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000007', '회계 그룹', 'fin-accounting', 2, 1, 'active'),
  ('c4000000-0000-0000-0000-000000000014', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'c3000000-0000-0000-0000-000000000007', '예산 그룹', 'fin-budget', 2, 2, 'active')
ON CONFLICT DO NOTHING;

-- Link users to leaf-team org_id (depth 1 teams for original users)
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000001'  -- 백엔드팀
WHERE id IN ('f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'a1000000-0000-0000-0000-000000000001');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000002'  -- 프론트엔드팀
WHERE id = 'a1000000-0000-0000-0000-000000000002';
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000004'  -- 브랜드팀
WHERE id = 'a1000000-0000-0000-0000-000000000003';
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000005'  -- 퍼포먼스팀
WHERE id IN ('a1000000-0000-0000-0000-000000000004', 'a1000000-0000-0000-0000-000000000006');
UPDATE users SET org_id = 'c3000000-0000-0000-0000-000000000006'  -- 인사팀
WHERE id = 'a1000000-0000-0000-0000-000000000005';

-- Link new users to depth-2 groups
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000001' WHERE id = 'a1000000-0000-0000-0000-000000000007';  -- 강현재 → 인프라 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000002' WHERE id = 'a1000000-0000-0000-0000-000000000008';  -- 오석민 → 데이터베이스 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000003' WHERE id = 'a1000000-0000-0000-0000-000000000009';  -- 송지율 → 웹 개발 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000004' WHERE id = 'a1000000-0000-0000-0000-000000000010';  -- 장인경 → 모바일 개발 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000005' WHERE id = 'a1000000-0000-0000-0000-000000000011';  -- 백우진 → 클라우드 운영 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000006' WHERE id = 'a1000000-0000-0000-0000-000000000012';  -- 심다영 → 데브옵스 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000007' WHERE id = 'a1000000-0000-0000-0000-000000000013';  -- 홍승우 → 크리에이티브 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000008' WHERE id = 'a1000000-0000-0000-0000-000000000014';  -- 윤지은 → 컨텐츠 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000009' WHERE id = 'a1000000-0000-0000-0000-000000000015';  -- 배진흔 → 광고 운영 그룹
UPDATE users SET org_id = 'c4000000-0000-0000-0000-000000000010' WHERE id = 'a1000000-0000-0000-0000-000000000016';  -- 이혜진 → 분석 그룹

-- ── 6. pjw 주소록 + 연락처 ──────────────────────────────────────────────────────

INSERT INTO carddav_addressbooks (id, company_id, domain_id, user_id, name, normalized_name, sync_token, status)
VALUES (
  'd1000000-0000-0000-0000-000000000001',
  '6106af4e-fc44-4a65-890d-55bb35741d6c',
  '6049fa6e-d649-44d3-83d2-b548c7e787d5',
  'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
  '주소록', 'addressbook', '1', 'active'
) ON CONFLICT DO NOTHING;

INSERT INTO carddav_contact_objects (id, user_id, addressbook_id, object_name, uid, etag, size, vcard, status)
VALUES
  ('e1000000-0000-0000-0000-000000000001', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'kim.chulsoo.vcf', 'kim-chulsoo-uid', '"0000000000000000000000000000000000000000000000000000000000000001"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:김철수\r\nEMAIL:kim.chulsoo@parkjw.org\r\nORG:백엔드팀\r\nTITLE:팀장\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000002', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'lee.younghee.vcf', 'lee-younghee-uid', '"0000000000000000000000000000000000000000000000000000000000000002"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:이영희\r\nEMAIL:lee.younghee@parkjw.org\r\nORG:프론트엔드팀\r\nTITLE:개발자\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000003', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'park.minjun.vcf', 'park-minjun-uid', '"0000000000000000000000000000000000000000000000000000000000000003"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:박민준\r\nEMAIL:park.minjun@parkjw.org\r\nORG:브랜드팀\r\nTITLE:팀장\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000004', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'jung.sooyeon.vcf', 'jung-sooyeon-uid', '"0000000000000000000000000000000000000000000000000000000000000004"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:정수연\r\nEMAIL:jung.sooyeon@parkjw.org\r\nORG:퍼포먼스팀\r\nTITLE:마케터\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000005', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'choi.junho.vcf', 'choi-junho-uid', '"0000000000000000000000000000000000000000000000000000000000000005"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:최준호\r\nEMAIL:choi.junho@parkjw.org\r\nORG:인사팀\r\nTITLE:팀장\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000006', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'han.jiyeon.vcf', 'han-jiyeon-uid', '"0000000000000000000000000000000000000000000000000000000000000006"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:한지연\r\nEMAIL:han.jiyeon@parkjw.org\r\nORG:퍼포먼스팀\r\nTITLE:마케터\r\nEND:VCARD', 'active'),
  -- New contacts from depth-2 teams
  ('e1000000-0000-0000-0000-000000000007', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'kang.hyunjae.vcf', 'kang-hyunjae-uid', '"0000000000000000000000000000000000000000000000000000000000000007"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:강현재\r\nEMAIL:kang.hyunjae@parkjw.org\r\nORG:인프라 그룹\r\nTITLE:엔지니어\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000008', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'oh.seokmin.vcf', 'oh-seokmin-uid', '"0000000000000000000000000000000000000000000000000000000000000008"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:오석민\r\nEMAIL:oh.seokmin@parkjw.org\r\nORG:데이터베이스 그룹\r\nTITLE:DBA\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000009', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'song.jiyul.vcf', 'song-jiyul-uid', '"0000000000000000000000000000000000000000000000000000000000000009"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:송지율\r\nEMAIL:song.jiyul@parkjw.org\r\nORG:웹 개발 그룹\r\nTITLE:프론트엔드 개발자\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000010', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'jang.inkyung.vcf', 'jang-inkyung-uid', '"0000000000000000000000000000000000000000000000000000000000000010"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:장인경\r\nEMAIL:jang.inkyung@parkjw.org\r\nORG:모바일 개발 그룹\r\nTITLE:모바일 개발자\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000011', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'baek.woojin.vcf', 'baek-woojin-uid', '"0000000000000000000000000000000000000000000000000000000000000011"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:백우진\r\nEMAIL:baek.woojin@parkjw.org\r\nORG:클라우드 운영 그룹\r\nTITLE:클라우드 엔지니어\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000012', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'shim.dayoung.vcf', 'shim-dayoung-uid', '"0000000000000000000000000000000000000000000000000000000000000012"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:심다영\r\nEMAIL:shim.dayoung@parkjw.org\r\nORG:데브옵스 그룹\r\nTITLE:데브옵스 엔지니어\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000013', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'hong.seungwoo.vcf', 'hong-seungwoo-uid', '"0000000000000000000000000000000000000000000000000000000000000013"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:홍승우\r\nEMAIL:hong.seungwoo@parkjw.org\r\nORG:크리에이티브 그룹\r\nTITLE:디자이너\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000014', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'yoon.jieun.vcf', 'yoon-jieun-uid', '"0000000000000000000000000000000000000000000000000000000000000014"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:윤지은\r\nEMAIL:yoon.jieun@parkjw.org\r\nORG:컨텐츠 그룹\r\nTITLE:컨텐츠 크리에이터\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000015', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'bae.jinhun.vcf', 'bae-jinhun-uid', '"0000000000000000000000000000000000000000000000000000000000000015"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:배진흔\r\nEMAIL:bae.jinhun@parkjw.org\r\nORG:광고 운영 그룹\r\nTITLE:광고운영\r\nEND:VCARD', 'active'),
  ('e1000000-0000-0000-0000-000000000016', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c', 'd1000000-0000-0000-0000-000000000001',
   'lee.hyejin.vcf', 'lee-hyejin-uid', '"0000000000000000000000000000000000000000000000000000000000000016"', 150,
   E'BEGIN:VCARD\r\nVERSION:3.0\r\nFN:이혜진\r\nEMAIL:lee.hyejin@parkjw.org\r\nORG:분석 그룹\r\nTITLE:데이터 애널리스트\r\nEND:VCARD', 'active')
ON CONFLICT DO NOTHING;

-- ── 7. pjw 수신함 테스트 메일 ────────────────────────────────────────────────────

INSERT INTO messages (
  id, tenant_id, domain_id, user_id, folder_id,
  rfc_message_id, thread_id,
  subject, from_addr, from_name,
  to_addrs, cc_addrs, bcc_addrs,
  received_at, sent_at, size, has_attachment,
  flags, status, storage_path, draft_text_body
)
VALUES
  -- 1
  ('f1000000-0000-0000-0000-000000000001',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg1@parkjw.org>', 'f1000000-0000-0000-0000-000000000001',
   '[개발팀] 5월 스프린트 킥오프 일정 공유',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', 1200, false,
   '{"read":false,"starred":true,"answered":false}'::jsonb, 'active', '',
   '안녕하세요 팀장님, 이번 주 스프린트 킥오프 일정을 공유드립니다. 수요일 오전 10시에 회의실 A에서 진행 예정입니다. 참석 부탁드립니다.'),
  -- 2
  ('f1000000-0000-0000-0000-000000000002',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg2@parkjw.org>', 'f1000000-0000-0000-0000-000000000002',
   'Re: PR #247 코드 리뷰 요청',
   'lee.younghee@parkjw.org', '이영희',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 hours', NOW() - INTERVAL '5 hours', 980, false,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요! PR #247 검토해 주셨나요? 인증 미들웨어 부분에 제안 드린 변경사항 확인 부탁드립니다. 오늘 머지 예정이라 빠른 리뷰 부탁드려요.'),
  -- 3
  ('f1000000-0000-0000-0000-000000000003',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg3@parkjw.org>', 'f1000000-0000-0000-0000-000000000003',
   'Q2 마케팅 캠페인 협업 요청',
   'park.minjun@parkjw.org', '박민준',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb,
   '[{"address":"jung.sooyeon@parkjw.org","name":"정수연"}]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', 1540, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요, 장원님. Q2 캠페인 랜딩 페이지 개발 협업 관련하여 연락드립니다. 다음 주 중 미팅 가능하실까요? 상세 기획서 첨부드립니다.'),
  -- 4
  ('f1000000-0000-0000-0000-000000000004',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg4@parkjw.org>', 'f1000000-0000-0000-0000-000000000004',
   '5월 인사평가 일정 및 자가평가 제출 안내',
   'choi.junho@parkjw.org', '최준호',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', 2100, true,
   '{"read":true,"starred":false,"answered":true}'::jsonb, 'active', '',
   '안녕하세요. 5월 정기 인사평가 일정을 안내드립니다. 자가평가서를 5월 15일까지 HR 포털에 제출해 주시기 바랍니다. 문의사항은 인사팀으로 연락주세요.'),
  -- 5
  ('f1000000-0000-0000-0000-000000000005',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg5@parkjw.org>', 'f1000000-0000-0000-0000-000000000005',
   '[전체] 5월 타운홀 미팅 일정 안내',
   'jung.sooyeon@parkjw.org', '정수연',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', 870, false,
   '{"read":true,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요. 이번 달 타운홀 미팅을 5월 22일 오후 2시에 대회의실에서 진행합니다. CEO 발표 및 Q&A 세션이 포함되어 있습니다. 많은 참석 부탁드립니다.'),
  -- 6
  ('f1000000-0000-0000-0000-000000000006',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg6@parkjw.org>', 'f1000000-0000-0000-0000-000000000006',
   '클라우드 인프라 비용 최적화 제안',
   'han.jiyeon@parkjw.org', '한지연',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb, '[]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', 1680, true,
   '{"read":false,"starred":false,"answered":false}'::jsonb, 'active', '',
   '안녕하세요 장원님. AWS 비용 분석 결과를 공유드립니다. Reserved Instance 전환과 S3 스토리지 티어 조정으로 월 약 30% 절감 가능합니다. 검토 후 회신 부탁드립니다.'),
  -- 7 (starred important)
  ('f1000000-0000-0000-0000-000000000007',
   '6106af4e-fc44-4a65-890d-55bb35741d6c', '6049fa6e-d649-44d3-83d2-b548c7e787d5', 'f4b5a283-d1e6-47a9-a69a-e71e90f5343c',
   '124979e1-0e59-4577-8ec6-72d5b89b9834',
   '<msg7@parkjw.org>', 'f1000000-0000-0000-0000-000000000007',
   '신규 서비스 런칭 계획 최종 검토 요청',
   'kim.chulsoo@parkjw.org', '김철수',
   '[{"address":"pjw@parkjw.org","name":"Jangwon Park"}]'::jsonb,
   '[{"address":"park.minjun@parkjw.org","name":"박민준"}]'::jsonb, '[]'::jsonb,
   NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 3200, true,
   '{"read":true,"starred":true,"answered":false}'::jsonb, 'active', '',
   '장원님, 내달 런칭 예정인 gogomail 서비스의 최종 계획서를 공유드립니다. 기술 검토 완료 후 사인오프 부탁드립니다. 마케팅팀과 협의된 출시 일정도 포함되어 있습니다.')
ON CONFLICT DO NOTHING;

COMMIT;

-- 결과 확인
SELECT 'users' AS tbl, COUNT(*) FROM users WHERE domain_id='6049fa6e-d649-44d3-83d2-b548c7e787d5'
UNION ALL SELECT 'org_units', COUNT(*) FROM organization_units WHERE company_id='6106af4e-fc44-4a65-890d-55bb35741d6c'
UNION ALL SELECT 'org_members', COUNT(*) FROM organization_members
UNION ALL SELECT 'contacts', COUNT(*) FROM carddav_contact_objects WHERE user_id='f4b5a283-d1e6-47a9-a69a-e71e90f5343c'
UNION ALL SELECT 'inbox_msgs', COUNT(*) FROM messages WHERE user_id='f4b5a283-d1e6-47a9-a69a-e71e90f5343c' AND folder_id='124979e1-0e59-4577-8ec6-72d5b89b9834';
