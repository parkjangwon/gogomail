import { consoleTerms, localeCodes, type DocsLocale, webmailTerms } from './terms';

type Link = {
  text: string;
  link: string;
};

type PageSection = {
  id?: string;
  title: string;
  body: string;
  paragraphs?: string[];
  items?: string[];
  aliases?: string[];
  examples?: Array<{
    title: string;
    language: string;
    code: string;
  }>;
};

type PageMessage = {
  eyebrow: string;
  title: string;
  lead: string;
  primaryCta?: Link;
  secondaryCta?: Link;
  sections: PageSection[];
};

export type DocsMessages = {
  site: {
    title: string;
    description: string;
    label: string;
    search: string;
    openSource: string;
    lastUpdated: string;
  };
  code: {
    copy: string;
    copied: string;
  };
  nav: {
    home: string;
    adminConsole: string;
    webmail: string;
    gettingStarted: string;
    overview: string;
  };
  pages: Record<string, PageMessage>;
};

const localeLabels: Record<DocsLocale, string> = {
  en: 'English',
  ko: '한국어',
  ja: '日本語',
  'zh-CN': '简体中文',
};

const copy = {
  en: {
    guide: 'Guide',
    description: 'Transparent product guide for GoGoMail administrators and webmail users.',
    openSource: 'Open source and transparent by default.',
    lastUpdated: 'Last updated',
    copyCode: 'Copy',
    copiedCode: 'Copied',
    overview: 'Overview',
    gettingStarted: 'Getting started',
    adminLead: 'Operate tenants, domains, users, security, delivery, storage, and audit workflows from one console.',
    webmailLead: 'Read, compose, organize, and manage everyday mail with the webmail experience.',
    homeLead: 'A concise, task-first guide for running GoGoMail without hiding how the product works.',
    primary: 'Start with the console',
    secondary: 'Open webmail guide',
    consoleStart: 'Console first steps',
    webmailStart: 'Webmail first steps',
    transparentTitle: 'Transparent operations',
    transparentBody: 'The guide should explain real product behavior, visible limits, and operational tradeoffs instead of hiding them behind marketing language.',
    agentTitle: 'Agent-friendly content system',
    agentBody: 'Every visible label comes from the shared i18n layer so future agents can update content without inventing conflicting terminology.',
    structureTitle: 'Two product areas',
    structureBody: 'The guide starts with two clear paths and can grow deeper under each path as the product expands.',
    loginTitle: 'Sign in and choose the right tenant',
    loginBody: 'Use the admin sign-in flow, confirm the active company, then work from the navigation that matches the task.',
    domainTitle: 'Configure domains before users',
    domainBody: 'Domain setup, DNS verification, DKIM, DMARC, and delivery policy should be documented before user onboarding.',
    auditTitle: 'Keep auditability visible',
    auditBody: 'Security, audit logs, admin activity, and retention settings should be treated as first-class operating workflows.',
    mailboxTitle: 'Start from the mailbox',
    mailboxBody: 'Users should understand the inbox, sent mail, drafts, trash, spam, archive, and folder model before advanced actions.',
    composeTitle: 'Compose with clear requirements',
    composeBody: 'Sending mail requires recipients and either a subject or body, and the guide should surface that validation plainly.',
    settingsTitle: 'Keep personal settings discoverable',
    settingsBody: 'Theme and language controls belong in the user guide because they affect the everyday reading experience.',
    factualTitle: 'Written from the running product',
    factualBody: 'The first version is based on the visible admin console navigation, the webmail mailbox, and the labels already shipped in each frontend. It intentionally avoids undocumented promises.',
    termTitle: 'Terminology source',
    termBody: 'Product names, navigation labels, folder names, compose labels, settings labels, and search labels are imported from the console and webmail i18n files instead of being duplicated in Markdown.',
    docGrowthTitle: 'How the guide grows',
    docGrowthBody: 'Each product area starts with a dense overview and a getting-started path. Deeper pages should be added only when a real screen, route, or user task exists.',
    adminNavTitle: 'Console navigation model',
    adminNavBody: 'The console is organized by operating responsibility, not by implementation detail. The visible groups are the safest way to explain the product to administrators.',
    adminDashboardTitle: 'Dashboard reading order',
    adminDashboardBody: 'The dashboard shows the active company first, then refresh timing, tenant status, mail volume, delivery counts, and user activity. Treat it as a status surface before jumping into configuration.',
    adminResourcesBody: 'Resource pages cover the tenant inventory: companies, domains, users, administrators, tenant health, change history, and onboarding.',
    adminSettingsBody: 'Configuration pages cover company defaults, domain settings, SSO, webhooks, notification templates, signatures, SCIM status, and user-level defaults.',
    adminOperationsBody: 'Operations pages cover delivery and runtime evidence: message trace, mail flow logs, outbox events, delivery attempts, routing rules, routes, relays, queue statistics, backpressure, and system health.',
    adminAccessBody: 'Access control pages cover directory records, aliases, delegations, group membership, and roles. These screens should be documented as permission and identity workflows.',
    adminGovernanceBody: 'Governance means operating rules and evidence management: deciding how the mail service is controlled, proving who changed what, and keeping security or compliance requirements reviewable. These screens include audit logs, administrator activity, alert rules, suppression lists, keys, API access, 2FA, IP access, authentication policy, audit policy, retention, sessions, rate limits, DMARC/SPF, spam filtering, SMTP policy, posture, compliance, and legal holds.',
    adminAnalyticsBody: 'Analytics and storage pages collect operational reports: quota dashboards, storage usage, quota alerts, attachments, drive, seat usage, API usage, push notifications, and printable reports.',
    adminLoginBody: 'Start at the admin login screen, enter the administrator email and password, then confirm that the company selector and page header show the company you intend to operate.',
    adminDomainBody: 'Before adding users, review domains and domain settings. The domain list exposes status, DNS state, quota, and creation date, so it is the practical checkpoint for mail readiness.',
    adminUsersBody: 'The user screen exposes total, active, and suspended counts, list filtering, CSV export, CSV import, and user creation. Document these as bulk administration tasks.',
    adminEvidenceBody: 'When investigating an issue, move from dashboard status to message trace and mail flow logs, then to audit logs or administrator activity if the change history matters.',
    adminReportBody: 'Reports are designed for review and export. The visible report cards include compliance, audit logs, user directory, domain status, storage, and delivery-oriented summaries.',
    webmailShellTitle: 'Application shell',
    webmailShellBody: 'The webmail app opens on mail and provides adjacent app surfaces for calendar, contacts or organization, drive, and settings. The guide should describe those as visible surfaces, not as hidden backend capabilities.',
    webmailFoldersBody: 'The mailbox model exposes inbox, sent, drafts, trash, spam, archive, folders, unread state, no-message state, date grouping, sender, recipient, subject, attachments, and selection.',
    webmailReadingBody: 'The reading pane presents message header fields, recipients, copyable addresses, attachments, reply, reply-all, forward, delete, mark-unread, archive-related actions, and safe rendering behavior.',
    webmailComposeBody: 'The compose experience has a new-message surface, recipient field, subject field, body editor, send state, validation messages, and a formatting toolbar.',
    webmailComposeToolsBody: 'The visible compose toolbar includes bold, italic, underline, bullet list, link insertion, and undo. Recipient selection can be connected to the organization picker where the screen exposes it.',
    webmailSettingsBody: 'Settings include theme and language controls, and the app persists the selected theme locally so the reading experience follows the user choice.',
    webmailLoginBody: 'The login screen asks for email and password. Empty fields show the shipped validation message, and a successful login opens the mail page.',
    webmailMailboxBody: 'Begin by locating the folder list, message list, and reading pane. Use the existing empty and loading states as part of the explanation so users know what normal waiting looks like.',
    webmailSendBody: 'For sending, fill recipients, subject, and body, then use the send action. The product has explicit required-recipient and required-body messages, so the guide states those rules directly.',
    webmailSettingsStartBody: 'Open settings from the webmail shell, then change theme or language. The same guide page should mention light and dark themes because they are first-class user controls.',
    factualLimitTitle: 'Documentation rule',
    factualLimitBody: 'If a behavior is not visible in the frontend, not present in the route list, or not backed by an existing message key, the guide should leave it undocumented until the product exposes it.',
  },
  ko: {
    guide: '가이드',
    description: '관리자와 웹메일 사용자를 위한 투명한 GoGoMail 제품 가이드입니다.',
    openSource: '오픈소스답게 기본값은 투명함입니다.',
    lastUpdated: '마지막 업데이트',
    copyCode: '복사',
    copiedCode: '복사됨',
    overview: '개요',
    gettingStarted: '시작하기',
    adminLead: '테넌트, 도메인, 사용자, 보안, 전송, 저장소, 감사 흐름을 하나의 콘솔에서 운영합니다.',
    webmailLead: '웹메일에서 메일을 읽고, 작성하고, 정리하고, 일상 설정을 관리합니다.',
    homeLead: 'GoGoMail을 운영하는 방법을 숨기지 않고 짧고 명확하게 안내하는 작업 중심 가이드입니다.',
    primary: '콘솔부터 시작',
    secondary: '웹메일 가이드 열기',
    consoleStart: '콘솔 첫 단계',
    webmailStart: '웹메일 첫 단계',
    transparentTitle: '투명한 운영',
    transparentBody: '이 가이드는 마케팅 문구 뒤에 숨기지 않고 실제 제품 동작, 보이는 한계, 운영상 선택지를 설명해야 합니다.',
    agentTitle: '에이전트 친화 콘텐츠 시스템',
    agentBody: '보이는 모든 라벨은 공유 다국어 레이어에서 나오므로, 이후 에이전트가 용어를 새로 지어내지 않고 문서를 갱신할 수 있습니다.',
    structureTitle: '두 제품 영역',
    structureBody: '가이드는 두 경로에서 시작하고, 제품이 커질수록 각 경로 아래로 깊이를 늘릴 수 있습니다.',
    loginTitle: '로그인하고 올바른 테넌트 확인하기',
    loginBody: '관리자 로그인 흐름을 사용하고, 활성 회사를 확인한 뒤, 작업에 맞는 네비게이션에서 시작합니다.',
    domainTitle: '사용자보다 도메인을 먼저 설정하기',
    domainBody: '사용자 온보딩 전에 도메인 설정, DNS 검증, DKIM, DMARC, 전송 정책을 먼저 문서화해야 합니다.',
    auditTitle: '감사 가능성을 계속 보이게 하기',
    auditBody: '보안, 감사 로그, 관리자 활동, 보존 설정은 부가 기능이 아니라 핵심 운영 흐름으로 다뤄야 합니다.',
    mailboxTitle: '메일함에서 시작하기',
    mailboxBody: '고급 동작 전에 받은편지함, 보낸편지함, 임시보관함, 휴지통, 스팸, 보관함, 폴더 모델을 먼저 이해하게 합니다.',
    composeTitle: '메일 작성 조건을 명확히 하기',
    composeBody: '메일 전송에는 받는 사람과 제목 또는 본문이 필요하며, 가이드는 이 검증 규칙을 분명히 보여줘야 합니다.',
    settingsTitle: '개인 설정을 쉽게 찾게 하기',
    settingsBody: '테마와 언어 설정은 매일 읽는 경험에 영향을 주므로 사용자 가이드 안에서 분명히 다룹니다.',
    factualTitle: '실행 중인 제품을 기준으로 작성',
    factualBody: '첫 버전은 실제 관리자 콘솔 네비게이션, 웹메일 메일함, 각 프론트엔드에 이미 포함된 라벨을 기준으로 작성합니다. 문서화되지 않은 약속은 의도적으로 넣지 않습니다.',
    termTitle: '용어의 출처',
    termBody: '제품명, 네비게이션 라벨, 폴더명, 작성 라벨, 설정 라벨, 검색 라벨은 마크다운에 복제하지 않고 콘솔과 웹메일 다국어 파일에서 가져옵니다.',
    docGrowthTitle: '가이드 확장 방식',
    docGrowthBody: '각 제품 영역은 빡빡한 개요와 시작 경로에서 출발합니다. 더 깊은 페이지는 실제 화면, 라우트, 사용자 작업이 있을 때만 추가합니다.',
    adminNavTitle: '콘솔 네비게이션 모델',
    adminNavBody: '콘솔은 구현 세부가 아니라 운영 책임을 기준으로 묶여 있습니다. 화면에 보이는 그룹이 관리자에게 제품을 설명하는 가장 안전한 기준입니다.',
    adminDashboardTitle: '대시보드 읽는 순서',
    adminDashboardBody: '대시보드는 활성 회사를 먼저 보여주고, 새로고침 시점, 테넌트 상태, 메일 볼륨, 전송 카운트, 사용자 활동을 이어서 보여줍니다. 설정으로 뛰어들기 전에 상태를 읽는 화면으로 다룹니다.',
    adminResourcesBody: '리소스 화면은 테넌트 인벤토리입니다. 회사, 도메인, 사용자, 관리자, 테넌트 상태, 변경 이력, 온보딩을 한 흐름으로 다룹니다.',
    adminSettingsBody: '설정 화면은 회사 기본값, 도메인 설정, SSO, 웹훅, 알림 템플릿, 전역 서명, SCIM 상태, 사용자 기본 설정을 다룹니다.',
    adminOperationsBody: '운영 화면은 전송과 런타임 근거를 다룹니다. 메시지 추적, 메일 흐름 로그, 아웃박스 이벤트, 전송 시도, 라우팅 규칙, 라우트, 릴레이, 큐, 백프레셔, 시스템 상태가 여기에 속합니다.',
    adminAccessBody: '접근 제어 화면은 디렉터리, 별칭, 위임, 그룹 멤버십, 역할을 다룹니다. 이 영역은 권한과 신원 관리 흐름으로 설명합니다.',
    adminGovernanceBody: '거버넌스는 회사가 메일 서비스를 어떤 규칙으로 운영할지 정하고, 누가 무엇을 바꿨는지 증거를 남기며, 보안이나 규정 준수 기준을 나중에 확인할 수 있게 관리하는 영역입니다. 이 화면에는 감사 로그, 관리자 활동, 알림 규칙, 차단 목록, 키, API 접근, 2FA, IP 접근, 인증 정책, 감사 정책, 보존, 세션, 발신 제한, DMARC/SPF, 스팸 필터, SMTP 정책, 보안 현황, 규정 준수, 법적 보관이 포함됩니다.',
    adminAnalyticsBody: '분석과 저장 화면은 운영 보고서를 모읍니다. 할당량 대시보드, 스토리지 사용량, 할당량 알림, 첨부파일, 드라이브, 좌석 사용량, API 사용, 푸시 알림, 인쇄 가능한 보고서가 여기에 속합니다.',
    adminLoginBody: '관리자 로그인 화면에서 관리자 이메일과 비밀번호를 입력한 뒤, 회사 선택 영역과 페이지 헤더가 운영하려는 회사를 가리키는지 확인합니다.',
    adminDomainBody: '사용자를 추가하기 전에 도메인과 도메인 설정을 먼저 확인합니다. 도메인 목록은 상태, DNS 상태, 할당량, 생성일을 보여주므로 메일 준비 상태를 확인하는 실무 체크포인트입니다.',
    adminUsersBody: '사용자 화면은 전체, 활성, 정지 카운트, 목록 필터, CSV 내보내기, CSV 가져오기, 사용자 생성을 보여줍니다. 이 흐름은 대량 관리 작업으로 문서화합니다.',
    adminEvidenceBody: '문제를 조사할 때는 대시보드 상태에서 메시지 추적과 메일 흐름 로그로 이동하고, 변경 원인이 필요하면 감사 로그 또는 관리자 활동으로 이어갑니다.',
    adminReportBody: '보고서는 검토와 내보내기를 위한 화면입니다. 보이는 보고서 카드는 규정 준수, 감사 로그, 사용자 디렉터리, 도메인 상태, 저장소, 전송 관련 요약을 포함합니다.',
    webmailShellTitle: '애플리케이션 셸',
    webmailShellBody: '웹메일은 메일 화면에서 시작하고 캘린더, 연락처 또는 조직도, 드라이브, 설정으로 이동할 수 있는 앱 표면을 제공합니다. 이 가이드는 이를 숨은 백엔드 기능이 아니라 화면에 보이는 표면으로 설명합니다.',
    webmailFoldersBody: '메일함 모델은 받은편지함, 보낸편지함, 임시보관함, 휴지통, 스팸, 보관함, 폴더, 읽지 않음, 메시지 없음, 날짜 그룹, 보낸 사람, 받는 사람, 제목, 첨부파일, 선택 상태를 보여줍니다.',
    webmailReadingBody: '읽기 패널은 메시지 헤더, 수신자, 복사 가능한 주소, 첨부파일, 답장, 전체 답장, 전달, 삭제, 읽지 않음 표시, 보관 관련 동작, 안전한 본문 렌더링을 다룹니다.',
    webmailComposeBody: '메일 작성 흐름은 새 메시지 화면, 받는 사람 필드, 제목 필드, 본문 편집기, 전송 상태, 검증 메시지, 서식 도구 모음을 포함합니다.',
    webmailComposeToolsBody: '보이는 작성 도구 모음에는 굵게, 기울임, 밑줄, 글머리 목록, 링크 삽입, 실행 취소가 있습니다. 화면이 제공하는 곳에서는 조직도 선택과 받는 사람 입력이 연결됩니다.',
    webmailSettingsBody: '설정에는 테마와 언어 제어가 있고, 앱은 선택한 테마를 로컬에 저장해 읽기 경험이 사용자 선택을 따르게 합니다.',
    webmailLoginBody: '로그인 화면은 이메일과 비밀번호를 요구합니다. 빈 필드는 제품에 포함된 검증 메시지를 보여주고, 로그인에 성공하면 메일 화면으로 이동합니다.',
    webmailMailboxBody: '먼저 폴더 목록, 메시지 목록, 읽기 패널을 찾습니다. 로딩과 빈 상태도 정상 흐름으로 설명해 사용자가 기다림과 데이터 없음 상태를 구분하게 합니다.',
    webmailSendBody: '전송하려면 받는 사람, 제목, 본문을 채우고 전송 동작을 사용합니다. 제품에는 받는 사람 필수와 본문 필수 메시지가 있으므로, 가이드에서도 이 규칙을 직접 밝힙니다.',
    webmailSettingsStartBody: '웹메일 셸에서 설정을 열고 테마 또는 언어를 변경합니다. 라이트 테마와 다크 테마는 사용자 제어이므로 같은 시작 페이지에서 함께 설명합니다.',
    factualLimitTitle: '문서화 규칙',
    factualLimitBody: '프론트엔드에서 보이지 않거나, 라우트 목록에 없거나, 기존 메시지 키로 뒷받침되지 않는 동작은 제품이 노출할 때까지 문서화하지 않습니다.',
  },
  ja: {
    guide: 'ガイド',
    description: '管理者とウェブメール利用者のための透明性の高い GoGoMail 製品ガイドです。',
    openSource: 'オープンソースらしく、既定は透明性です。',
    lastUpdated: '最終更新',
    copyCode: 'コピー',
    copiedCode: 'コピー済み',
    overview: '概要',
    gettingStarted: 'はじめに',
    adminLead: 'テナント、ドメイン、ユーザー、セキュリティ、配信、ストレージ、監査を 1 つのコンソールで運用します。',
    webmailLead: 'ウェブメールでメールの閲覧、作成、整理、日常設定を管理します。',
    homeLead: 'GoGoMail の運用方法を隠さず、簡潔に案内するタスク中心のガイドです。',
    primary: 'コンソールから始める',
    secondary: 'ウェブメールガイドを開く',
    consoleStart: 'コンソールの最初の手順',
    webmailStart: 'ウェブメールの最初の手順',
    transparentTitle: '透明な運用',
    transparentBody: 'このガイドはマーケティング表現ではなく、実際の動作、見える制約、運用上の選択肢を説明します。',
    agentTitle: 'エージェントに優しいコンテンツ体系',
    agentBody: '表示ラベルは共有 i18n レイヤーから取得するため、将来のエージェントが用語を増やさずに文書を更新できます。',
    structureTitle: '2 つの製品領域',
    structureBody: 'ガイドは 2 つの道筋から始まり、製品の拡張に合わせて各領域を深くできます。',
    loginTitle: 'サインインして正しいテナントを確認する',
    loginBody: '管理者のサインイン後、アクティブな会社を確認し、作業に合うナビゲーションから始めます。',
    domainTitle: 'ユーザーより先にドメインを設定する',
    domainBody: 'ユーザー招待の前に、ドメイン設定、DNS 検証、DKIM、DMARC、配信ポリシーを文書化します。',
    auditTitle: '監査可能性を見える状態にする',
    auditBody: 'セキュリティ、監査ログ、管理者アクティビティ、保持設定は中心的な運用フローとして扱います。',
    mailboxTitle: 'メールボックスから始める',
    mailboxBody: '高度な操作の前に、受信トレイ、送信済み、下書き、ゴミ箱、迷惑メール、アーカイブ、フォルダーを理解します。',
    composeTitle: '作成条件を明確にする',
    composeBody: '送信には宛先と件名または本文が必要です。ガイドではこの検証ルールを明確に示します。',
    settingsTitle: '個人設定を見つけやすくする',
    settingsBody: 'テーマと言語は日々の閲覧体験に影響するため、ユーザーガイドで明確に扱います。',
    factualTitle: '稼働中の製品を基準に書く',
    factualBody: '初版は実際の管理コンソールナビゲーション、ウェブメールのメールボックス、各フロントエンドに含まれるラベルを基準にします。未文書化の約束は入れません。',
    termTitle: '用語の出典',
    termBody: '製品名、ナビゲーション、フォルダー、作成、設定、検索のラベルは Markdown に複製せず、コンソールとウェブメールの i18n ファイルから取得します。',
    docGrowthTitle: 'ガイドの拡張方法',
    docGrowthBody: '各製品領域は密度の高い概要とはじめにから始めます。より深いページは、実画面、ルート、ユーザータスクが存在するときだけ追加します。',
    adminNavTitle: 'コンソールのナビゲーションモデル',
    adminNavBody: 'コンソールは実装詳細ではなく運用責任で整理されています。表示されているグループが管理者向け説明の基準です。',
    adminDashboardTitle: 'ダッシュボードの読み方',
    adminDashboardBody: 'ダッシュボードはアクティブな会社、更新タイミング、テナント状態、メール量、配信数、ユーザー活動を順に示します。設定前の状態確認画面として扱います。',
    adminResourcesBody: 'リソース画面はテナントのインベントリです。会社、ドメイン、ユーザー、管理者、テナント状態、変更履歴、オンボーディングを一連の流れとして扱います。',
    adminSettingsBody: '設定画面は会社既定値、ドメイン設定、SSO、Webhook、通知テンプレート、グローバル署名、SCIM 状態、ユーザー既定値を扱います。',
    adminOperationsBody: '運用画面は配信とランタイムの根拠を扱います。メッセージ追跡、メールフローログ、アウトボックス、配信試行、ルーティングルール、ルート、リレー、キュー、バックプレッシャー、システム状態が含まれます。',
    adminAccessBody: 'アクセス制御画面はディレクトリ、エイリアス、委任、グループメンバーシップ、ロールを扱います。権限とアイデンティティの流れとして説明します。',
    adminGovernanceBody: 'ガバナンスとは、メールサービスをどの規則で運用するかを決め、誰が何を変更したかの証跡を残し、セキュリティやコンプライアンス要件を後から確認できるように管理する領域です。監査ログ、管理者活動、アラートルール、抑止リスト、キー、API アクセス、2FA、IP アクセス、認証ポリシー、監査ポリシー、保持、セッション、レート制限、DMARC/SPF、スパムフィルター、SMTP ポリシー、セキュリティ態勢、コンプライアンス、法的保全を含みます。',
    adminAnalyticsBody: '分析とストレージ画面は運用レポートを集約します。クォータダッシュボード、ストレージ使用量、クォータ通知、添付、ドライブ、シート使用量、API 使用、プッシュ通知、印刷可能なレポートが含まれます。',
    adminLoginBody: '管理者ログイン画面でメールアドレスとパスワードを入力し、会社セレクターとページヘッダーが操作対象の会社を示していることを確認します。',
    adminDomainBody: 'ユーザー追加の前にドメインとドメイン設定を確認します。ドメイン一覧は状態、DNS、クォータ、作成日を表示するため、メール準備の実務チェックポイントです。',
    adminUsersBody: 'ユーザー画面は合計、有効、停止数、リストフィルター、CSV エクスポート、CSV インポート、ユーザー作成を表示します。これは一括管理タスクとして文書化します。',
    adminEvidenceBody: '問題調査では、ダッシュボード状態からメッセージ追跡とメールフローログへ進み、変更原因が必要なら監査ログまたは管理者活動へ進みます。',
    adminReportBody: 'レポートはレビューとエクスポートのための画面です。表示されるカードにはコンプライアンス、監査ログ、ユーザーディレクトリ、ドメイン状態、ストレージ、配信関連の要約が含まれます。',
    webmailShellTitle: 'アプリケーションシェル',
    webmailShellBody: 'ウェブメールはメール画面から始まり、カレンダー、連絡先または組織図、ドライブ、設定へ移動できる面を提供します。隠れたバックエンド機能ではなく、表示される面として説明します。',
    webmailFoldersBody: 'メールボックスモデルは受信トレイ、送信済み、下書き、ゴミ箱、迷惑メール、アーカイブ、フォルダー、未読、メッセージなし、日付グループ、差出人、宛先、件名、添付、選択状態を表示します。',
    webmailReadingBody: '閲覧ペインはメッセージヘッダー、宛先、コピー可能なアドレス、添付、返信、全員に返信、転送、削除、未読化、アーカイブ関連操作、安全な本文レンダリングを扱います。',
    webmailComposeBody: '作成フローは新規メッセージ、宛先、件名、本文エディター、送信状態、検証メッセージ、書式ツールバーを含みます。',
    webmailComposeToolsBody: '表示される作成ツールバーには太字、斜体、下線、箇条書き、リンク挿入、元に戻すがあります。画面が提供する場所では組織図選択と宛先入力がつながります。',
    webmailSettingsBody: '設定にはテーマと言語の操作があり、アプリは選択したテーマをローカルに保存して閲覧体験に反映します。',
    webmailLoginBody: 'ログイン画面はメールアドレスとパスワードを求めます。空欄では製品内の検証メッセージを表示し、成功するとメール画面へ移動します。',
    webmailMailboxBody: 'まずフォルダー一覧、メッセージ一覧、閲覧ペインを見つけます。読み込み状態と空状態も通常フローとして説明し、待機とデータなしを区別できるようにします。',
    webmailSendBody: '送信するには宛先、件名、本文を入力し、送信操作を使います。製品には宛先必須と本文必須のメッセージがあるため、ガイドでも明記します。',
    webmailSettingsStartBody: 'ウェブメールシェルから設定を開き、テーマまたは言語を変更します。ライトテーマとダークテーマはユーザー操作なので、同じ開始ページで扱います。',
    factualLimitTitle: '文書化ルール',
    factualLimitBody: 'フロントエンドに見えない、ルート一覧にない、既存メッセージキーで裏付けられない動作は、製品が表示するまで文書化しません。',
  },
  'zh-CN': {
    guide: '指南',
    description: '面向管理员和 Webmail 用户的透明 GoGoMail 产品指南。',
    openSource: '作为开源产品，默认保持透明。',
    lastUpdated: '最后更新',
    copyCode: '复制',
    copiedCode: '已复制',
    overview: '概览',
    gettingStarted: '开始使用',
    adminLead: '在一个控制台中运营租户、域名、用户、安全、投递、存储和审计流程。',
    webmailLead: '在 Webmail 中阅读、撰写、整理邮件，并管理日常设置。',
    homeLead: '这是一份以任务为中心的简明指南，透明说明如何运营 GoGoMail。',
    primary: '从控制台开始',
    secondary: '打开 Webmail 指南',
    consoleStart: '控制台第一步',
    webmailStart: 'Webmail 第一步',
    transparentTitle: '透明运营',
    transparentBody: '本指南应说明真实产品行为、可见限制和运营取舍，而不是用营销语言掩盖它们。',
    agentTitle: '面向代理的内容系统',
    agentBody: '所有可见标签都来自共享 i18n 层，因此后续代理可以更新内容而不会创造冲突术语。',
    structureTitle: '两个产品区域',
    structureBody: '指南从两条清晰路径开始，并可随着产品扩展在每条路径下继续加深。',
    loginTitle: '登录并确认正确租户',
    loginBody: '使用管理员登录流程，确认当前公司，然后从匹配任务的导航开始。',
    domainTitle: '先配置域名，再配置用户',
    domainBody: '在用户入驻前，应先记录域名设置、DNS 验证、DKIM、DMARC 和投递策略。',
    auditTitle: '保持审计能力可见',
    auditBody: '安全、审计日志、管理员活动和保留设置应作为一等运营流程处理。',
    mailboxTitle: '从邮箱开始',
    mailboxBody: '在高级操作前，用户应先理解收件箱、已发送、草稿、垃圾箱、垃圾邮件、归档和文件夹模型。',
    composeTitle: '明确撰写要求',
    composeBody: '发送邮件需要收件人，以及主题或正文；指南应清楚展示这条校验规则。',
    settingsTitle: '让个人设置易于发现',
    settingsBody: '主题和语言会影响日常阅读体验，因此应在用户指南中清楚说明。',
    factualTitle: '以正在运行的产品为准',
    factualBody: '第一版基于实际管理员控制台导航、Webmail 邮箱界面，以及各前端已经发布的标签。未文档化的承诺不会写入指南。',
    termTitle: '术语来源',
    termBody: '产品名、导航标签、文件夹名、撰写标签、设置标签和搜索标签不复制到 Markdown，而是从控制台和 Webmail 的 i18n 文件导入。',
    docGrowthTitle: '指南扩展方式',
    docGrowthBody: '每个产品区域从高密度概览和开始路径起步。只有当真实屏幕、路由或用户任务存在时，才添加更深页面。',
    adminNavTitle: '控制台导航模型',
    adminNavBody: '控制台按运营职责组织，而不是按实现细节组织。可见分组是向管理员解释产品的可靠依据。',
    adminDashboardTitle: '仪表板阅读顺序',
    adminDashboardBody: '仪表板先显示当前公司，然后显示刷新时间、租户状态、邮件量、投递计数和用户活动。进入配置前，应先把它作为状态页面阅读。',
    adminResourcesBody: '资源页面覆盖租户清单：公司、域名、用户、管理员、租户状态、变更历史和入驻。',
    adminSettingsBody: '设置页面覆盖公司默认值、域名设置、SSO、Webhook、通知模板、全局签名、SCIM 状态和用户级默认值。',
    adminOperationsBody: '运营页面覆盖投递和运行证据：消息追踪、邮件流日志、发件箱事件、投递尝试、路由规则、路由、 Relay、队列、背压和系统健康。',
    adminAccessBody: '访问控制页面覆盖目录、别名、委派、组成员和角色。应作为权限与身份工作流来说明。',
    adminGovernanceBody: '治理指运营规则和证据管理：决定邮件服务按什么规则运行，记录谁更改了什么，并让安全或合规要求之后仍可审查。相关页面包括审计日志、管理员活动、告警规则、阻止列表、密钥、API 访问、2FA、IP 访问、认证策略、审计策略、保留、会话、发送限制、DMARC/SPF、垃圾邮件过滤、SMTP 策略、安全态势、合规和法律保留。',
    adminAnalyticsBody: '分析与存储页面汇集运营报告：配额仪表板、存储使用量、配额告警、附件、Drive、席位使用量、API 使用、推送通知和可打印报告。',
    adminLoginBody: '在管理员登录页输入管理员邮箱和密码，然后确认公司选择器与页面标题指向要运营的公司。',
    adminDomainBody: '添加用户前先检查域名和域名设置。域名列表显示状态、DNS 状态、配额和创建日期，因此是邮件准备情况的实际检查点。',
    adminUsersBody: '用户页面显示总数、活跃数、暂停数、列表筛选、CSV 导出、CSV 导入和用户创建。应作为批量管理任务记录。',
    adminEvidenceBody: '调查问题时，从仪表板状态进入消息追踪和邮件流日志；如果需要变更来源，再进入审计日志或管理员活动。',
    adminReportBody: '报告页面用于审阅和导出。可见报告卡包括合规、审计日志、用户目录、域名状态、存储和投递相关摘要。',
    webmailShellTitle: '应用外壳',
    webmailShellBody: 'Webmail 从邮件页面开始，并提供可进入日历、联系人或组织、Drive 和设置的应用区域。指南应把它们描述为可见界面，而不是隐藏后端能力。',
    webmailFoldersBody: '邮箱模型显示收件箱、已发送、草稿、垃圾箱、垃圾邮件、归档、文件夹、未读、无邮件、日期分组、发件人、收件人、主题、附件和选择状态。',
    webmailReadingBody: '阅读窗格覆盖邮件头、收件人、可复制地址、附件、回复、全部回复、转发、删除、标为未读、归档相关操作和安全正文渲染。',
    webmailComposeBody: '撰写流程包括新邮件界面、收件人字段、主题字段、正文编辑器、发送状态、校验消息和格式工具栏。',
    webmailComposeToolsBody: '可见撰写工具栏包含加粗、斜体、下划线、项目符号列表、插入链接和撤销。界面提供时，组织选择器会连接到收件人输入。',
    webmailSettingsBody: '设置包含主题和语言控制，应用会在本地保存所选主题，使阅读体验跟随用户选择。',
    webmailLoginBody: '登录页要求邮箱和密码。空字段显示产品内置校验消息，登录成功后进入邮件页面。',
    webmailMailboxBody: '先找到文件夹列表、邮件列表和阅读窗格。加载状态和空状态也应作为正常流程说明，帮助用户区分等待和无数据。',
    webmailSendBody: '发送时填写收件人、主题和正文，然后使用发送操作。产品已有收件人必填和正文必填消息，因此指南直接说明这些规则。',
    webmailSettingsStartBody: '从 Webmail 外壳打开设置，然后更改主题或语言。浅色主题和深色主题是用户控制项，因此在同一开始页面说明。',
    factualLimitTitle: '文档规则',
    factualLimitBody: '如果行为在前端不可见、不在路由列表中，或没有现有消息键支撑，应在产品暴露之前不写入文档。',
  },
} satisfies Record<DocsLocale, Record<string, string>>;

function link(locale: DocsLocale, path: string) {
  return locale === 'en' ? path : `/${locale}${path}`;
}

const featureCopy = {
  en: {
    rolesTitle: 'Administrator roles',
    rolesLead: 'Use this page to decide who should operate the console and which screens they should touch.',
    roleSystem: 'System administrator: visible in the user role list as the role for all-company administration. Use it for cross-company tenancy, global troubleshooting, and administrator account governance.',
    roleCompany: 'Company administrator: visible in the user role list as the role that grants console access for one company. Use it for company users, domains, policies, reports, and day-to-day operations.',
    roleMailOnly: 'User: visible as the mail-only role. This account belongs in webmail unless it is promoted to an administrator role.',
    roleAdminAccount: 'Administrator accounts also expose system administrator, administrator, and read-only options. Treat these as console account privileges, separate from ordinary mailbox use.',
    roleDomain: 'Domain administrator is not exposed as a first-class account role in the current console. Domain work is represented through domain pages, delegation, group membership, and domain-level policy screens.',
    resourcesTitle: 'Resources',
    resourcesLead: 'These screens answer what exists in the tenant and whether it is ready to be used.',
    settingsTitle: 'Console settings',
    settingsLead: 'These screens change company, domain, organization, and user defaults.',
    operationsTitle: 'Operations and delivery',
    operationsLead: 'These screens are for mail movement, runtime evidence, retry decisions, and delivery health.',
    accessTitle: 'Access control',
    accessLead: 'These screens explain who can receive, send, delegate, or administer access inside a company.',
    governanceTitle: 'Governance and security',
    governanceLead: 'Governance here means operating rules and evidence management: security controls, change history, policy proof, and compliance review.',
    analyticsTitle: 'Analytics and storage',
    analyticsLead: 'These screens turn usage, storage, attachment, drive, API, push, and report data into reviewable operations.',
    externalIntegrationTitle: 'External integration API',
    externalIntegrationLead: 'Connect external services to GoGoMail with domain API keys, explicit mailbox user binding, scoped mail permissions, and API usage metering.',
    webmailMailTitle: 'Mail reading and organization',
    webmailMailLead: 'These features describe the mailbox, message list, reading pane, and message actions.',
    webmailComposeTitle: 'Compose and sending',
    webmailComposeLead: 'These features describe composing, validation, formatting, recipients, and send behavior.',
    webmailSettingsTitle: 'Webmail settings',
    webmailSettingsLead: 'These settings are user-facing controls that change the daily webmail experience.',
    webmailAppsTitle: 'Calendar, contacts, organization, and drive',
    webmailAppsLead: 'These app surfaces are visible in the webmail shell and should be documented as user workflows only where the UI exposes them.',
    webmailShortcutsTitle: 'Keyboard shortcuts',
    webmailShortcutsLead: 'The shortcut help groups are visible in the frontend and should be documented as command families.',
    startAdminLead: 'Sign in, confirm the company, read the dashboard, then move to the feature page that matches the task.',
    startWebmailLead: 'Sign in, open the mailbox, read or compose mail, then adjust personal settings only when needed.',
    sourceNote: 'Documented from the current frontend route list, message files, and visible controls.',
  },
  ko: {
    rolesTitle: '관리자 역할',
    rolesLead: '콘솔을 누가 운영해야 하는지, 어떤 화면까지 맡겨야 하는지 판단하는 기준입니다.',
    roleSystem: '시스템 관리자: 사용자 역할 목록에서 전체 회사 운영 역할로 보입니다. 회사 전체 테넌시, 전역 문제 조사, 관리자 계정의 운영 규칙과 증적 관리에 사용합니다.',
    roleCompany: '회사 관리자: 사용자 역할 목록에서 특정 회사의 콘솔 접근 역할로 보입니다. 회사 사용자, 도메인, 정책, 보고서, 일상 운영에 사용합니다.',
    roleMailOnly: '사용자: 메일만 사용하는 역할로 보입니다. 관리자 역할로 승격하지 않는 한 웹메일 사용 계정으로 다룹니다.',
    roleAdminAccount: '관리자 계정 화면은 시스템 관리자, 관리자, 읽기 전용 옵션도 노출합니다. 이는 일반 메일함 사용과 분리된 콘솔 계정 권한으로 설명합니다.',
    roleDomain: '현재 콘솔에는 도메인 관리자가 1급 계정 역할로 노출되지 않습니다. 도메인 단위 운영은 도메인 화면, 위임, 그룹 멤버십, 도메인 정책 화면으로 표현됩니다.',
    resourcesTitle: '리소스',
    resourcesLead: '테넌트 안에 무엇이 있고 실제 사용 준비가 되었는지 확인하는 화면입니다.',
    settingsTitle: '콘솔 설정',
    settingsLead: '회사, 도메인, 조직, 사용자 기본값을 바꾸는 화면입니다.',
    operationsTitle: '운영과 전송',
    operationsLead: '메일 이동, 런타임 근거, 재시도 판단, 전송 상태를 다루는 화면입니다.',
    accessTitle: '접근 제어',
    accessLead: '회사 안에서 누가 받고, 보내고, 위임하고, 접근을 관리할 수 있는지 설명하는 화면입니다.',
    governanceTitle: '거버넌스와 보안',
    governanceLead: '여기서 거버넌스는 운영 규칙과 증적 관리를 뜻합니다. 보안 설정, 변경 이력, 정책 근거, 규정 준수 검토를 한곳에서 확인하게 해주는 화면입니다.',
    analyticsTitle: '분석과 저장',
    analyticsLead: '사용량, 저장소, 첨부파일, 드라이브, API, 푸시, 보고서 데이터를 검토 가능한 운영 자료로 바꾸는 화면입니다.',
    externalIntegrationTitle: '외부 연동 API',
    externalIntegrationLead: '도메인 API 키, 명시적인 메일함 사용자 지정, 메일 권한 범위, API 사용량 미터링을 기준으로 외부 시스템과 GoGoMail을 연결합니다.',
    webmailMailTitle: '메일 읽기와 정리',
    webmailMailLead: '메일함, 메시지 목록, 읽기 패널, 메시지 작업을 설명하는 기능입니다.',
    webmailComposeTitle: '작성과 전송',
    webmailComposeLead: '작성, 검증, 서식, 수신자, 전송 동작을 설명하는 기능입니다.',
    webmailSettingsTitle: '웹메일 설정',
    webmailSettingsLead: '매일의 웹메일 경험을 바꾸는 사용자 제어입니다.',
    webmailAppsTitle: '캘린더, 연락처, 조직도, 드라이브',
    webmailAppsLead: '웹메일 셸에서 보이는 앱 표면이며, UI가 드러내는 사용자 흐름만 문서화합니다.',
    webmailShortcutsTitle: '키보드 단축키',
    webmailShortcutsLead: '프론트엔드에 노출된 단축키 도움말 그룹을 명령 묶음으로 설명합니다.',
    startAdminLead: '로그인하고 회사를 확인한 뒤 대시보드를 읽고, 작업에 맞는 기능 상세 페이지로 이동합니다.',
    startWebmailLead: '로그인하고 메일함을 연 뒤 메일을 읽거나 작성하고, 필요할 때 개인 설정을 조정합니다.',
    sourceNote: '현재 프론트엔드 라우트 목록, 메시지 파일, 화면에 보이는 컨트롤을 기준으로 문서화했습니다.',
  },
  ja: {
    rolesTitle: '管理者ロール',
    rolesLead: '誰がコンソールを運用し、どの画面まで任せるべきかを判断する基準です。',
    roleSystem: 'システム管理者: ユーザーロール一覧では全会社を管理するロールとして表示されます。全社テナンシー、横断調査、管理者アカウント統制に使います。',
    roleCompany: '会社管理者: ユーザーロール一覧では 1 社のコンソールアクセスを持つロールとして表示されます。ユーザー、ドメイン、ポリシー、レポート、日常運用に使います。',
    roleMailOnly: 'ユーザー: メールのみのロールとして表示されます。管理者に昇格しない限りウェブメール利用者として扱います。',
    roleAdminAccount: '管理者アカウント画面にはシステム管理者、管理者、読み取り専用も表示されます。通常のメールボックス利用とは別のコンソール権限です。',
    roleDomain: '現在のコンソールでは、ドメイン管理者は第一級のアカウントロールとして表示されません。ドメイン単位の運用はドメイン画面、委任、グループメンバーシップ、ドメインポリシーで表現されます。',
    resourcesTitle: 'リソース',
    resourcesLead: 'テナント内に何が存在し、利用準備ができているかを確認する画面です。',
    settingsTitle: 'コンソール設定',
    settingsLead: '会社、ドメイン、組織、ユーザーの既定値を変更する画面です。',
    operationsTitle: '運用と配信',
    operationsLead: 'メール移動、実行時の根拠、再試行判断、配信状態を扱う画面です。',
    accessTitle: 'アクセス制御',
    accessLead: '会社内で誰が受信、送信、委任、アクセス管理できるかを説明する画面です。',
    governanceTitle: 'ガバナンスとセキュリティ',
    governanceLead: 'ここでのガバナンスは、運用ルールと証跡管理を意味します。セキュリティ設定、変更履歴、ポリシー根拠、コンプライアンスレビューを確認する画面です。',
    analyticsTitle: '分析とストレージ',
    analyticsLead: '使用量、ストレージ、添付、ドライブ、API、プッシュ、レポートをレビュー可能な運用資料にします。',
    externalIntegrationTitle: '外部連携 API',
    externalIntegrationLead: 'ドメイン API キー、明示的なメールボックスユーザー指定、メール権限スコープ、API 使用量メータリングを基準に外部システムと GoGoMail を接続します。',
    webmailMailTitle: 'メール閲覧と整理',
    webmailMailLead: 'メールボックス、メッセージ一覧、閲覧ペイン、メッセージ操作を説明します。',
    webmailComposeTitle: '作成と送信',
    webmailComposeLead: '作成、検証、書式、宛先、送信動作を説明します。',
    webmailSettingsTitle: 'ウェブメール設定',
    webmailSettingsLead: '日々のウェブメール体験を変えるユーザー向け設定です。',
    webmailAppsTitle: 'カレンダー、連絡先、組織図、ドライブ',
    webmailAppsLead: 'ウェブメールシェルに表示されるアプリ面であり、UI が示すユーザーフローだけを文書化します。',
    webmailShortcutsTitle: 'キーボードショートカット',
    webmailShortcutsLead: 'フロントエンドに表示されるショートカットヘルプをコマンド群として説明します。',
    startAdminLead: 'サインインし、会社を確認し、ダッシュボードを読んでから、作業に合う機能詳細へ進みます。',
    startWebmailLead: 'サインインしてメールボックスを開き、メールを読み書きし、必要に応じて個人設定を調整します。',
    sourceNote: '現在のフロントエンドルート、メッセージファイル、画面上のコントロールを基準に文書化しています。',
  },
  'zh-CN': {
    rolesTitle: '管理员角色',
    rolesLead: '用于判断谁应运营控制台，以及应负责哪些屏幕。',
    roleSystem: '系统管理员：在用户角色列表中显示为跨公司管理角色。用于全公司租户、全局排障以及管理员账号的运营规则和证据管理。',
    roleCompany: '公司管理员：在用户角色列表中显示为某一公司的控制台访问角色。用于公司用户、域名、策略、报告和日常运营。',
    roleMailOnly: '用户：显示为仅邮件角色。除非提升为管理员，否则应作为 Webmail 用户处理。',
    roleAdminAccount: '管理员账号页面还显示系统管理员、管理员和只读选项。它们是控制台账号权限，不等同于普通邮箱使用。',
    roleDomain: '当前控制台没有把域名管理员暴露为一级账号角色。域名工作通过域名页面、委派、组成员和域名级策略页面表达。',
    resourcesTitle: '资源',
    resourcesLead: '这些页面用于确认租户中存在什么，以及是否已经准备好使用。',
    settingsTitle: '控制台设置',
    settingsLead: '这些页面改变公司、域名、组织和用户默认值。',
    operationsTitle: '运营与投递',
    operationsLead: '这些页面处理邮件流转、运行证据、重试判断和投递健康。',
    accessTitle: '访问控制',
    accessLead: '这些页面说明公司内谁可以接收、发送、委派或管理访问。',
    governanceTitle: '治理与安全',
    governanceLead: '这里的治理指运营规则和证据管理：查看安全设置、变更历史、策略依据和合规审查。',
    analyticsTitle: '分析与存储',
    analyticsLead: '这些页面把使用量、存储、附件、Drive、API、推送和报告数据变成可审阅的运营资料。',
    externalIntegrationTitle: '外部集成 API',
    externalIntegrationLead: '通过域名 API 密钥、明确的邮箱用户绑定、邮件权限范围和 API 使用量计量，把外部系统连接到 GoGoMail。',
    webmailMailTitle: '邮件阅读与整理',
    webmailMailLead: '这些功能说明邮箱、邮件列表、阅读窗格和邮件操作。',
    webmailComposeTitle: '撰写与发送',
    webmailComposeLead: '这些功能说明撰写、校验、格式、收件人和发送行为。',
    webmailSettingsTitle: 'Webmail 设置',
    webmailSettingsLead: '这些设置是改变日常 Webmail 体验的用户控制项。',
    webmailAppsTitle: '日历、联系人、组织和 Drive',
    webmailAppsLead: '这些是 Webmail 外壳中可见的应用区域，只记录 UI 暴露的用户流程。',
    webmailShortcutsTitle: '键盘快捷键',
    webmailShortcutsLead: '前端显示的快捷键帮助应作为命令组记录。',
    startAdminLead: '登录、确认公司、阅读仪表板，然后进入匹配任务的功能详情页。',
    startWebmailLead: '登录、打开邮箱、阅读或撰写邮件，然后在需要时调整个人设置。',
    sourceNote: '基于当前前端路由列表、消息文件和可见控件编写。',
  },
} satisfies Record<DocsLocale, Record<string, string>>;

function makeMessages(locale: DocsLocale): DocsMessages {
  const c = copy[locale];
  const console = consoleTerms(locale);
  const webmail = webmailTerms(locale);
  const product = webmail.common.gogomail;
  const adminConsole = console.login.subtitle;
  const webmailName = product;
  const guideTitle = `${product} ${c.guide}`;
  const f = featureCopy[locale];
  const userPage = console.pages.users_page;
  const userBulk = userPage.users_bulk as Record<string, string>;
  const adminUsersPage = console.pages.admin_users_page;
  const settingCategories = {
    en: ['Mailbox', 'Compose', 'Filters', 'Theme', 'Notifications', 'Account', 'Security', 'Shortcuts', 'Advanced'],
    ko: ['메일함', '메일 쓰기', '필터', '테마', '알림', '계정', '보안', '단축키', '고급'],
    ja: ['メールボックス', '作成', 'フィルター', 'テーマ', '通知', 'アカウント', 'セキュリティ', 'ショートカット', '詳細'],
    'zh-CN': ['邮箱', '撰写', '过滤器', '主题', '通知', '账号', '安全', '快捷键', '高级'],
  }[locale];
  const settingsViewSections = {
    en: ['Account', 'Inbox', 'Reading', 'Compose', 'Filters', 'Storage and backup', 'Blocked list', 'Vacation response', 'Privacy', 'Appearance', 'Notifications', 'Shortcuts', 'Security', 'Accessibility', 'About'],
    ko: ['계정', '받은편지함', '읽기', '작성', '필터', '용량/백업', '차단 목록', '자동 응답', '개인정보 보호', '외관', '알림', '단축키', '보안', '접근성', '정보'],
    ja: ['アカウント', '受信トレイ', '閲覧', '作成', 'フィルター', '容量/バックアップ', 'ブロックリスト', '自動応答', 'プライバシー', '外観', '通知', 'ショートカット', 'セキュリティ', 'アクセシビリティ', '情報'],
    'zh-CN': ['账号', '收件箱', '阅读', '撰写', '过滤器', '容量/备份', '阻止列表', '自动回复', '隐私', '外观', '通知', '快捷键', '安全', '辅助功能', '关于'],
  }[locale];
  const settingItems = {
    en: {
      mailbox: ['read/unread behavior', 'list density', 'default sort', 'conversation view', 'message preview'],
      compose: ['quote original message on reply', 'signature', 'draft auto-save', 'send delay'],
      filters: ['sender', 'recipient', 'cc', 'subject', 'body text', 'has attachment', 'unread only', 'larger than', 'smaller than'],
      theme: ['light', 'dark', 'system default', 'accent color', 'font size'],
      notifications: ['mail notifications'],
      security: ['block tracking pixels', 'external images', 'inline images'],
      advanced: ['webmail settings', 'theme preference', 'filter rules'],
    },
    ko: {
      mailbox: ['읽음/읽지 않음 처리', '목록 밀도', '기본 정렬', '대화형 보기', '미리보기 표시'],
      compose: ['답장 시 원문 인용', '서명', '임시보관 자동 저장', '전송 지연'],
      filters: ['보낸 사람', '받는 사람', '참조', '제목', '본문', '첨부파일 있음', '읽지 않음', '용량 초과', '용량 미만'],
      theme: ['라이트', '다크', '시스템 설정 따르기', '강조 색상', '글자 크기'],
      notifications: ['메일 알림'],
      security: ['추적 픽셀 차단', '외부 이미지', '본문 내 이미지'],
      advanced: ['웹메일 설정', '테마 선택', '필터 규칙'],
    },
    ja: {
      mailbox: ['既読/未読の扱い', '一覧密度', '既定の並び順', 'スレッド表示', 'プレビュー表示'],
      compose: ['返信時に元メッセージを引用', '署名', '下書き自動保存', '送信遅延'],
      filters: ['差出人', '宛先', 'CC', '件名', '本文', '添付あり', '未読のみ', '指定サイズより大きい', '指定サイズより小さい'],
      theme: ['ライト', 'ダーク', 'システム設定に従う', 'アクセントカラー', '文字サイズ'],
      notifications: ['メール通知'],
      security: ['トラッキングピクセルをブロック', '外部画像', 'インライン画像'],
      advanced: ['ウェブメール設定', 'テーマ設定', 'フィルタールール'],
    },
    'zh-CN': {
      mailbox: ['已读/未读处理', '列表密度', '默认排序', '会话视图', '预览显示'],
      compose: ['回复时引用原文', '签名', '草稿自动保存', '发送延迟'],
      filters: ['发件人', '收件人', '抄送', '主题', '正文', '有附件', '仅未读', '大于指定大小', '小于指定大小'],
      theme: ['浅色', '深色', '跟随系统', '强调色', '字体大小'],
      notifications: ['邮件通知'],
      security: ['阻止跟踪像素', '外部图片', '内联图片'],
      advanced: ['Webmail 设置', '主题偏好', '过滤规则'],
    },
  }[locale];
  const translated = (messages: Record<DocsLocale, string>) => messages[locale];
  const glossaryCopy = {
    en: {
      title: 'Glossary',
      lead: 'Plain-language explanations for mail, security, administration, and delivery terms used across the GoGoMail guide.',
      eyebrow: 'Reference',
    },
    ko: {
      title: '용어 사전',
      lead: 'GoGoMail 가이드에서 쓰는 메일, 보안, 관리자, 전송 관련 전문 용어를 실제 운영자가 이해할 수 있는 말로 설명합니다.',
      eyebrow: '참조',
    },
    ja: {
      title: '用語集',
      lead: 'GoGoMail ガイドで使うメール、セキュリティ、管理、配信関連の専門用語を、運用者が理解しやすい言葉で説明します。',
      eyebrow: '参照',
    },
    'zh-CN': {
      title: '术语表',
      lead: '用易懂语言解释 GoGoMail 指南中使用的邮件、安全、管理和投递相关术语。',
      eyebrow: '参考',
    },
  }[locale];
  const glossarySections: PageSection[] = [
    {
      id: 'governance',
      title: translated({ en: 'Governance', ko: '거버넌스', ja: 'ガバナンス', 'zh-CN': '治理' }),
      aliases: [translated({ en: 'Governance and security', ko: '거버넌스와 보안', ja: 'ガバナンスとセキュリティ', 'zh-CN': '治理与安全' })],
      body: translated({
        en: 'Governance means operating rules and evidence management. In GoGoMail, it covers how the mail service is controlled, how changes are recorded, and how security or compliance review is supported later.',
        ko: '거버넌스는 운영 규칙과 증적 관리를 뜻합니다. GoGoMail에서는 메일 서비스를 어떤 규칙으로 운영할지, 누가 무엇을 바꿨는지 어떻게 남길지, 보안이나 규정 준수 검토를 나중에 어떻게 확인할지를 다룹니다.',
        ja: 'ガバナンスは、運用ルールと証跡管理を意味します。GoGoMail では、メールサービスをどの規則で運用するか、誰が何を変更したか、後からセキュリティやコンプライアンスを確認できるかを扱います。',
        'zh-CN': '治理指运营规则和证据管理。在 GoGoMail 中，它说明邮件服务按什么规则运行、谁更改了什么，以及之后如何支持安全或合规审查。',
      }),
      paragraphs: [translated({
        en: 'Open this area when the question is about policy, audit evidence, administrator activity, retention, legal hold, access restrictions, or security posture.',
        ko: '정책, 감사 증적, 관리자 활동, 보존 기간, 법적 보관, 접근 제한, 보안 현황을 확인해야 할 때 이 영역을 사용합니다.',
        ja: 'ポリシー、監査証跡、管理者活動、保持、法的保全、アクセス制限、セキュリティ態勢を確認するときに使います。',
        'zh-CN': '当问题涉及策略、审计证据、管理员活动、保留、法律保留、访问限制或安全态势时使用此区域。',
      })],
    },
    {
      id: 'tenant',
      title: translated({ en: 'Tenant', ko: '테넌트', ja: 'テナント', 'zh-CN': '租户' }),
      body: translated({
        en: 'A tenant is the company or organization being operated inside GoGoMail. The selected company, its domains, users, policies, reports, and audit records belong to that tenant.',
        ko: '테넌트는 GoGoMail 안에서 운영되는 회사 또는 조직 단위입니다. 선택된 회사, 도메인, 사용자, 정책, 보고서, 감사 기록은 이 테넌트에 속합니다.',
        ja: 'テナントは GoGoMail 内で運用される会社または組織単位です。選択中の会社、ドメイン、ユーザー、ポリシー、レポート、監査記録はそのテナントに属します。',
        'zh-CN': '租户是在 GoGoMail 中运营的公司或组织单位。所选公司、域名、用户、策略、报告和审计记录都属于该租户。',
      }),
    },
    {
      id: 'domain',
      title: translated({ en: 'Domain', ko: '도메인', ja: 'ドメイン', 'zh-CN': '域名' }),
      body: translated({
        en: 'A domain is the part after @ in an email address. Before users receive or send company mail, the domain must be added, verified, and connected to DNS records such as SPF, DKIM, and DMARC.',
        ko: '도메인은 이메일 주소에서 @ 뒤에 오는 회사 주소입니다. 사용자가 회사 메일을 주고받기 전에 도메인을 추가하고, 소유를 확인하고, SPF, DKIM, DMARC 같은 DNS 레코드를 연결해야 합니다.',
        ja: 'ドメインはメールアドレスの @ の後ろにある会社の住所です。ユーザーが会社メールを送受信する前に、追加、所有確認、SPF、DKIM、DMARC などの DNS レコード接続が必要です。',
        'zh-CN': '域名是邮箱地址中 @ 后面的公司地址。用户收发公司邮件前，需要添加域名、验证所有权，并连接 SPF、DKIM、DMARC 等 DNS 记录。',
      }),
    },
    {
      id: 'dns',
      title: 'DNS',
      body: translated({
        en: 'DNS is the public address book for domains. Mail services use DNS records to prove domain ownership, find receiving mail servers, and publish authentication settings.',
        ko: 'DNS는 도메인의 공개 주소록입니다. 메일 서비스는 DNS 레코드로 도메인 소유를 확인하고, 받는 메일 서버를 찾고, 인증 설정을 공개합니다.',
        ja: 'DNS はドメインの公開アドレス帳です。メールサービスは DNS レコードで所有確認、受信メールサーバーの検索、認証設定の公開を行います。',
        'zh-CN': 'DNS 是域名的公开地址簿。邮件服务使用 DNS 记录验证域名所有权、查找收件服务器并发布认证设置。',
      }),
    },
    {
      id: 'sso-saml',
      title: 'SSO / SAML',
      aliases: ['SSO', 'SAML'],
      body: translated({
        en: 'SSO means single sign-on: users sign in to GoGoMail with the work account they already use. SAML is one common protocol used to connect that company identity provider.',
        ko: 'SSO는 Single Sign-On, 즉 사용자가 회사에서 이미 쓰는 계정으로 GoGoMail에 로그인하게 하는 방식입니다. SAML은 그 회사 인증 시스템을 연결할 때 자주 쓰는 표준 규약입니다.',
        ja: 'SSO は Single Sign-On の略で、会社で既に使っているアカウントで GoGoMail にログインする方式です。SAML はその認証システムを接続するためによく使われる標準プロトコルです。',
        'zh-CN': 'SSO 是单点登录，让用户用工作中已有的账号登录 GoGoMail。SAML 是连接公司身份提供商时常用的标准协议。',
      }),
    },
    {
      id: 'scim-provisioning',
      title: translated({ en: 'SCIM provisioning', ko: 'SCIM 프로비저닝', ja: 'SCIM プロビジョニング', 'zh-CN': 'SCIM 预配' }),
      aliases: ['SCIM', translated({ en: 'SCIM status', ko: 'SCIM 상태', ja: 'SCIM 状態', 'zh-CN': 'SCIM 状态' })],
      body: translated({
        en: 'SCIM provisioning is automatic user synchronization between an identity system and GoGoMail. It helps create, update, disable, or remove users without manually editing every account in the console.',
        ko: 'SCIM 프로비저닝은 외부 계정 시스템과 GoGoMail 사용자 목록을 자동으로 맞추는 기능입니다. 사용자를 생성, 변경, 비활성화, 삭제할 때 콘솔에서 계정을 하나씩 손으로 고치지 않게 해줍니다.',
        ja: 'SCIM プロビジョニングは、外部 ID システムと GoGoMail のユーザー一覧を自動同期する機能です。ユーザーの作成、変更、無効化、削除を手作業で繰り返さずに済みます。',
        'zh-CN': 'SCIM 预配是在身份系统和 GoGoMail 用户列表之间自动同步用户的功能。它可帮助创建、更新、停用或删除用户，而不必在控制台逐个手动修改。',
      }),
    },
    {
      id: 'webhook',
      title: translated({ en: 'Webhook', ko: '웹훅', ja: 'Webhook', 'zh-CN': 'Webhook' }),
      aliases: [translated({ en: 'Webhooks', ko: '웹훅', ja: 'Webhook', 'zh-CN': 'Webhook' })],
      body: translated({
        en: 'A webhook is an automatic notification from GoGoMail to another system. Use it when an external tool must react to mail, security, or operational events.',
        ko: '웹훅은 GoGoMail에서 다른 시스템으로 보내는 자동 알림입니다. 메일, 보안, 운영 이벤트가 발생했을 때 외부 도구가 바로 반응해야 하면 사용합니다.',
        ja: 'Webhook は GoGoMail から別システムへ送る自動通知です。メール、セキュリティ、運用イベントに外部ツールを反応させたいときに使います。',
        'zh-CN': 'Webhook 是 GoGoMail 发送到其他系统的自动通知。外部工具需要响应邮件、安全或运营事件时使用。',
      }),
    },
    {
      id: 'dkim',
      title: 'DKIM',
      aliases: [translated({ en: 'DKIM keys', ko: 'DKIM 키', ja: 'DKIM キー', 'zh-CN': 'DKIM 密钥' })],
      body: translated({
        en: 'DKIM is a mail authentication signature. GoGoMail signs outbound mail, and the receiver checks the public key in DNS to confirm the mail was authorized by the domain and was not changed in transit.',
        ko: 'DKIM은 발신 메일에 붙는 인증 서명입니다. GoGoMail이 메일에 서명하면 받는 서버는 DNS에 공개된 키를 확인해 이 메일이 해당 도메인에서 허가되어 나갔고 중간에 변조되지 않았는지 판단합니다.',
        ja: 'DKIM は送信メールに付ける認証署名です。GoGoMail がメールに署名し、受信側は DNS の公開鍵でドメインの許可と改ざん有無を確認します。',
        'zh-CN': 'DKIM 是出站邮件上的认证签名。GoGoMail 对邮件签名，收件服务器通过 DNS 中的公钥确认邮件由该域名授权发送且未被篡改。',
      }),
    },
    {
      id: 'spf',
      title: 'SPF',
      body: translated({
        en: 'SPF is a DNS record that lists which servers may send mail for a domain. It helps receivers reject mail that pretends to come from the domain but was sent from an unauthorized server.',
        ko: 'SPF는 어떤 서버가 이 도메인으로 메일을 보낼 수 있는지 DNS에 적어두는 기록입니다. 허가되지 않은 서버가 회사 도메인인 척 보내는 메일을 걸러내는 데 도움을 줍니다.',
        ja: 'SPF は、そのドメインとしてメール送信できるサーバーを DNS に記録する仕組みです。未許可サーバーからのなりすまし送信を判定しやすくします。',
        'zh-CN': 'SPF 是写在 DNS 中的记录，用于列出哪些服务器可以代表该域名发信。它帮助收件方拦截未经授权服务器伪装域名发送的邮件。',
      }),
    },
    {
      id: 'dmarc',
      title: 'DMARC',
      aliases: ['DMARC / SPF', translated({ en: 'DMARC/SPF', ko: 'DMARC / SPF 정책', ja: 'DMARC/SPF', 'zh-CN': 'DMARC/SPF' })],
      body: translated({
        en: 'DMARC is a domain policy that tells receivers what to do when SPF or DKIM authentication fails. It can be used for monitoring first, then stricter handling such as quarantine or rejection.',
        ko: 'DMARC는 SPF나 DKIM 인증이 실패했을 때 받는 서버가 어떻게 처리해야 하는지 알려주는 도메인 정책입니다. 처음에는 모니터링으로 시작하고, 준비가 되면 격리나 거부처럼 더 강한 정책으로 올릴 수 있습니다.',
        ja: 'DMARC は、SPF または DKIM 認証に失敗したとき受信側がどう扱うかを示すドメインポリシーです。まず監視から始め、準備後に隔離や拒否へ強められます。',
        'zh-CN': 'DMARC 是域名策略，用于告诉收件方当 SPF 或 DKIM 认证失败时如何处理。可先用于监控，准备好后再提升到隔离或拒收。',
      }),
    },
    {
      id: 'smtp',
      title: 'SMTP',
      aliases: [translated({ en: 'SMTP policy', ko: 'SMTP 정책', ja: 'SMTP ポリシー', 'zh-CN': 'SMTP 策略' })],
      body: translated({
        en: 'SMTP is the basic protocol mail servers use to send mail to each other. SMTP policy controls what outbound mail is allowed, restricted, or routed through specific relays.',
        ko: 'SMTP는 메일 서버끼리 메일을 보내는 기본 전송 규약입니다. SMTP 정책은 발신 메일을 어떤 조건으로 허용하고 제한하며 어느 릴레이나 경로로 보낼지 정합니다.',
        ja: 'SMTP はメールサーバー同士がメールを送る基本プロトコルです。SMTP ポリシーは送信許可、制限、リレーや経路を制御します。',
        'zh-CN': 'SMTP 是邮件服务器之间发送邮件的基础协议。SMTP 策略控制出站邮件如何允许、限制或通过指定中继路由。',
      }),
    },
    {
      id: 'api',
      title: 'API',
      body: translated({
        en: 'An API is a software interface. Instead of a person clicking the console, another system can call GoGoMail through an API to automate a supported task.',
        ko: 'API는 프로그램이 GoGoMail과 통신하는 접점입니다. 사람이 콘솔을 클릭하는 대신 다른 시스템이 API를 호출해 지원되는 작업을 자동화할 수 있습니다.',
        ja: 'API はソフトウェア用の接点です。人がコンソールを操作する代わりに、別システムが GoGoMail API を呼び出して対応タスクを自動化できます。',
        'zh-CN': 'API 是软件接口。其他系统可通过 API 调用 GoGoMail 来自动化支持的任务，而不是由人点击控制台。',
      }),
    },
    {
      id: 'api-key',
      title: translated({ en: 'API key', ko: 'API 키', ja: 'API キー', 'zh-CN': 'API 密钥' }),
      body: translated({
        en: 'An API key is a credential for software. Treat it like a password because another system can use it to call GoGoMail APIs.',
        ko: 'API 키는 프로그램이 GoGoMail API를 호출할 때 쓰는 비밀 키입니다. 다른 시스템이 이 키로 작업을 수행할 수 있으므로 비밀번호처럼 관리해야 합니다.',
        ja: 'API キーはソフトウェア用の認証情報です。他システムが GoGoMail API を呼び出せるため、パスワード同様に管理します。',
        'zh-CN': 'API 密钥是给软件使用的凭据。其他系统可用它调用 GoGoMail API，因此应像密码一样管理。',
      }),
    },
    {
      id: 'api-metering',
      title: translated({ en: 'API metering', ko: 'API 미터링', ja: 'API メータリング', 'zh-CN': 'API 计量' }),
      aliases: [translated({ en: 'API usage', ko: 'API 사용량', ja: 'API 使用量', 'zh-CN': 'API 使用量' })],
      body: translated({
        en: 'API metering records API calls so administrators can review who called which route, with which key, how often, how much data moved, and whether the call succeeded.',
        ko: 'API 미터링은 API 호출을 기록해 관리자가 누가 어떤 라우트를 어떤 키로 얼마나 자주 호출했는지, 데이터가 얼마나 오갔는지, 호출이 성공했는지 확인할 수 있게 하는 운영 기록입니다.',
        ja: 'API メータリングは API 呼び出しを記録し、管理者が誰がどのルートをどのキーで、どの頻度で呼び出し、どれだけデータが動き、成功したかを確認できるようにします。',
        'zh-CN': 'API 计量记录 API 调用，使管理员能够查看谁使用哪个密钥调用了哪个路由、频率、数据量以及调用是否成功。',
      }),
    },
    {
      id: 'mfa-2fa',
      title: 'MFA / 2FA',
      aliases: ['MFA', '2FA', translated({ en: 'MFA management', ko: '2FA 관리', ja: 'MFA 管理', 'zh-CN': 'MFA 管理' })],
      body: translated({
        en: 'MFA or 2FA means requiring an additional verification step after the password. It reduces the damage from a stolen password because sign-in still needs a second factor.',
        ko: 'MFA 또는 2FA는 비밀번호 외에 추가 확인 단계를 요구하는 로그인 보호 방식입니다. 비밀번호가 유출되어도 두 번째 확인 수단이 필요하므로 계정 탈취 위험을 줄입니다.',
        ja: 'MFA または 2FA は、パスワードに加えて追加確認を求めるログイン保護方式です。パスワード漏えい時の被害を減らします。',
        'zh-CN': 'MFA 或 2FA 表示密码之外还需要额外验证步骤。即使密码泄露，登录仍需要第二个因素，可降低账号被盗风险。',
      }),
    },
    {
      id: 'ip-access',
      title: translated({ en: 'IP access', ko: 'IP 접근', ja: 'IP アクセス', 'zh-CN': 'IP 访问' }),
      body: translated({
        en: 'IP access controls which network addresses may reach an admin or service surface. Use it to limit sensitive access to trusted office, VPN, or infrastructure networks.',
        ko: 'IP 접근은 어떤 네트워크 주소에서 관리자 화면이나 서비스에 접근할 수 있는지 제한하는 설정입니다. 민감한 접근을 사무실, VPN, 인프라 네트워크처럼 신뢰한 위치로 좁힐 때 사용합니다.',
        ja: 'IP アクセスは、どのネットワークアドレスから管理画面やサービスへ到達できるかを制限する設定です。',
        'zh-CN': 'IP 访问控制哪些网络地址可以访问管理或服务界面。可用于把敏感访问限制到可信办公室、VPN 或基础设施网络。',
      }),
    },
    {
      id: 'suppression-list',
      title: translated({ en: 'Suppression list', ko: '차단 목록', ja: '抑止リスト', 'zh-CN': '阻止列表' }),
      body: translated({
        en: 'A suppression list is a do-not-send list. Addresses or domains on the list should be skipped before delivery is attempted.',
        ko: '차단 목록은 보내지 말아야 할 주소나 도메인을 모아둔 목록입니다. 여기에 포함된 대상은 전송을 시도하기 전에 제외해야 합니다.',
        ja: '抑止リストは送信しないアドレスやドメインの一覧です。登録された対象は配信前に除外します。',
        'zh-CN': '阻止列表是“不应发送”的地址或域名列表。列入其中的对象应在投递尝试前跳过。',
      }),
    },
    {
      id: 'retention-policy',
      title: translated({ en: 'Retention policy', ko: '보존 정책', ja: '保持ポリシー', 'zh-CN': '保留策略' }),
      aliases: [translated({ en: 'Data retention policy', ko: '데이터 보존 정책', ja: 'データ保持ポリシー', 'zh-CN': '数据保留策略' })],
      body: translated({
        en: 'Retention policy decides how long mail, logs, or evidence should be kept before normal deletion rules can remove them.',
        ko: '보존 정책은 메일, 로그, 증적을 얼마나 오래 남겨야 하는지 정하는 규칙입니다. 일반 삭제나 정리 작업보다 우선하는 운영 기준으로 다룹니다.',
        ja: '保持ポリシーは、メール、ログ、証跡をどれだけ保存するかを決める規則です。通常削除より優先される運用基準です。',
        'zh-CN': '保留策略决定邮件、日志或证据应保存多久，通常应优先于普通删除或清理规则。',
      }),
    },
    {
      id: 'legal-hold',
      title: translated({ en: 'Legal hold', ko: '법적 보관', ja: '法的保全', 'zh-CN': '法律保留' }),
      body: translated({
        en: 'A legal hold preserves selected data because it may be needed for legal or compliance review. Normal cleanup should not remove data under hold.',
        ko: '법적 보관은 법무나 규정 준수 검토에 필요할 수 있는 데이터를 삭제되지 않게 붙잡아 두는 설정입니다. 보관 대상 데이터는 일반 정리 작업으로 삭제되지 않아야 합니다.',
        ja: '法的保全は、法務またはコンプライアンスレビューに必要な可能性があるデータを削除されないよう保持する設定です。',
        'zh-CN': '法律保留会保存可能用于法律或合规审查的数据。处于保留状态的数据不应被普通清理规则删除。',
      }),
    },
    {
      id: 'audit-log',
      title: translated({ en: 'Audit log', ko: '감사 로그', ja: '監査ログ', 'zh-CN': '审计日志' }),
      body: translated({
        en: 'An audit log records important administrative actions: who did what, when, from where, and whether it succeeded. Use it as evidence during investigation or review.',
        ko: '감사 로그는 중요한 관리자 작업을 기록합니다. 누가, 언제, 어디서, 무엇을 했고 성공했는지 남기므로 조사나 검토 때 증거로 사용합니다.',
        ja: '監査ログは重要な管理操作を記録します。誰が、いつ、どこから、何を行い、成功したかを調査やレビューの証拠として使います。',
        'zh-CN': '审计日志记录重要管理操作：谁在何时何地做了什么，以及是否成功。调查或审查时作为证据使用。',
      }),
    },
    {
      id: 'message-trace',
      title: translated({ en: 'Message trace', ko: '메시지 추적', ja: 'メッセージ追跡', 'zh-CN': '消息追踪' }),
      body: translated({
        en: 'Message trace follows one message through the system. Start here when you know a sender, recipient, message ID, time window, or delivery status.',
        ko: '메시지 추적은 특정 메일 한 건이 시스템 안에서 어떻게 처리됐는지 따라가는 기능입니다. 보낸 사람, 받는 사람, 메시지 ID, 시간대, 전송 상태 중 하나라도 알 때 먼저 사용합니다.',
        ja: 'メッセージ追跡は、特定の 1 通がシステム内でどう処理されたかを追う機能です。送信者、受信者、メッセージ ID、時間帯、配信状態が分かるときに使います。',
        'zh-CN': '消息追踪用于跟踪单封邮件在系统中的处理过程。当知道发件人、收件人、消息 ID、时间窗口或投递状态时先使用。',
      }),
    },
    {
      id: 'mail-flow-log',
      title: translated({ en: 'Mail flow log', ko: '메일 흐름 로그', ja: 'メールフローログ', 'zh-CN': '邮件流日志' }),
      body: translated({
        en: 'Mail flow logs show delivery-related events across messages. Use them after message trace when you need broader context around routing, retries, or failures.',
        ko: '메일 흐름 로그는 여러 메일의 전송 관련 이벤트를 보여줍니다. 메시지 추적만으로 부족할 때 라우팅, 재시도, 실패 주변 맥락을 확인하는 데 사용합니다.',
        ja: 'メールフローログは複数メールの配信関連イベントを表示します。メッセージ追跡だけでは足りないとき、ルーティング、再試行、失敗の文脈を確認します。',
        'zh-CN': '邮件流日志显示多封邮件的投递相关事件。当消息追踪不足时，用于查看路由、重试或失败周边上下文。',
      }),
    },
    {
      id: 'delivery-attempt',
      title: translated({ en: 'Delivery attempt', ko: '전송 시도', ja: '配信試行', 'zh-CN': '投递尝试' }),
      body: translated({
        en: 'A delivery attempt is one try to send a message to the next server. A message may have multiple attempts when retry rules are applied.',
        ko: '전송 시도는 메일을 다음 서버로 보내려는 한 번의 시도입니다. 재시도 규칙이 적용되면 하나의 메일에도 여러 전송 시도가 남을 수 있습니다.',
        ja: '配信試行は、次のサーバーへメールを送ろうとする 1 回の試みです。再試行ルールにより、1 通に複数の試行が残ることがあります。',
        'zh-CN': '投递尝试是把邮件发送到下一台服务器的一次尝试。应用重试规则时，一封邮件可能留下多次投递尝试。',
      }),
    },
    {
      id: 'routing-rule',
      title: translated({ en: 'Routing rule', ko: '라우팅 규칙', ja: 'ルーティングルール', 'zh-CN': '路由规则' }),
      aliases: [translated({ en: 'Delivery route', ko: '전송 라우트', ja: '配信ルート', 'zh-CN': '投递路由' })],
      body: translated({
        en: 'A routing rule decides where matching mail should go next. It can direct mail through a specific route, relay, or policy path.',
        ko: '라우팅 규칙은 조건에 맞는 메일을 다음에 어디로 보낼지 정합니다. 특정 라우트, 릴레이, 정책 경로로 보내는 데 사용합니다.',
        ja: 'ルーティングルールは、条件に合うメールを次にどこへ送るかを決めます。特定のルート、リレー、ポリシー経路へ送るために使います。',
        'zh-CN': '路由规则决定匹配邮件下一步去哪里。可用于把邮件送到特定路由、中继或策略路径。',
      }),
    },
    {
      id: 'trusted-relay',
      title: translated({ en: 'Trusted relay', ko: '신뢰 릴레이', ja: '信頼済みリレー', 'zh-CN': '可信中继' }),
      aliases: [translated({ en: 'Relay', ko: '릴레이', ja: 'リレー', 'zh-CN': '中继' })],
      body: translated({
        en: 'A relay is a server that forwards mail on the way to its destination. A trusted relay is one the administrator has explicitly allowed for delivery flow.',
        ko: '릴레이는 목적지로 가는 중간에서 메일을 전달하는 서버입니다. 신뢰 릴레이는 관리자가 전송 흐름에 사용해도 된다고 명시적으로 허용한 릴레이입니다.',
        ja: 'リレーは宛先へ向かう途中でメールを転送するサーバーです。信頼済みリレーは、管理者が配信フローで使用を明示的に許可したリレーです。',
        'zh-CN': '中继是在邮件到达目的地途中转发邮件的服务器。可信中继是管理员明确允许用于投递流程的中继。',
      }),
    },
    {
      id: 'queue',
      title: translated({ en: 'Queue', ko: '큐', ja: 'キュー', 'zh-CN': '队列' }),
      aliases: [translated({ en: 'Queue statistics', ko: '큐 통계', ja: 'キュー統計', 'zh-CN': '队列统计' })],
      body: translated({
        en: 'A queue is a waiting line for work the system has not finished yet. In mail delivery, queued messages are waiting for processing, retry, or downstream capacity.',
        ko: '큐는 아직 끝나지 않은 작업이 기다리는 줄입니다. 메일 전송에서는 처리, 재시도, 다음 서버의 여유를 기다리는 메시지가 큐에 쌓입니다.',
        ja: 'キューは、まだ完了していない作業の待ち行列です。メール配信では、処理、再試行、後段の余力を待つメッセージがキューに入ります。',
        'zh-CN': '队列是尚未完成工作的等待线。邮件投递中，等待处理、重试或下游容量的邮件会进入队列。',
      }),
    },
    {
      id: 'backpressure',
      title: translated({ en: 'Backpressure', ko: '백프레셔', ja: 'バックプレッシャー', 'zh-CN': '背压' }),
      body: translated({
        en: 'Backpressure means the system intentionally slows intake or delivery because a downstream queue, server, or provider is under pressure. Read it as a protection signal, not just a failure.',
        ko: '백프레셔는 뒤쪽 큐, 서버, 외부 제공자가 부담을 받고 있어서 시스템이 의도적으로 수신이나 전송 속도를 낮추는 상태입니다. 단순 장애가 아니라 과부하 확산을 막는 보호 신호로 읽어야 합니다.',
        ja: 'バックプレッシャーは、後段のキュー、サーバー、外部プロバイダーに負荷があるため、取り込みや配信を意図的に遅くしている状態です。',
        'zh-CN': '背压表示下游队列、服务器或外部提供商承压，系统有意放慢接收或投递速度。它应被视为保护信号，而不只是失败。',
      }),
    },
    {
      id: 'quota',
      title: translated({ en: 'Quota', ko: '할당량', ja: 'クォータ', 'zh-CN': '配额' }),
      body: translated({
        en: 'A quota is an allowed limit, usually for storage, seats, messages, or API usage. Quota pages help administrators see current usage before users hit a limit.',
        ko: '할당량은 저장소, 좌석, 메시지, API 사용량처럼 허용된 한도를 뜻합니다. 할당량 화면은 사용자가 한도에 걸리기 전에 현재 사용량을 확인하게 해줍니다.',
        ja: 'クォータは、ストレージ、シート、メッセージ、API 使用量などの許容量です。クォータ画面は制限到達前に現在使用量を確認するために使います。',
        'zh-CN': '配额是存储、席位、消息或 API 使用量等允许上限。配额页面帮助管理员在用户触及限制前查看当前使用量。',
      }),
    },
    {
      id: 'csv',
      title: 'CSV',
      body: translated({
        en: 'CSV is a simple spreadsheet-style file format. In administration, it is commonly used to import or export user lists and review them outside the console.',
        ko: 'CSV는 스프레드시트처럼 행과 열로 된 단순 파일 형식입니다. 관리자 작업에서는 사용자 목록을 가져오거나 내보내고 콘솔 밖에서 검토할 때 자주 사용합니다.',
        ja: 'CSV は表計算のように行と列で構成される単純なファイル形式です。管理作業ではユーザー一覧のインポートやエクスポートに使われます。',
        'zh-CN': 'CSV 是类似电子表格的简单行列文件格式。管理工作中常用于导入或导出用户列表，并在控制台外审阅。',
      }),
    },
  ];
  const screenBody = (name: string) => ({
    en: `${name} is written as an operator guide for the visible route, labels, and controls. Follow the sequence below before saving or changing related screens.`,
    ko: `${name} 화면은 실제 라우트, 라벨, 컨트롤을 기준으로 관리자가 따라갈 수 있는 작업 순서로 설명합니다. 저장하거나 연결 화면을 바꾸기 전에 아래 흐름을 먼저 확인합니다.`,
    ja: `${name} 画面は、実際のルート、ラベル、コントロールを基準に、管理者が従える作業順序として説明します。保存または関連画面を変更する前に、下の流れを確認します。`,
    'zh-CN': `${name} 页面根据实际路由、标签和控件写成管理员可执行的操作顺序。保存或更改相关页面前，先按照下面的流程确认。`,
  })[locale];
  const sourceBody = {
    en: f.sourceNote,
    ko: f.sourceNote,
    ja: f.sourceNote,
    'zh-CN': f.sourceNote,
  }[locale];
  const join = (items?: string[]) => (items?.filter(Boolean).join(', ') || {
    en: 'No related labels are exposed on this screen yet.',
    ko: '아직 이 화면에서 연결 라벨이 충분히 노출되지 않았습니다.',
    ja: 'この画面では、関連ラベルがまだ十分に表示されていません。',
    'zh-CN': '此页面尚未暴露足够的相关标签。',
  }[locale]);
  const consoleSpecific = (section: PageSection) => {
    const title = section.title;
    const related = join(section.items);
    const termIntro = new Map<string, string>([
      [console.nav.sso_config, translated({
        en: 'SSO means single sign-on: administrators connect the company identity provider so users can sign in with the same account they already use at work.',
        ko: 'SSO는 Single Sign-On, 즉 회사에서 이미 쓰는 로그인 계정으로 GoGoMail에도 로그인하게 만드는 설정입니다. 관리자는 외부 인증 시스템을 연결하고, 어떤 사용자가 어떤 권한으로 들어오는지 확인합니다.',
        ja: 'SSO は Single Sign-On の略で、会社で既に使っているログインアカウントで GoGoMail にもサインインできるようにする設定です。',
        'zh-CN': 'SSO 是单点登录，表示把公司的身份提供商连接到 GoGoMail，让用户用工作中已有的账号登录。',
      })],
      [console.nav.webhooks, translated({
        en: 'A webhook is an outgoing notification from GoGoMail to another system. Use it when an external tool must react to mail, security, or operation events.',
        ko: '웹훅은 GoGoMail에서 다른 시스템으로 보내는 자동 알림입니다. 메일, 보안, 운영 이벤트가 발생했을 때 외부 도구가 즉시 반응해야 한다면 이 화면을 설정합니다.',
        ja: 'Webhook は GoGoMail から別システムへ送る自動通知です。メール、セキュリティ、運用イベントに外部ツールを反応させたいときに使います。',
        'zh-CN': 'Webhook 是 GoGoMail 发送到其他系统的自动通知。外部工具需要响应邮件、安全或运营事件时使用。',
      })],
      [console.nav.scim_provisioning, translated({
        en: 'SCIM provisioning means automatic user synchronization between an identity provider and GoGoMail. It is used to create, update, disable, or remove accounts without manually editing every user in the console.',
        ko: 'SCIM 프로비저닝은 외부 계정 시스템과 GoGoMail 사용자 목록을 자동으로 맞추는 기능입니다. 회사의 인사/계정 시스템에서 사용자가 생성, 변경, 비활성화되면 콘솔에서 매번 수동으로 수정하지 않도록 연결 상태를 확인합니다.',
        ja: 'SCIM プロビジョニングは、外部の ID 管理システムと GoGoMail のユーザー一覧を自動同期する仕組みです。ユーザー作成、変更、無効化を手作業で繰り返さないために使います。',
        'zh-CN': 'SCIM 预配表示在身份系统和 GoGoMail 用户列表之间自动同步用户。用于创建、更新、停用或删除账号，而不必在控制台逐个手动修改。',
      })],
      [console.nav.dkim_keys, translated({
        en: 'DKIM is an email signature mechanism. It lets receiving mail servers verify that outbound mail was authorized by the domain and was not changed in transit.',
        ko: 'DKIM은 발신 메일에 도메인의 서명을 붙이는 메일 인증 방식입니다. 받는 서버는 이 서명을 보고 메일이 해당 도메인에서 허가되어 나갔고 중간에 변조되지 않았는지 확인합니다.',
        ja: 'DKIM は送信メールにドメインの署名を付けるメール認証方式です。受信側は送信許可と改ざん有無を確認できます。',
        'zh-CN': 'DKIM 是给出站邮件加上域名签名的邮件认证方式。收件服务器可用它确认邮件由该域名授权发送且未被篡改。',
      })],
      [console.nav.dmarc_spf, translated({
        en: 'DMARC and SPF are domain protection records. SPF says which servers may send mail for the domain; DMARC says what receivers should do when authentication fails.',
        ko: 'DMARC와 SPF는 도메인을 사칭 발신으로부터 보호하는 DNS 정책입니다. SPF는 어떤 서버가 이 도메인으로 메일을 보낼 수 있는지 선언하고, DMARC는 인증 실패 메일을 받는 서버가 어떻게 처리해야 하는지 정합니다.',
        ja: 'DMARC と SPF はドメインをなりすまし送信から守る DNS ポリシーです。SPF は送信可能なサーバーを示し、DMARC は認証失敗時の扱いを定めます。',
        'zh-CN': 'DMARC 和 SPF 是保护域名免受冒充发送的 DNS 策略。SPF 声明哪些服务器可代表域名发信，DMARC 指定认证失败时收件方应如何处理。',
      })],
      [console.nav.smtp_policy, translated({
        en: 'SMTP is the protocol used to send mail between mail servers. SMTP policy controls how outbound mail is allowed, restricted, or routed.',
        ko: 'SMTP는 메일 서버끼리 메일을 보내는 기본 전송 규약입니다. SMTP 정책은 발신 메일을 어떤 조건으로 허용하고 제한하며 어느 경로로 보낼지 정합니다.',
        ja: 'SMTP はメールサーバー間でメールを送る基本プロトコルです。SMTP ポリシーは送信許可、制限、経路を決めます。',
        'zh-CN': 'SMTP 是邮件服务器之间发送邮件的基础协议。SMTP 策略控制出站邮件如何允许、限制或路由。',
      })],
      [console.nav.api_keys, translated({
        en: 'An API key is a credential for software, not a person. Treat it like a password because another system can use it to call GoGoMail APIs.',
        ko: 'API 키는 사람이 아니라 프로그램이 GoGoMail API를 호출할 때 쓰는 비밀 키입니다. 다른 시스템이 이 키로 작업을 수행할 수 있으므로 비밀번호처럼 관리합니다.',
        ja: 'API キーは人ではなくソフトウェア用の認証情報です。他システムが GoGoMail API を呼び出せるため、パスワード同様に扱います。',
        'zh-CN': 'API 密钥是给软件使用的凭据，不是给人使用的账号。其他系统可用它调用 GoGoMail API，因此应像密码一样管理。',
      })],
      [console.nav.backpressure, translated({
        en: 'Backpressure means the system is intentionally slowing intake or delivery because a downstream queue, server, or provider is under pressure.',
        ko: '백프레셔는 뒤쪽 큐, 서버, 외부 제공자가 부담을 받고 있어서 시스템이 의도적으로 수신이나 전송 속도를 낮추는 상태입니다. 장애가 아니라 과부하 확산을 막는 보호 신호로 읽어야 합니다.',
        ja: 'バックプレッシャーは、後段のキュー、サーバー、外部プロバイダーに負荷があるため、取り込みや配信を意図的に遅くしている状態です。',
        'zh-CN': '背压表示下游队列、服务器或外部提供商承压，系统有意放慢接收或投递速度，以避免过载扩散。',
      })],
      [console.nav.suppression_list, translated({
        en: 'A suppression list is a do-not-send list. Addresses or domains placed here should be skipped before delivery is attempted.',
        ko: '차단 목록은 “보내지 말아야 할 대상” 목록입니다. 여기에 들어간 주소나 도메인은 전송을 시도하기 전에 제외해야 합니다.',
        ja: '抑止リストは「送信しない対象」の一覧です。登録されたアドレスやドメインは配信前に除外します。',
        'zh-CN': '阻止列表是“不应发送”的对象列表。列入其中的地址或域名应在投递尝试前跳过。',
      })],
      [console.nav.retention_policy, translated({
        en: 'Retention policy decides how long mail, logs, or evidence should be kept before normal deletion rules can remove them.',
        ko: '보존 정책은 메일, 로그, 증적을 얼마나 오래 남겨야 하는지 정하는 규칙입니다. 일반 삭제나 정리 작업보다 우선하는 운영 기준으로 다룹니다.',
        ja: '保持ポリシーは、メール、ログ、証跡をどれだけ保存するかを決める規則です。通常削除より優先される運用基準です。',
        'zh-CN': '保留策略决定邮件、日志或证据应保存多久，通常应优先于普通删除或清理规则。',
      })],
      [console.nav.legal_holds, translated({
        en: 'A legal hold preserves selected data because it may be needed for legal or compliance review. It should prevent normal cleanup from removing that evidence.',
        ko: '법적 보관은 법무나 규정 준수 검토에 필요할 수 있는 데이터를 삭제되지 않게 붙잡아 두는 설정입니다. 일반 보존 기간이 지나도 해당 증적이 정리되지 않도록 다룹니다.',
        ja: '法的保全は、法務またはコンプライアンスレビューに必要な可能性があるデータを削除されないよう保持する設定です。',
        'zh-CN': '法律保留会保存可能用于法律或合规审查的数据，避免普通清理规则删除这些证据。',
      })],
    ]);
    const entries = new Map<string, string[]>([
      [console.nav.domains, [
        translated({ en: `Open ${title} after choosing the company. Add or review each domain, check status, DNS state, quota, and creation date, then run DNS checks before onboarding users.`, ko: `${title}에서는 먼저 회사 선택이 맞는지 확인한 뒤 도메인을 추가하거나 기존 도메인을 검토합니다. 상태, DNS, 할당량, 생성일을 한 줄씩 확인하고, 사용자 온보딩 전에 DNS 확인을 실행해 메일 수신과 발신 준비 상태를 맞춥니다.`, ja: `${title} では、会社選択を確認してからドメインを追加または確認します。状態、DNS、クォータ、作成日を行ごとに確認し、ユーザーオンボーディング前に DNS チェックを実行します。`, 'zh-CN': `在 ${title} 中，先确认所选公司，然后添加或检查域名。逐行确认状态、DNS、配额和创建日期，并在用户入驻前执行 DNS 检查。` }),
        translated({ en: 'When a domain is not ready, move to domain settings, DKIM, DMARC/SPF, and outbound SMTP in that order. Do not create users on a domain until the domain row shows the expected operational state.', ko: '도메인이 준비되지 않았다면 도메인 설정, DKIM 키, DMARC / SPF 정책, 아웃바운드 SMTP 순서로 이동합니다. 도메인 행의 상태가 기대한 운영 상태가 되기 전에는 그 도메인에 사용자를 대량 생성하지 않습니다.', ja: 'ドメインが未準備の場合は、ドメイン設定、DKIM、DMARC/SPF、アウトバウンド SMTP の順に確認します。期待する状態になる前に、そのドメインへユーザーを大量作成しません。', 'zh-CN': '如果域名尚未就绪，依次进入域名设置、DKIM、DMARC/SPF 和出站 SMTP。域名行达到预期状态前，不要批量创建该域名用户。' }),
      ]],
      [console.nav.users, [
        translated({ en: `Use ${title} for the complete user lifecycle: search, filter by domain or status, create users, send invites, import/export CSV, change quota, change role, suspend, activate, and offboard.`, ko: `${title}에서는 검색, 도메인/상태 필터, 사용자 생성, 초대 발송, CSV 가져오기/내보내기, 할당량 변경, 역할 변경, 정지, 활성화, 오프보딩까지 사용자 생애주기를 처리합니다.`, ja: `${title} では、検索、ドメイン/状態フィルター、作成、招待、CSV インポート/エクスポート、クォータ変更、ロール変更、停止、有効化、オフボーディングまで扱います。`, 'zh-CN': `在 ${title} 中处理用户生命周期：搜索、按域名或状态筛选、创建、邀请、CSV 导入/导出、配额、角色、暂停、启用和离职处理。` }),
        translated({ en: 'For a new user, choose the domain, enter the address and display name, select invite or temporary-password mode, then save. After saving, verify the count tabs and copy or deliver the invite link if the flow created one.', ko: '신규 사용자는 도메인을 고르고 이메일 주소와 표시 이름을 입력한 뒤 초대 방식 또는 임시 비밀번호 방식을 선택해 저장합니다. 저장 후 전체/활성/정지 카운트와 목록 행을 확인하고, 초대 링크가 생성되었다면 사용자에게 전달합니다.', ja: '新規ユーザーでは、ドメイン、メールアドレス、表示名を入力し、招待方式または一時パスワード方式を選んで保存します。保存後、件数タブと一覧行を確認し、招待リンクがあれば共有します。', 'zh-CN': '新用户应选择域名，输入邮箱地址和显示名，选择邀请或临时密码模式后保存。保存后确认计数标签和列表行，若生成邀请链接则交给用户。' }),
      ]],
      [console.nav.admin_users, [
        translated({ en: `Use ${title} separately from mailbox users. Create only the administrator accounts that must sign in to the console, then assign system administrator, administrator, or read-only according to operational responsibility.`, ko: `${title}는 일반 메일 사용자와 분리해서 다룹니다. 콘솔에 로그인해야 하는 관리자 계정만 만들고, 실제 책임에 맞춰 시스템 관리자, 관리자, 읽기 전용을 배정합니다.`, ja: `${title} は通常のメールユーザーとは分けて扱います。コンソールにログインする必要がある管理者だけを作成し、責任に応じてシステム管理者、管理者、読み取り専用を割り当てます。`, 'zh-CN': `${title} 应与邮箱用户分开处理。只创建必须登录控制台的管理员账号，并按职责分配系统管理员、管理员或只读。` }),
        translated({ en: 'Before removing an administrator, check audit logs and administrator activity so ownership of recent changes is not lost. After removal, verify that the account disappeared from the administrator list.', ko: '관리자를 삭제하기 전에는 감사 로그와 관리자 활동에서 최근 변경의 소유자가 사라져도 되는지 확인합니다. 삭제 후 관리자 목록에서 계정이 사라졌는지 확인합니다.', ja: '管理者を削除する前に、監査ログと管理者アクティビティで最近の変更所有者を確認します。削除後、管理者一覧から消えていることを確認します。', 'zh-CN': '删除管理员前，先在审计日志和管理员活动中确认近期变更归属不会丢失。删除后确认该账号已从管理员列表消失。' }),
      ]],
      [console.nav.message_trace, [
        translated({ en: `Start ${title} with the most specific value available: RFC message ID, sender, recipient, direction, status, or time. Use it before broad log browsing.`, ko: `${title}는 RFC 메시지 ID, 보낸 사람, 받는 사람, 방향, 상태, 시간처럼 가장 구체적인 단서부터 입력합니다. 넓은 로그를 뒤지기 전에 특정 메시지를 좁히는 첫 화면으로 사용합니다.`, ja: `${title} は RFC メッセージ ID、送信者、受信者、方向、状態、時刻など最も具体的な手掛かりから始めます。広いログを見る前に特定メッセージを絞り込みます。`, 'zh-CN': `${title} 应从最具体的线索开始：RFC 消息 ID、发件人、收件人、方向、状态或时间。先定位消息，再查看宽泛日志。` }),
        translated({ en: 'If the trace result is incomplete, continue to mail flow logs and delivery attempts. Keep the same time window and recipient so the investigation stays aligned.', ko: '추적 결과가 부족하면 같은 시간대와 수신자를 유지한 채 메일 흐름 로그와 전송 시도로 이동합니다. 이렇게 해야 조사 기준이 흔들리지 않습니다.', ja: '追跡結果が不十分な場合は、同じ時間帯と宛先を維持してメールフローログと配信試行へ進みます。', 'zh-CN': '如果追踪结果不足，保持相同时间窗口和收件人，继续查看邮件流日志和投递尝试。' }),
      ]],
      [console.nav.audit_logs, [
        translated({ en: `Use ${title} when the question is who changed what, when, from where, and with what result. Filter first, then export only the evidence needed for review.`, ko: `${title}는 누가, 언제, 어디서, 무엇을 바꿨고 결과가 무엇인지 확인할 때 사용합니다. 먼저 행을 필터링하고, 검토에 필요한 증적만 내보냅니다.`, ja: `${title} は、誰が、いつ、どこから、何を変更し、結果が何だったかを確認するときに使います。先に絞り込み、必要な証跡だけをエクスポートします。`, 'zh-CN': `${title} 用于确认谁在何时何地更改了什么以及结果。先筛选行，再只导出审查所需证据。` }),
        translated({ en: 'Open administrator activity when the audit row needs operator-friendly context. Open reports when the same evidence must be shared as a review artifact.', ko: '감사 행만으로 맥락이 부족하면 관리자 활동을 열고, 같은 증적을 검토 자료로 공유해야 하면 보고서로 이동합니다.', ja: '監査行だけで文脈が不足する場合は管理者アクティビティを開き、レビュー資料として共有する場合はレポートへ進みます。', 'zh-CN': '如果审计行缺少上下文，打开管理员活动；如果需要作为审查材料共享，进入报告。' }),
      ]],
      [console.nav.dkim_keys, [
        translated({ en: `Use ${title} after the domain exists. Select or enter the domain, choose a selector, provide key material, then copy the DNS public key record into the domain DNS zone.`, ko: `${title}는 도메인이 생성된 뒤 설정합니다. 도메인을 선택하거나 입력하고 selector와 키 자료를 준비한 뒤, DNS 공개 키 레코드를 해당 도메인의 DNS 존에 반영합니다.`, ja: `${title} はドメイン作成後に設定します。ドメイン、selector、鍵素材を用意し、DNS 公開鍵レコードを対象ドメインの DNS ゾーンへ反映します。`, 'zh-CN': `${title} 在域名创建后设置。选择或输入域名，准备 selector 和密钥材料，然后把 DNS 公钥记录写入域名 DNS 区。` }),
        translated({ en: 'After saving, run DNS-related checks from the domain or DMARC/SPF screen. A key that exists in the console but is missing in DNS does not protect outbound mail.', ko: '저장 후에는 도메인 또는 DMARC / SPF 화면에서 DNS 관련 확인을 수행합니다. 콘솔에 키가 있어도 DNS에 공개 키가 없으면 발신 메일 보호가 완성되지 않습니다.', ja: '保存後、ドメインまたは DMARC/SPF 画面で DNS 確認を実行します。コンソールに鍵があっても DNS に公開鍵がなければ送信保護は完了しません。', 'zh-CN': '保存后，在域名或 DMARC/SPF 页面执行 DNS 检查。控制台中有密钥但 DNS 中没有公钥，并不能保护出站邮件。' }),
      ]],
      [console.nav.reports, [
        translated({ en: `Use ${title} after filtering the source screens. Pick the report type, generate or export it, then verify the file reflects the same company, domain, user, and time window used during review.`, ko: `${title}는 원본 화면에서 필터 기준을 정리한 뒤 사용합니다. 보고서 유형을 선택해 생성하거나 내보내고, 파일 안의 회사, 도메인, 사용자, 시간대가 검토 기준과 맞는지 확인합니다.`, ja: `${title} は元画面でフィルター条件を整理してから使います。レポート種別を選び、生成またはエクスポートし、会社、ドメイン、ユーザー、時間帯がレビュー条件と一致することを確認します。`, 'zh-CN': `${title} 应在源页面整理筛选条件后使用。选择报告类型并生成或导出，然后确认文件中的公司、域名、用户和时间窗口与审查标准一致。` }),
        translated({ en: 'For compliance work, keep the report together with audit logs and administrator activity. For user or domain work, compare the export with the current list screen before sharing it.', ko: '규정 준수 검토라면 보고서를 감사 로그와 관리자 활동과 함께 보관합니다. 사용자나 도메인 보고서라면 공유 전에 현재 목록 화면과 내보낸 파일이 맞는지 비교합니다.', ja: 'コンプライアンスでは、レポートを監査ログと管理者アクティビティと一緒に保管します。ユーザーやドメインのレポートは、共有前に現在の一覧画面と比較します。', 'zh-CN': '合规审查时，将报告与审计日志和管理员活动一起保存。用户或域名报告在共享前应与当前列表页面比较。' }),
      ]],
    ]);
    const paragraphs = entries.get(title) ?? [
      translated({ en: `Start ${title} by confirming the current company, then read the list, filters, counters, and primary action buttons from top to bottom. Do not save a change until the visible row or form matches the intended target.`, ko: `콘솔의 ${title} 화면을 사용할 때는 먼저 현재 회사가 맞는지 확인하고, 목록, 필터, 카운터, 주요 작업 버튼을 위에서 아래로 읽습니다. 보이는 행이나 폼이 의도한 대상과 일치하기 전에는 저장하지 않습니다.`, ja: `${title} を使うときは、まず現在の会社を確認し、一覧、フィルター、件数、主要ボタンを上から順に読みます。対象が一致するまで保存しません。`, 'zh-CN': `使用 ${title} 时，先确认当前公司，然后从上到下阅读列表、筛选器、计数器和主要操作按钮。可见行或表单与目标一致前不要保存。` }),
      translated({ en: `After the action, verify the result in this screen and in related screens: ${related}. If the value affects delivery, security, or audit evidence, also check the relevant operational log before closing the task.`, ko: `작업 후에는 이 화면과 연결 화면에서 결과를 확인합니다: ${related}. 값이 전송, 보안, 감사 증적에 영향을 준다면 작업을 닫기 전에 관련 운영 로그까지 확인합니다.`, ja: `作業後、この画面と関連画面で結果を確認します: ${related}。配信、セキュリティ、監査証跡に影響する値なら、関連する運用ログも確認します。`, 'zh-CN': `操作后，在此页面和相关页面确认结果：${related}。如果该值影响投递、安全或审计证据，关闭任务前还要检查相关运营日志。` }),
    ];
    const intro = termIntro.get(title);
    return intro ? [intro, ...paragraphs] : paragraphs;
  };
  const webmailSpecific = (section: PageSection) => {
    const title = section.title;
    const related = join(section.items);
    const entries = new Map<string, string[]>([
      [webmail.mail.folders, [
        translated({ en: `Use ${title} from the left mailbox list. Start with inbox, sent, drafts, trash, spam, archive, and custom folders, then confirm whether the message list is showing all, unread, or filtered mail.`, ko: `${title}는 왼쪽 메일함 목록에서 시작합니다. 받은편지함, 보낸편지함, 임시보관함, 휴지통, 스팸, 보관함, 사용자 폴더를 확인하고 메시지 목록이 전체, 읽지 않음, 검색/필터 상태인지 먼저 확인합니다.`, ja: `${title} は左のメールボックス一覧から始めます。受信、送信済み、下書き、ゴミ箱、迷惑メール、アーカイブ、ユーザーフォルダーを確認し、一覧が全体、未読、検索/フィルター状態か確認します。`, 'zh-CN': `${title} 从左侧邮箱列表开始。先确认收件箱、已发送、草稿、垃圾箱、垃圾邮件、归档和自定义文件夹，并确认邮件列表是否处于全部、未读、搜索或筛选状态。` }),
        translated({ en: 'When a folder looks empty, check loading and empty states before assuming data loss. If a message should have attachments or labels, verify the row indicators before opening the reading pane.', ko: '폴더가 비어 보이면 데이터가 없다고 판단하기 전에 로딩 상태와 빈 상태를 구분합니다. 첨부파일이나 라벨이 있어야 하는 메시지는 읽기 패널을 열기 전에 목록 행의 표시를 먼저 확인합니다.', ja: 'フォルダーが空に見える場合、データ欠損と判断する前に読み込み状態と空状態を区別します。添付やラベルがあるはずのメッセージは、閲覧ペインを開く前に行の表示を確認します。', 'zh-CN': '文件夹看起来为空时，先区分加载状态和空状态。应有附件或标签的邮件，在打开阅读窗格前先确认列表行标识。' }),
      ]],
      [webmail.compose.newMessage, [
        translated({ en: `Use ${title} only after confirming the sender account. Fill recipients first, then subject, then body. The product validates recipients and requires a subject or body before sending.`, ko: `${title}는 보내는 계정을 확인한 뒤 사용합니다. 받는 사람을 먼저 입력하고, 제목과 본문을 차례로 채웁니다. 제품은 받는 사람을 검증하며 제목 또는 본문이 있어야 전송할 수 있습니다.`, ja: `${title} は送信アカウントを確認してから使います。宛先、件名、本文の順に入力します。製品は宛先を検証し、件名または本文が必要です。`, 'zh-CN': `${title} 应在确认发件账号后使用。先填写收件人，再填写主题和正文。产品会校验收件人，并要求主题或正文。` }),
        translated({ en: 'Use formatting actions only after the message content is stable. Before sending, review recipient chips, subject, body, attachments, and any link inserted through the toolbar.', ko: '서식 도구는 본문 내용이 정리된 뒤 적용합니다. 보내기 전에는 받는 사람 칩, 제목, 본문, 첨부파일, 도구 모음으로 삽입한 링크를 모두 검토합니다.', ja: '書式操作は本文が固まってから適用します。送信前に宛先チップ、件名、本文、添付、ツールバーで挿入したリンクを確認します。', 'zh-CN': '格式工具应在正文稳定后使用。发送前检查收件人 chip、主题、正文、附件以及通过工具栏插入的链接。' }),
      ]],
      [webmail.settings.theme, [
        translated({ en: `Use ${title} when the user wants light, dark, or system-following display behavior. Change the theme, then immediately check the mailbox list and reading pane because those are the screens users stare at longest.`, ko: `${title}는 라이트, 다크, 시스템 설정 따르기 같은 표시 방식을 바꿀 때 사용합니다. 변경 후에는 사용자가 가장 오래 보는 메일 목록과 읽기 패널의 대비를 바로 확인합니다.`, ja: `${title} はライト、ダーク、システム追従を変更するときに使います。変更後、最も長く見るメール一覧と閲覧ペインのコントラストを確認します。`, 'zh-CN': `${title} 用于更改浅色、深色或跟随系统的显示方式。更改后立即检查邮件列表和阅读窗格的对比度。` }),
        translated({ en: 'If an accent color or font size setting is used, verify buttons, selected rows, links, and validation messages. A theme setting is not complete until interactive states remain readable.', ko: '강조 색상이나 글자 크기도 바꾼다면 버튼, 선택된 행, 링크, 검증 메시지를 함께 확인합니다. 인터랙션 상태가 읽기 쉬워야 테마 설정이 완료된 것입니다.', ja: 'アクセントカラーやフォントサイズも変更する場合、ボタン、選択行、リンク、検証メッセージを確認します。操作状態が読みやすいことが完了条件です。', 'zh-CN': '如果还更改强调色或字体大小，应同时检查按钮、选中行、链接和校验消息。交互状态仍然可读，主题设置才算完成。' }),
      ]],
    ]);
    return entries.get(title) ?? [
      translated({ en: `Use ${title} from the visible webmail surface and keep the user task in view: read, compose, organize, search, or personalize. Change one thing at a time so the result is obvious.`, ko: `웹메일의 ${title} 항목은 사용자의 실제 목적을 놓치지 않고 다룹니다. 읽기, 작성, 정리, 검색, 개인화 중 어떤 작업인지 먼저 정하고 한 번에 하나씩 변경해 결과를 분명하게 확인합니다.`, ja: `${title} は、読む、書く、整理する、検索する、個人化するという目的を意識して使います。一度に一つだけ変更し、結果を明確に確認します。`, 'zh-CN': `在 Webmail 中使用 ${title} 时，先明确用户是在阅读、撰写、整理、搜索还是个性化。一次只改一项，确保结果清楚。` }),
      translated({ en: `After the action, verify the visible state, loading or empty messages, validation text, and related labels: ${related}. If the change affects sending, test with a draft before relying on it.`, ko: `작업 후에는 보이는 상태, 로딩 또는 빈 메시지, 검증 문구, 연결 라벨을 확인합니다: ${related}. 전송에 영향을 주는 변경이라면 실제로 의존하기 전에 임시 작성 흐름으로 먼저 확인합니다.`, ja: `作業後、表示状態、読み込みまたは空メッセージ、検証文言、関連ラベルを確認します: ${related}。送信に影響する変更なら、下書きで先に確認します。`, 'zh-CN': `操作后确认可见状态、加载或空消息、校验文字和相关标签：${related}。如果影响发送，先用草稿流程验证。` }),
    ];
  };
  const guideParagraphs = (pageKey: string, section: PageSection) =>
    pageKey.startsWith('webmail') ? webmailSpecific(section) : consoleSpecific(section);
  const enrichPages = (pages: Record<string, PageMessage>) => Object.fromEntries(
    Object.entries(pages).map(([pageKey, page]) => [
      pageKey,
      {
        ...page,
        sections: page.sections.map(section => ({
          ...section,
          paragraphs: section.paragraphs ?? (
            ['home', 'adminOverview', 'webmailOverview'].includes(pageKey)
              ? undefined
              : guideParagraphs(pageKey, section)
          ),
        })),
      },
    ])
  ) as Record<string, PageMessage>;

  return {
    site: {
      title: guideTitle,
      description: c.description,
      label: localeLabels[locale],
      search: console.common.search,
      openSource: c.openSource,
      lastUpdated: c.lastUpdated,
    },
    code: {
      copy: c.copyCode,
      copied: c.copiedCode,
    },
    nav: {
      home: c.guide,
      adminConsole,
      webmail: webmailName,
      gettingStarted: c.gettingStarted,
      overview: c.overview,
    },
    pages: enrichPages({
      glossary: {
        eyebrow: glossaryCopy.eyebrow,
        title: glossaryCopy.title,
        lead: glossaryCopy.lead,
        sections: glossarySections,
      },
      home: {
        eyebrow: c.openSource,
        title: guideTitle,
        lead: c.homeLead,
        primaryCta: { text: c.primary, link: link(locale, '/admin-console/') },
        secondaryCta: { text: c.secondary, link: link(locale, '/webmail/') },
        sections: [
          {
            title: adminConsole,
            body: c.adminLead,
            items: [
              console.nav.dashboard,
              console.nav.section_resources,
              console.nav.section_operations,
              console.nav.section_governance,
              console.nav.section_analytics_storage,
              console.nav.domains,
              console.nav.users,
            ],
          },
          {
            title: webmailName,
            body: c.webmailLead,
            items: [
              webmail.mail.inbox,
              webmail.mail.sent,
              webmail.mail.drafts,
              webmail.mail.attachments,
              webmail.compose.newMessage,
              webmail.settings.language,
            ],
          },
          {
            title: c.factualTitle,
            body: c.factualBody,
            items: [c.termTitle, c.factualLimitTitle, c.docGrowthTitle],
          },
          {
            title: c.termTitle,
            body: c.termBody,
            items: [console.common.search, webmail.common.search, webmail.settings.theme, webmail.settings.language],
          },
          {
            title: c.transparentTitle,
            body: c.transparentBody,
            items: [console.nav.audit_logs, console.nav.admin_activity, console.nav.reports],
          },
        ],
      },
      adminOverview: {
        eyebrow: adminConsole,
        title: adminConsole,
        lead: c.adminLead,
        primaryCta: { text: c.consoleStart, link: link(locale, '/admin-console/getting-started') },
        sections: [
          {
            title: c.adminNavTitle,
            body: c.adminNavBody,
            items: [
              console.nav.section_resources,
              console.nav.settings,
              console.nav.section_operations,
              console.nav.section_access_control,
              console.nav.section_governance,
              console.nav.section_analytics_storage,
            ],
          },
          {
            title: console.nav.dashboard,
            body: c.adminDashboardBody,
            items: [console.layout.company, console.common.refresh, console.nav.tenant_health, console.nav.users],
          },
          {
            title: console.nav.section_resources,
            body: c.adminResourcesBody,
            items: [
              console.nav.companies,
              console.nav.domains,
              console.nav.tenant_health,
              console.nav.change_history,
              console.nav.users,
              console.nav.admin_users,
              console.nav.onboarding,
            ],
          },
          {
            title: console.nav.settings,
            body: c.adminSettingsBody,
            items: [
              console.nav.company_config,
              console.nav.domain_settings,
              console.nav.sso_config,
              console.nav.webhooks,
              console.nav.notif_templates,
              console.nav.global_signature,
              console.nav.scim_provisioning,
              console.nav.user_config,
            ],
          },
          {
            title: console.nav.section_operations,
            body: c.adminOperationsBody,
            items: [
              console.nav.message_trace,
              console.nav.mail_flow_logs,
              console.nav.outbox_events,
              console.nav.delivery_attempts,
              console.nav.routing_rules,
              console.nav.delivery_routes,
              console.nav.trusted_relays,
              console.nav.queue_stats,
              console.nav.backpressure,
              console.nav.api_health,
            ],
          },
          {
            title: console.nav.section_access_control,
            body: c.adminAccessBody,
            items: [
              console.nav.directory,
              console.nav.aliases,
              console.nav.delegations,
              console.nav.group_memberships,
              console.nav.roles,
            ],
          },
          {
            title: console.nav.section_governance,
            body: c.adminGovernanceBody,
            items: [
              console.nav.audit_logs,
              console.nav.admin_activity,
              console.nav.alert_rules,
              console.nav.suppression_list,
              console.nav.dkim_keys,
              console.nav.api_keys,
              console.nav.api_settings,
              console.nav.mfa_management,
              console.nav.ip_access,
              console.nav.auth_policy,
              console.nav.audit_policy,
              console.nav.retention_policy,
              console.nav.session_mgmt,
              console.nav.rate_limits,
              console.nav.dmarc_spf,
              console.nav.spam_filter,
              console.nav.smtp_policy,
              console.nav.security_posture,
              console.nav.compliance,
              console.nav.legal_holds,
            ],
          },
          {
            title: console.nav.section_analytics_storage,
            body: c.adminAnalyticsBody,
            items: [
              console.nav.quota_dashboard,
              console.nav.quota_usage,
              console.nav.quota_alerts,
              console.nav.attachments,
              console.nav.drive,
              console.nav.seat_usage,
              console.nav.api_usage,
              console.nav.push_notifications,
              console.nav.reports,
            ],
          },
        ],
      },
      adminGettingStarted: {
        eyebrow: adminConsole,
        title: c.consoleStart,
        lead: f.startAdminLead,
        sections: [
          {
            title: c.loginTitle,
            body: c.adminLoginBody,
            items: [console.login.email_label, console.login.password_label, console.layout.company, console.nav.dashboard],
          },
          {
            title: c.adminDashboardTitle,
            body: c.adminDashboardBody,
            items: [console.common.refresh, console.nav.tenant_health, console.nav.message_trace, console.nav.audit_logs],
          },
          {
            title: f.rolesTitle,
            body: f.rolesLead,
            items: [
              userPage.role_system_admin as string,
              userPage.role_company_admin as string,
              userPage.role_user_email as string,
              console.nav.roles,
            ],
          },
        ],
      },
      adminRoles: {
        eyebrow: adminConsole,
        title: f.rolesTitle,
        lead: f.rolesLead,
        sections: [
          {
            title: userPage.role_system_admin as string,
            body: f.roleSystem,
            items: [console.nav.companies, console.nav.admin_users, console.nav.audit_logs, console.nav.reports],
          },
          {
            title: userPage.role_company_admin as string,
            body: f.roleCompany,
            items: [console.nav.users, console.nav.domains, console.nav.company_config, console.nav.security_posture],
          },
          {
            title: userPage.role_user_email as string,
            body: f.roleMailOnly,
            items: [webmail.mail.inbox, webmail.compose.newMessage, webmail.settings.theme, webmail.settings.language],
          },
          {
            title: adminUsersPage.admin_accounts as string,
            body: f.roleAdminAccount,
            items: [
              adminUsersPage.system_admin as string,
              adminUsersPage.admin as string,
              adminUsersPage.read_only as string,
              adminUsersPage.remove as string,
            ],
          },
          {
            title: console.nav.domains,
            body: f.roleDomain,
            items: [console.nav.domain_settings, console.nav.delegations, console.nav.group_memberships, console.nav.dmarc_spf],
          },
        ],
      },
      adminResources: {
        eyebrow: adminConsole,
        title: f.resourcesTitle,
        lead: f.resourcesLead,
        sections: [
          { title: console.nav.companies, body: screenBody(console.nav.companies), items: [console.layout.company, console.nav.company_config, console.nav.quota_dashboard] },
          { title: console.nav.domains, body: c.adminDomainBody, items: [console.nav.domain_settings, console.nav.dkim_keys, console.nav.dmarc_spf, console.nav.smtp_policy] },
          { title: console.nav.tenant_health, body: screenBody(console.nav.tenant_health), items: [console.nav.dashboard, console.common.refresh, console.nav.api_health] },
          { title: console.nav.change_history, body: screenBody(console.nav.change_history), items: [console.nav.audit_logs, console.nav.admin_activity] },
          { title: console.nav.users, body: c.adminUsersBody, items: [userPage.total_label as string, userPage.active_label as string, userPage.suspended_label as string, userBulk.export_btn, userBulk.import_btn] },
          { title: console.nav.admin_users, body: screenBody(console.nav.admin_users), items: [adminUsersPage.add_admin_btn as string, adminUsersPage.role as string, adminUsersPage.status as string, adminUsersPage.remove as string] },
          { title: console.nav.onboarding, body: screenBody(console.nav.onboarding), items: [console.nav.companies, console.nav.domains, console.nav.dkim_keys, console.nav.users] },
        ],
      },
      adminSettings: {
        eyebrow: adminConsole,
        title: f.settingsTitle,
        lead: f.settingsLead,
        sections: [
          { title: console.nav.company_config, body: screenBody(console.nav.company_config), items: [console.nav.domains, console.nav.users, console.nav.quota_dashboard] },
          { title: console.nav.domain_settings, body: screenBody(console.nav.domain_settings), items: [console.nav.domains, console.nav.dkim_keys, console.nav.dmarc_spf] },
          { title: console.nav.sso_config, body: screenBody(console.nav.sso_config), items: ['Admin', 'Member', 'Viewer'] },
          { title: console.nav.webhooks, body: screenBody(console.nav.webhooks), items: [console.nav.notif_templates, console.nav.alert_rules] },
          { title: console.nav.notif_templates, body: screenBody(console.nav.notif_templates), items: [console.common.save, console.common.cancel] },
          { title: console.nav.global_signature, body: screenBody(console.nav.global_signature), items: [webmail.compose.newMessage, webmail.compose.bodyPlaceholder] },
          { title: console.nav.scim_provisioning, body: screenBody(console.nav.scim_provisioning), items: [console.nav.users, console.nav.domains, console.nav.sso_config] },
          { title: console.nav.user_config, body: screenBody(console.nav.user_config), items: [console.nav.users, console.nav.domains] },
        ],
      },
      externalIntegration: {
        eyebrow: adminConsole,
        title: f.externalIntegrationTitle,
        lead: f.externalIntegrationLead,
        sections: [
          {
            title: translated({ en: 'When to use it', ko: '언제 사용하나', ja: 'いつ使うか', 'zh-CN': '何时使用' }),
            body: translated({
              en: 'Use the external integration API when another company system needs to show mail counts, unread mail, recent mailbox lists, search results, or a compose entry point without forcing users to open webmail first.',
              ko: '다른 회사 시스템에서 사용자가 웹메일을 먼저 열지 않아도 메일 카운트, 안 읽은 메일, 최근 메일 목록, 검색 결과, 메일 쓰기 진입점을 보여주고 싶을 때 외부 연동 API를 사용합니다.',
              ja: '別の社内システムが、ユーザーに先にウェブメールを開かせずにメール件数、未読メール、最近の一覧、検索結果、作成入口を表示したい場合に使います。',
              'zh-CN': '当其他公司系统希望在用户进入 Webmail 前显示邮件数量、未读邮件、最近邮件列表、搜索结果或写信入口时使用外部集成 API。',
            }),
            paragraphs: [
              translated({
                en: 'The external system should call GoGoMail from a trusted server process, not directly from browser JavaScript. The API key is a server credential and must not be exposed to end users.',
                ko: '외부 시스템은 브라우저 JavaScript에서 직접 호출하지 말고, 신뢰할 수 있는 서버 프로세스에서 GoGoMail을 호출해야 합니다. API 키는 서버 자격 증명이므로 최종 사용자에게 노출하면 안 됩니다.',
                ja: '外部システムはブラウザー JavaScript から直接呼び出さず、信頼できるサーバープロセスから GoGoMail を呼び出します。API キーはサーバー資格情報であり、利用者に見せてはいけません。',
                'zh-CN': '外部系统应由可信服务器进程调用 GoGoMail，而不是由浏览器 JavaScript 直接调用。API 密钥是服务器凭据，不能暴露给最终用户。',
              }),
            ],
            items: [console.nav.api_keys, console.nav.api_settings, console.nav.api_usage, webmail.mail.inbox, webmail.compose.newMessage],
          },
          {
            title: translated({ en: 'Authentication model', ko: '인증 모델', ja: '認証モデル', 'zh-CN': '认证模型' }),
            body: translated({
              en: 'External integration calls use Authorization: Bearer with a GoGoMail API key. The key is bound to one domain, and every mailbox request must also identify the target user with user_id or the X-Gogomail-User-ID header.',
              ko: '외부 연동 호출은 Authorization: Bearer 헤더에 GoGoMail API 키를 넣어 사용합니다. 키는 하나의 도메인에 묶이며, 메일함 요청은 user_id 쿼리 또는 X-Gogomail-User-ID 헤더로 대상 사용자를 반드시 지정해야 합니다.',
              ja: '外部連携呼び出しは Authorization: Bearer に GoGoMail API キーを入れて使います。キーは 1 つのドメインに結び付き、メールボックス要求では user_id または X-Gogomail-User-ID ヘッダーで対象ユーザーを必ず指定します。',
              'zh-CN': '外部集成调用使用 Authorization: Bearer 携带 GoGoMail API 密钥。密钥绑定到一个域名，每个邮箱请求还必须通过 user_id 或 X-Gogomail-User-ID 请求头指定目标用户。',
            }),
            paragraphs: [
              translated({
                en: 'GoGoMail checks that the target user belongs to the same domain as the API key. If the user belongs to another domain, the request is rejected before mailbox data is returned.',
                ko: 'GoGoMail은 대상 사용자가 API 키와 같은 도메인에 속하는지 확인합니다. 다른 도메인의 사용자를 지정하면 메일함 데이터를 반환하기 전에 요청을 거부합니다.',
                ja: 'GoGoMail は対象ユーザーが API キーと同じドメインに属するか確認します。別ドメインのユーザーを指定すると、メールボックスデータを返す前に拒否します。',
                'zh-CN': 'GoGoMail 会检查目标用户是否属于 API 密钥所在的同一域名。如果指定其他域名的用户，请求会在返回邮箱数据前被拒绝。',
              }),
            ],
            items: ['Authorization: Bearer gm_...', 'X-Gogomail-User-ID', 'user_id', console.nav.domains],
            examples: [
              {
                title: translated({ en: 'Common request headers', ko: '공통 요청 헤더', ja: '共通リクエストヘッダー', 'zh-CN': '通用请求头' }),
                language: 'http',
                code: `Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY
X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111
Accept: application/json
Content-Type: application/json`,
              },
              {
                title: translated({ en: 'Equivalent query parameter form', ko: '쿼리 파라미터 방식', ja: 'クエリパラメーター形式', 'zh-CN': '查询参数形式' }),
                language: 'http',
                code: `GET /api/v1/mailbox/overview?user_id=11111111-1111-1111-1111-111111111111 HTTP/1.1
Host: mail.example.com
Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY
Accept: application/json`,
              },
            ],
          },
          {
            title: translated({ en: 'Permission scopes', ko: '권한 범위', ja: '権限スコープ', 'zh-CN': '权限范围' }),
            body: translated({
              en: 'The API key must carry a mail scope that matches the operation. Reading counts, folders, messages, searches, and attachments uses mail:read. Sending a message or sending a draft uses mail:send. Folder changes, message moves, deletes, restores, draft edits, attachment uploads, preferences, push devices, and profile changes use mail:manage.',
              ko: 'API 키에는 작업에 맞는 메일 권한 범위가 있어야 합니다. 카운트, 폴더, 메시지, 검색, 첨부파일 읽기는 mail:read를 사용합니다. 새 메일 전송과 임시보관함 전송은 mail:send를 사용합니다. 폴더 변경, 메시지 이동, 삭제, 복원, 임시보관 수정, 첨부파일 업로드, 환경설정, 푸시 기기, 프로필 변경은 mail:manage를 사용합니다.',
              ja: 'API キーには操作に合うメールスコープが必要です。件数、フォルダー、メッセージ、検索、添付の読み取りは mail:read を使います。新規送信と下書き送信は mail:send を使います。フォルダー変更、移動、削除、復元、下書き編集、添付アップロード、設定、プッシュデバイス、プロフィール変更は mail:manage を使います。',
              'zh-CN': 'API 密钥必须带有匹配操作的邮件范围。读取数量、文件夹、邮件、搜索和附件使用 mail:read。发送新邮件或发送草稿使用 mail:send。文件夹变更、移动、删除、恢复、草稿编辑、附件上传、偏好设置、推送设备和个人资料变更使用 mail:manage。',
            }),
            paragraphs: [
              translated({
                en: 'For old or broad integrations, GoGoMail also accepts mail and mail:* as full mail access, and mail:write as a write-capable alias. New integrations should prefer the narrowest scope that works.',
                ko: '기존 연동이나 넓은 권한이 필요한 연동을 위해 GoGoMail은 mail과 mail:*을 전체 메일 권한으로, mail:write를 쓰기 가능한 별칭으로도 받아들입니다. 새 연동은 필요한 가장 좁은 범위를 선택해야 합니다.',
                ja: '既存連携や広い権限が必要な連携のため、GoGoMail は mail と mail:* を全メール権限、mail:write を書き込み可能な別名として受け付けます。新規連携では必要最小のスコープを選びます。',
                'zh-CN': '为兼容旧集成或需要宽权限的集成，GoGoMail 也接受 mail 和 mail:* 作为完整邮件权限，并接受 mail:write 作为可写别名。新集成应选择能满足需求的最小范围。',
              }),
            ],
            items: ['mail:read', 'mail:send', 'mail:manage', 'mail:write', 'mail:*'],
          },
          {
            title: translated({ en: 'Mailbox calls for external systems', ko: '외부 시스템에서 주로 호출하는 메일함 API', ja: '外部システムでよく使うメールボックス API', 'zh-CN': '外部系统常用邮箱 API' }),
            body: translated({
              en: 'For an external dashboard, start with GET /api/v1/mailbox/overview for total, unread, starred, size, and system folder identifiers. Use GET /api/v1/messages with folder_id, read=false, limit, cursor, and sort for inbox or unread lists. Use GET /api/v1/search when the external system exposes search filters.',
              ko: '외부 시스템 대시보드는 GET /api/v1/mailbox/overview로 전체 메일 수, 안 읽은 메일 수, 별표 수, 사용량, 시스템 폴더 식별자를 먼저 가져오면 됩니다. 받은 메일함이나 안 읽은 메일 목록은 GET /api/v1/messages에 folder_id, read=false, limit, cursor, sort를 조합해 호출합니다. 외부 시스템이 검색 필터를 제공하면 GET /api/v1/search를 사용합니다.',
              ja: '外部システムのダッシュボードでは、まず GET /api/v1/mailbox/overview で総数、未読数、スター数、容量、システムフォルダー ID を取得します。受信箱や未読一覧は GET /api/v1/messages に folder_id、read=false、limit、cursor、sort を組み合わせます。検索フィルターを出す場合は GET /api/v1/search を使います。',
              'zh-CN': '外部系统仪表板应先调用 GET /api/v1/mailbox/overview 获取总邮件数、未读数、星标数、容量和系统文件夹标识。收件箱或未读列表调用 GET /api/v1/messages，并组合 folder_id、read=false、limit、cursor 和 sort。外部系统提供搜索过滤器时使用 GET /api/v1/search。',
            }),
            paragraphs: [
              translated({
                en: 'For compose, call POST /api/v1/messages/send from the external system server. The body uses the same webmail send model: user_id, recipients, subject, text_body or html_body, optional attachments, and optional tracking or scheduling fields where enabled.',
                ko: '메일 쓰기는 외부 시스템 서버에서 POST /api/v1/messages/send를 호출합니다. 본문은 웹메일 전송 모델과 동일하게 user_id, 수신자, 제목, text_body 또는 html_body, 선택 첨부파일, 허용된 경우 추적 또는 예약 필드를 사용합니다.',
                ja: '作成は外部システムサーバーから POST /api/v1/messages/send を呼び出します。本文はウェブメール送信モデルと同じで、user_id、宛先、件名、text_body または html_body、任意の添付、許可された追跡または予約フィールドを使います。',
                'zh-CN': '写信由外部系统服务器调用 POST /api/v1/messages/send。请求体与 Webmail 发送模型相同：user_id、收件人、主题、text_body 或 html_body、可选附件，以及启用时的跟踪或定时字段。',
              }),
            ],
            items: ['/api/v1/mailbox/overview', '/api/v1/messages', '/api/v1/search', '/api/v1/messages/send'],
            examples: [
              {
                title: translated({ en: 'GET mailbox count summary', ko: 'GET 메일함 카운트 요약', ja: 'GET メールボックス件数サマリー', 'zh-CN': 'GET 邮箱数量摘要' }),
                language: 'bash',
                code: `curl -sS 'https://mail.example.com/api/v1/mailbox/overview' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111' \\
  -H 'Accept: application/json'`,
              },
              {
                title: translated({ en: 'Mailbox summary response', ko: '메일함 요약 응답', ja: 'メールボックスサマリー応答', 'zh-CN': '邮箱摘要响应' }),
                language: 'json',
                code: `{
  "mailbox_overview": {
    "total_messages": 1284,
    "unread_messages": 17,
    "starred_messages": 9,
    "total_size_bytes": 98234122,
    "system_folders": {
      "inbox": "folder-inbox",
      "sent": "folder-sent",
      "drafts": "folder-drafts",
      "trash": "folder-trash"
    }
  }
}`,
              },
              {
                title: translated({ en: 'GET unread inbox list with paging', ko: 'GET 안 읽은 받은편지함 목록과 페이지 이동', ja: 'GET 未読受信箱一覧とページング', 'zh-CN': 'GET 未读收件箱列表与分页' }),
                language: 'bash',
                code: `curl -sS 'https://mail.example.com/api/v1/messages?folder_id=folder-inbox&read=false&limit=20&sort=newest' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111' \\
  -H 'Accept: application/json'`,
              },
              {
                title: translated({ en: 'Message list response', ko: '메시지 목록 응답', ja: 'メッセージ一覧応答', 'zh-CN': '邮件列表响应' }),
                language: 'json',
                code: `{
  "messages": [
    {
      "id": "msg-01HZY...",
      "folder_id": "folder-inbox",
      "subject": "Quarterly invoice",
      "from_addr": "billing@example.net",
      "from_name": "Example Billing",
      "received_at": "2026-05-16T03:15:00Z",
      "read": false,
      "starred": false,
      "has_attachment": true
    }
  ],
  "limit": 20,
  "has_more": true,
  "next_cursor": "eyJyZWNlaXZlZF9hdCI6..."
}`,
              },
            ],
          },
          {
            title: translated({ en: 'GET parameters', ko: 'GET 파라미터', ja: 'GET パラメーター', 'zh-CN': 'GET 参数' }),
            body: translated({
              en: 'GET calls use query parameters for filtering and pagination. limit controls page size and is capped by the server. cursor is the opaque next_cursor returned by the previous response. folder_id narrows the list to one folder. read=false is the usual unread-mail filter. sort accepts newest or oldest on message lists, while search uses date or relevance.',
              ko: 'GET 호출은 필터와 페이지 이동에 쿼리 파라미터를 사용합니다. limit는 페이지 크기이며 서버 제한을 넘을 수 없습니다. cursor는 이전 응답의 next_cursor를 그대로 넣는 불투명 값입니다. folder_id는 특정 폴더로 목록을 좁힙니다. read=false는 안 읽은 메일 필터입니다. 메시지 목록의 sort는 newest 또는 oldest를 사용하고, 검색은 date 또는 relevance를 사용합니다.',
              ja: 'GET 呼び出しはフィルターとページングにクエリパラメーターを使います。limit はページサイズで、サーバー上限を超えられません。cursor は前回応答の next_cursor をそのまま渡す不透明値です。folder_id は 1 つのフォルダーに絞ります。read=false は未読フィルターです。メッセージ一覧の sort は newest または oldest、検索は date または relevance を使います。',
              'zh-CN': 'GET 调用使用查询参数进行过滤和分页。limit 控制页大小并受服务器上限限制。cursor 是上一响应返回的 next_cursor，应原样传回。folder_id 将列表限制到一个文件夹。read=false 是常用的未读过滤器。邮件列表 sort 使用 newest 或 oldest，搜索使用 date 或 relevance。',
            }),
            paragraphs: [
              translated({
                en: 'Do not parse cursor contents. Treat it as a token owned by GoGoMail. When has_more is false or next_cursor is empty, stop requesting the next page.',
                ko: 'cursor 내부 값을 해석하지 마세요. GoGoMail이 소유한 토큰으로 보고 그대로 전달합니다. has_more가 false이거나 next_cursor가 비어 있으면 다음 페이지 호출을 멈춥니다.',
                ja: 'cursor の中身を解析しないでください。GoGoMail が所有するトークンとしてそのまま渡します。has_more が false、または next_cursor が空なら次ページ要求を止めます。',
                'zh-CN': '不要解析 cursor 内容。它是 GoGoMail 拥有的令牌，应原样传递。当 has_more 为 false 或 next_cursor 为空时停止请求下一页。',
              }),
            ],
            items: ['limit', 'cursor', 'folder_id', 'read', 'starred', 'has_attachment', 'sort', 'q', 'from', 'to', 'subject', 'since', 'until'],
            examples: [
              {
                title: translated({ en: 'GET search with filters', ko: 'GET 필터 검색', ja: 'GET フィルター検索', 'zh-CN': 'GET 带过滤器搜索' }),
                language: 'bash',
                code: `curl -sS 'https://mail.example.com/api/v1/search?q=invoice&from=billing%40example.net&has_attachment=true&limit=10&sort=date' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111' \\
  -H 'Accept: application/json'`,
              },
            ],
          },
          {
            title: translated({ en: 'POST examples', ko: 'POST 예시', ja: 'POST 例', 'zh-CN': 'POST 示例' }),
            body: translated({
              en: 'POST calls create or trigger work. For external mail composition, use POST /api/v1/messages/send. The request body must identify the user, at least one recipient, and message content. Use text_body for plain text, html_body for HTML, or both when the external system can produce both forms.',
              ko: 'POST 호출은 새 리소스를 만들거나 작업을 실행합니다. 외부 시스템에서 메일을 작성해 보내려면 POST /api/v1/messages/send를 사용합니다. 요청 본문에는 사용자, 최소 한 명의 수신자, 메시지 내용이 필요합니다. 일반 텍스트는 text_body를 사용하고, HTML 본문은 html_body를 사용하며, 둘 다 만들 수 있으면 둘 다 보낼 수 있습니다.',
              ja: 'POST 呼び出しはリソース作成または処理実行に使います。外部システムからメールを作成して送る場合は POST /api/v1/messages/send を使います。本文にはユーザー、少なくとも 1 件の宛先、メッセージ内容が必要です。プレーンテキストは text_body、HTML は html_body を使い、両方生成できる場合は両方送れます。',
              'zh-CN': 'POST 调用用于创建资源或触发操作。外部系统发送邮件时使用 POST /api/v1/messages/send。请求体必须包含用户、至少一个收件人和邮件内容。纯文本使用 text_body，HTML 正文使用 html_body；如果外部系统能同时生成两种格式，也可以同时发送。',
            }),
            paragraphs: [
              translated({
                en: 'Use mail:send for sending. If the external system needs to create drafts, upload attachments, or create folders, issue a key with mail:manage instead of overloading a send-only key.',
                ko: '메일 전송에는 mail:send를 사용합니다. 외부 시스템이 임시보관함 생성, 첨부파일 업로드, 폴더 생성까지 해야 한다면 전송 전용 키를 넓히지 말고 mail:manage 권한의 키를 발급합니다.',
                ja: '送信には mail:send を使います。外部システムが下書き作成、添付アップロード、フォルダー作成も必要な場合は、送信専用キーを広げず mail:manage のキーを発行します。',
                'zh-CN': '发送邮件使用 mail:send。如果外部系统还需要创建草稿、上传附件或创建文件夹，应签发 mail:manage 权限的密钥，而不是扩大仅发送密钥的用途。',
              }),
            ],
            items: ['/api/v1/messages/send', 'user_id', 'to', 'cc', 'bcc', 'subject', 'text_body', 'html_body', 'attachment_ids'],
            examples: [
              {
                title: translated({ en: 'POST send message', ko: 'POST 메일 전송', ja: 'POST メール送信', 'zh-CN': 'POST 发送邮件' }),
                language: 'bash',
                code: `curl -sS -X POST 'https://mail.example.com/api/v1/messages/send' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "user_id": "11111111-1111-1111-1111-111111111111",
    "to": [{"email": "customer@example.net", "name": "Customer"}],
    "cc": [],
    "bcc": [],
    "subject": "Your requested document",
    "text_body": "Hello, the requested document is attached.",
    "html_body": "<p>Hello, the requested document is attached.</p>",
    "attachment_ids": []
  }'`,
              },
              {
                title: translated({ en: 'Send response', ko: '전송 응답', ja: '送信応答', 'zh-CN': '发送响应' }),
                language: 'json',
                code: `{
  "message": {
    "id": "msg-01HZZA...",
    "message_id": "<msg-01HZZA@example.com>",
    "send_status": "queued",
    "delivery_status": "pending"
  }
}`,
              },
            ],
          },
          {
            title: translated({ en: 'PUT and update examples', ko: 'PUT 및 수정 예시', ja: 'PUT と更新例', 'zh-CN': 'PUT 与更新示例' }),
            body: translated({
              en: 'PUT replaces a complete setting document. For example, PUT /api/v1/preferences stores the webmail preference object for the target user. Message state changes use PATCH because only one field or a bounded set of fields changes.',
              ko: 'PUT은 설정 문서 전체를 교체할 때 사용합니다. 예를 들어 PUT /api/v1/preferences는 대상 사용자의 웹메일 환경설정 객체를 저장합니다. 메시지 상태 변경은 일부 필드만 바꾸므로 PATCH를 사용합니다.',
              ja: 'PUT は設定ドキュメント全体を置き換えるときに使います。たとえば PUT /api/v1/preferences は対象ユーザーのウェブメール設定オブジェクトを保存します。メッセージ状態の変更は一部フィールドだけを変えるため PATCH を使います。',
              'zh-CN': 'PUT 用于替换完整设置文档。例如 PUT /api/v1/preferences 会存储目标用户的 Webmail 偏好对象。邮件状态变更只修改部分字段，因此使用 PATCH。',
            }),
            paragraphs: [
              translated({
                en: 'Use mail:manage for preferences, folder changes, message flags, moves, delete, restore, attachment upload, and push device registration.',
                ko: '환경설정, 폴더 변경, 메시지 플래그, 이동, 삭제, 복원, 첨부파일 업로드, 푸시 기기 등록에는 mail:manage를 사용합니다.',
                ja: '設定、フォルダー変更、メッセージフラグ、移動、削除、復元、添付アップロード、プッシュデバイス登録には mail:manage を使います。',
                'zh-CN': '偏好设置、文件夹变更、邮件标记、移动、删除、恢复、附件上传和推送设备注册使用 mail:manage。',
              }),
            ],
            items: ['/api/v1/preferences', '/api/v1/messages/{id}/flags', '/api/v1/messages/{id}/folder'],
            examples: [
              {
                title: translated({ en: 'PUT replace user preferences', ko: 'PUT 사용자 환경설정 교체', ja: 'PUT ユーザー設定置換', 'zh-CN': 'PUT 替换用户偏好' }),
                language: 'bash',
                code: `curl -sS -X PUT 'https://mail.example.com/api/v1/preferences' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "theme": "dark",
    "language": "ko",
    "density": "comfortable"
  }'`,
              },
              {
                title: translated({ en: 'PATCH mark a message as read', ko: 'PATCH 메시지를 읽음으로 표시', ja: 'PATCH メッセージを既読化', 'zh-CN': 'PATCH 标记邮件为已读' }),
                language: 'bash',
                code: `curl -sS -X PATCH 'https://mail.example.com/api/v1/messages/msg-01HZY/flags' \\
  -H 'Authorization: Bearer gm_REPLACE_WITH_ISSUED_KEY' \\
  -H 'X-Gogomail-User-ID: 11111111-1111-1111-1111-111111111111' \\
  -H 'Content-Type: application/json' \\
  -d '{"flag":"read","value":true}'`,
              },
            ],
          },
          {
            title: translated({ en: 'Error handling', ko: '오류 처리', ja: 'エラー処理', 'zh-CN': '错误处理' }),
            body: translated({
              en: 'External systems should handle HTTP status codes first and the JSON error message second. 400 means the request shape or parameter is invalid. 401 means the API key is missing, malformed, expired, revoked, or not recognized. 403 means the key is valid but lacks the required scope or does not belong to the target user domain. 404 means the requested message, folder, attachment, or draft was not found for that user.',
              ko: '외부 시스템은 먼저 HTTP 상태 코드를 보고, 그 다음 JSON 오류 메시지를 처리해야 합니다. 400은 요청 형식이나 파라미터가 잘못된 경우입니다. 401은 API 키가 없거나 형식이 틀렸거나 만료, 폐기, 미등록 상태인 경우입니다. 403은 키는 유효하지만 필요한 권한 범위가 없거나 대상 사용자의 도메인과 맞지 않는 경우입니다. 404는 해당 사용자 기준으로 메시지, 폴더, 첨부파일, 임시보관 메일을 찾지 못한 경우입니다.',
              ja: '外部システムはまず HTTP ステータスコード、次に JSON エラーメッセージを処理します。400 は要求形状またはパラメーター不正、401 は API キーの欠落、形式不正、期限切れ、失効、未登録、403 はキーは有効だが必要スコープ不足または対象ユーザードメイン不一致、404 はそのユーザーのメッセージ、フォルダー、添付、下書きが見つからない状態です。',
              'zh-CN': '外部系统应先处理 HTTP 状态码，再处理 JSON 错误消息。400 表示请求结构或参数无效。401 表示 API 密钥缺失、格式错误、过期、已撤销或无法识别。403 表示密钥有效但缺少所需范围，或不属于目标用户域名。404 表示在该用户范围内找不到请求的邮件、文件夹、附件或草稿。',
            }),
            paragraphs: [
              translated({
                en: 'Retry only transient failures such as 429, 502, 503, or 504. Do not retry 400, 401, or 403 without changing the request, key, scope, or target user.',
                ko: '재시도는 429, 502, 503, 504처럼 일시적인 실패에만 적용합니다. 400, 401, 403은 요청, 키, 권한 범위, 대상 사용자를 바꾸지 않고 반복 호출하지 않습니다.',
                ja: '再試行は 429、502、503、504 のような一時的失敗だけに適用します。400、401、403 は要求、キー、スコープ、対象ユーザーを変えずに再試行しません。',
                'zh-CN': '仅对 429、502、503 或 504 等临时失败进行重试。不要在未修改请求、密钥、范围或目标用户的情况下重试 400、401 或 403。',
              }),
            ],
            examples: [
              {
                title: translated({ en: 'Error response shape', ko: '오류 응답 형식', ja: 'エラー応答形式', 'zh-CN': '错误响应格式' }),
                language: 'json',
                code: `{
  "error": "api key scope mail:read is required"
}`,
              },
            ],
          },
          {
            title: translated({ en: 'Metering and operations', ko: '미터링과 운영 확인', ja: 'メータリングと運用確認', 'zh-CN': '计量与运营确认' }),
            body: translated({
              en: 'Public API calls are metered through the existing API metering pipeline. Valid API-key requests are recorded with auth_source api_key, the API key identifier, the domain identifier, the target user, method, route, status, bytes, and latency.',
              ko: '외부 공개 API 호출은 기존 API 미터링 파이프라인으로 기록됩니다. 유효한 API 키 요청은 auth_source api_key, API 키 식별자, 도메인 식별자, 대상 사용자, 메서드, 라우트, 상태 코드, 바이트 수, 지연 시간과 함께 남습니다.',
              ja: '外部公開 API 呼び出しは既存の API メータリングパイプラインで記録されます。有効な API キー要求は auth_source api_key、API キー ID、ドメイン ID、対象ユーザー、メソッド、ルート、状態コード、バイト数、レイテンシと共に残ります。',
              'zh-CN': '外部公开 API 调用通过现有 API 计量管道记录。有效的 API 密钥请求会记录 auth_source api_key、API 密钥标识、域名标识、目标用户、方法、路由、状态码、字节数和延迟。',
            }),
            paragraphs: [
              translated({
                en: 'Administrators should review API usage together with API keys and API settings. If an external system suddenly sends too many requests, use API usage first, then narrow the key, rotate it, or adjust rate limits and CIDR access controls.',
                ko: '관리자는 API 사용량을 API 키와 API 설정과 함께 봐야 합니다. 외부 시스템 호출량이 갑자기 늘면 먼저 API 사용량을 확인하고, 필요하면 키 권한을 좁히거나 키를 회전하거나 발신 제한과 CIDR 접근 제어를 조정합니다.',
                ja: '管理者は API 使用量を API キーと API 設定と一緒に確認します。外部システム呼び出しが急増した場合は API 使用量を先に確認し、必要に応じてキー権限を狭め、ローテートし、レート制限や CIDR アクセス制御を調整します。',
                'zh-CN': '管理员应将 API 使用量与 API 密钥、API 设置一起查看。如果外部系统请求突然增加，先查看 API 使用量，再按需缩小密钥权限、轮换密钥，或调整速率限制和 CIDR 访问控制。',
              }),
            ],
            items: [console.nav.api_usage, console.nav.api_keys, console.nav.api_settings, console.nav.rate_limits, console.nav.ip_access],
          },
        ],
      },
      adminOperations: {
        eyebrow: adminConsole,
        title: f.operationsTitle,
        lead: f.operationsLead,
        sections: [
          { title: console.nav.message_trace, body: screenBody(console.nav.message_trace), items: [console.nav.mail_flow_logs, console.nav.delivery_attempts] },
          { title: console.nav.mail_flow_logs, body: screenBody(console.nav.mail_flow_logs), items: [console.nav.message_trace, console.nav.audit_logs] },
          { title: console.nav.outbox_events, body: screenBody(console.nav.outbox_events), items: [console.nav.delivery_attempts, console.nav.queue_stats] },
          { title: console.nav.delivery_attempts, body: screenBody(console.nav.delivery_attempts), items: [console.nav.trusted_relays, console.nav.delivery_routes] },
          { title: console.nav.routing_rules, body: screenBody(console.nav.routing_rules), items: [console.nav.delivery_routes, console.nav.smtp_policy] },
          { title: console.nav.delivery_routes, body: screenBody(console.nav.delivery_routes), items: [console.nav.trusted_relays, console.nav.routing_rules] },
          { title: console.nav.trusted_relays, body: screenBody(console.nav.trusted_relays), items: [console.nav.delivery_routes, console.nav.smtp_policy] },
          { title: console.nav.queue_stats, body: screenBody(console.nav.queue_stats), items: [console.nav.backpressure, console.nav.api_health] },
          { title: console.nav.backpressure, body: screenBody(console.nav.backpressure), items: [console.nav.queue_stats, console.nav.api_health] },
          { title: console.nav.api_health, body: screenBody(console.nav.api_health), items: [console.nav.dashboard, console.common.refresh] },
        ],
      },
      adminAccess: {
        eyebrow: adminConsole,
        title: f.accessTitle,
        lead: f.accessLead,
        sections: [
          { title: console.nav.directory, body: screenBody(console.nav.directory), items: [console.nav.users, console.nav.group_memberships, console.nav.aliases] },
          { title: console.nav.aliases, body: screenBody(console.nav.aliases), items: [userPage.offboard_alias_prefix as string, userPage.access_aliases as string] },
          { title: console.nav.delegations, body: screenBody(console.nav.delegations), items: [console.nav.directory, console.nav.users] },
          { title: console.nav.group_memberships, body: screenBody(console.nav.group_memberships), items: [console.nav.directory, console.nav.roles] },
          { title: console.nav.roles, body: screenBody(console.nav.roles), items: [userPage.role_company_admin as string, userPage.role_system_admin as string, adminUsersPage.read_only as string] },
        ],
      },
      adminGovernance: {
        eyebrow: adminConsole,
        title: f.governanceTitle,
        lead: f.governanceLead,
        sections: [
          { title: console.nav.audit_logs, body: screenBody(console.nav.audit_logs), items: [console.nav.admin_activity, console.nav.reports] },
          { title: console.nav.admin_activity, body: screenBody(console.nav.admin_activity), items: [console.nav.audit_logs, console.nav.change_history] },
          { title: console.nav.alert_rules, body: screenBody(console.nav.alert_rules), items: [console.nav.webhooks, console.nav.notif_templates] },
          { title: console.nav.suppression_list, body: screenBody(console.nav.suppression_list), items: [console.nav.outbox_events, console.nav.delivery_attempts] },
          { title: console.nav.dkim_keys, body: screenBody(console.nav.dkim_keys), items: [console.nav.domains, console.nav.dmarc_spf] },
          { title: console.nav.api_keys, body: screenBody(console.nav.api_keys), items: [console.nav.api_settings, console.nav.api_usage] },
          { title: console.nav.api_settings, body: screenBody(console.nav.api_settings), items: [console.nav.api_keys, console.nav.rate_limits] },
          { title: console.nav.mfa_management, body: screenBody(console.nav.mfa_management), items: [console.nav.users, console.nav.auth_policy] },
          { title: console.nav.ip_access, body: screenBody(console.nav.ip_access), items: [console.nav.auth_policy, console.nav.session_mgmt] },
          { title: console.nav.auth_policy, body: screenBody(console.nav.auth_policy), items: [console.nav.mfa_management, console.nav.session_mgmt] },
          { title: console.nav.audit_policy, body: screenBody(console.nav.audit_policy), items: [console.nav.audit_logs, console.nav.retention_policy] },
          { title: console.nav.retention_policy, body: screenBody(console.nav.retention_policy), items: [console.nav.audit_policy, console.nav.legal_holds] },
          { title: console.nav.session_mgmt, body: screenBody(console.nav.session_mgmt), items: [console.nav.auth_policy, console.nav.ip_access] },
          { title: console.nav.rate_limits, body: screenBody(console.nav.rate_limits), items: [console.nav.smtp_policy, console.nav.api_settings] },
          { title: console.nav.dmarc_spf, body: screenBody(console.nav.dmarc_spf), items: [console.nav.domains, console.nav.dkim_keys] },
          { title: console.nav.spam_filter, body: screenBody(console.nav.spam_filter), items: [console.nav.smtp_policy, console.nav.suppression_list] },
          { title: console.nav.smtp_policy, body: screenBody(console.nav.smtp_policy), items: [console.nav.trusted_relays, console.nav.dmarc_spf] },
          { title: console.nav.security_posture, body: screenBody(console.nav.security_posture), items: [console.nav.domains, console.nav.dmarc_spf, console.nav.dkim_keys] },
          { title: console.nav.compliance, body: screenBody(console.nav.compliance), items: [console.nav.audit_logs, console.nav.legal_holds] },
          { title: console.nav.legal_holds, body: screenBody(console.nav.legal_holds), items: [console.nav.retention_policy, console.nav.compliance] },
        ],
      },
      adminAnalyticsStorage: {
        eyebrow: adminConsole,
        title: f.analyticsTitle,
        lead: f.analyticsLead,
        sections: [
          { title: console.nav.quota_dashboard, body: screenBody(console.nav.quota_dashboard), items: [console.nav.quota_usage, console.nav.quota_alerts] },
          { title: console.nav.quota_usage, body: screenBody(console.nav.quota_usage), items: [console.nav.users, console.nav.domains] },
          { title: console.nav.quota_alerts, body: screenBody(console.nav.quota_alerts), items: [console.nav.quota_dashboard, console.nav.reports] },
          { title: console.nav.attachments, body: screenBody(console.nav.attachments), items: [webmail.mail.attachments, console.nav.drive] },
          { title: console.nav.drive, body: screenBody(console.nav.drive), items: [console.nav.attachments, console.nav.quota_usage] },
          { title: console.nav.seat_usage, body: screenBody(console.nav.seat_usage), items: [console.nav.users, console.nav.domains] },
          { title: console.nav.api_usage, body: screenBody(console.nav.api_usage), items: [console.nav.api_keys, console.nav.api_settings] },
          { title: console.nav.push_notifications, body: screenBody(console.nav.push_notifications), items: [console.nav.alert_rules, console.nav.reports] },
          { title: console.nav.reports, body: c.adminReportBody, items: [console.nav.compliance, console.nav.audit_logs, console.nav.users, console.nav.domains] },
        ],
      },
      webmailOverview: {
        eyebrow: webmailName,
        title: webmailName,
        lead: c.webmailLead,
        primaryCta: { text: c.webmailStart, link: link(locale, '/webmail/getting-started') },
        sections: [
          {
            title: c.webmailShellTitle,
            body: c.webmailShellBody,
            items: [webmail.mail.compose, webmail.settings.theme, webmail.settings.language, webmail.common.search],
          },
          {
            title: webmail.mail.folders,
            body: c.webmailFoldersBody,
            items: [
              webmail.mail.inbox,
              webmail.mail.sent,
              webmail.mail.drafts,
              webmail.mail.trash,
              webmail.mail.spam,
              webmail.mail.archive,
              webmail.mail.folders,
              webmail.mail.unread,
            ],
          },
          {
            title: webmail.mail.selectMessage,
            body: c.webmailReadingBody,
            items: [
              webmail.mail.from,
              webmail.mail.to,
              webmail.mail.subject,
              webmail.mail.attachments,
              webmail.mail.reply,
              webmail.mail.replyAll,
              webmail.mail.forward,
              webmail.mail.deleteMessage,
              webmail.mail.markUnread,
            ],
          },
          {
            title: webmail.compose.newMessage,
            body: c.webmailComposeBody,
            items: [webmail.compose.to, webmail.compose.subject, webmail.compose.bodyPlaceholder, webmail.compose.send, webmail.compose.sending],
          },
          {
            title: webmail.compose.insertLink,
            body: c.webmailComposeToolsBody,
            items: [
              webmail.compose.bold,
              webmail.compose.italic,
              webmail.compose.underline,
              webmail.compose.bulletList,
              webmail.compose.insertLink,
              webmail.compose.undo,
            ],
          },
          {
            title: webmail.settings.theme,
            body: c.webmailSettingsBody,
            items: [webmail.settings.lightMode, webmail.settings.darkMode, webmail.settings.language],
          },
          {
            title: c.factualLimitTitle,
            body: c.factualLimitBody,
            items: [webmail.common.loading, webmail.common.error, webmail.common.retry],
          },
        ],
      },
      webmailGettingStarted: {
        eyebrow: webmailName,
        title: c.webmailStart,
        lead: f.startWebmailLead,
        sections: [
          {
            title: c.loginTitle,
            body: c.webmailLoginBody,
            items: [webmail.auth.email, webmail.auth.password, webmail.auth.loginButton, webmail.auth.emptyFieldsError],
          },
          {
            title: c.mailboxTitle,
            body: c.webmailMailboxBody,
            items: [
              webmail.mail.inbox,
              webmail.mail.noMessages,
              webmail.common.loading,
            ],
          },
          {
            title: c.composeTitle,
            body: c.webmailSendBody,
            items: [webmail.compose.toRequired, webmail.compose.bodyRequired, webmail.compose.send, webmail.compose.sendFailed],
          },
        ],
      },
      webmailMail: {
        eyebrow: webmailName,
        title: f.webmailMailTitle,
        lead: f.webmailMailLead,
        sections: [
          { title: webmail.mail.folders, body: c.webmailFoldersBody, items: [webmail.mail.inbox, webmail.mail.sent, webmail.mail.drafts, webmail.mail.trash, webmail.mail.spam, webmail.mail.archive, webmail.mail.folders] },
          { title: webmail.mail.selectMessage, body: c.webmailReadingBody, items: [webmail.mail.from, webmail.mail.to, webmail.mail.subject, webmail.mail.date, webmail.mail.noSubject] },
          { title: webmail.mail.attachments, body: c.webmailReadingBody, items: [webmail.mail.attachments, console.nav.drive, webmail.common.error, webmail.common.retry] },
          { title: webmail.mail.unread, body: c.webmailFoldersBody, items: [webmail.mail.unread, webmail.mail.markUnread, webmail.mail.noMessages] },
          { title: webmail.common.search, body: c.webmailMailboxBody, items: [webmail.common.search, webmail.common.noResults, webmail.common.loading] },
          { title: webmail.mail.reply, body: c.webmailReadingBody, items: [webmail.mail.reply, webmail.mail.replyAll, webmail.mail.forward, webmail.mail.deleteMessage] },
        ],
      },
      webmailCompose: {
        eyebrow: webmailName,
        title: f.webmailComposeTitle,
        lead: f.webmailComposeLead,
        sections: [
          { title: webmail.compose.newMessage, body: c.webmailComposeBody, items: [webmail.compose.to, webmail.compose.subject, webmail.compose.bodyPlaceholder, webmail.compose.send] },
          { title: webmail.compose.to, body: c.webmailSendBody, items: [webmail.compose.toPadding, webmail.compose.toRequired, webmail.mail.to] },
          { title: webmail.compose.subject, body: c.webmailSendBody, items: [webmail.compose.subjectPlaceholder, webmail.compose.bodyRequired, webmail.mail.noSubject] },
          { title: webmail.compose.send, body: c.webmailSendBody, items: [webmail.compose.sending, webmail.compose.sent, webmail.compose.sendFailed] },
          { title: webmail.compose.insertLink, body: c.webmailComposeToolsBody, items: [webmail.compose.bold, webmail.compose.italic, webmail.compose.underline, webmail.compose.bulletList, webmail.compose.insertLink, webmail.compose.undo, webmail.compose.linkPrompt] },
          { title: webmail.mail.attachments, body: c.webmailComposeBody, items: [webmail.mail.attachments, console.nav.drive, webmail.common.error] },
        ],
      },
      webmailSettings: {
        eyebrow: webmailName,
        title: f.webmailSettingsTitle,
        lead: f.webmailSettingsLead,
        sections: [
          { title: webmail.settings.theme, body: c.webmailSettingsBody, items: [webmail.settings.lightMode, webmail.settings.darkMode, ...settingCategories] },
          { title: webmail.settings.language, body: c.webmailSettingsStartBody, items: [webmail.settings.korean, webmail.settings.english, webmail.settings.japanese, webmail.settings.chineseSimplified] },
          { title: settingCategories[0], body: c.webmailMailboxBody, items: settingItems.mailbox },
          { title: settingCategories[1], body: c.webmailComposeBody, items: settingItems.compose },
          { title: settingCategories[2], body: c.webmailFoldersBody, items: settingItems.filters },
          { title: settingCategories[3], body: c.webmailSettingsBody, items: settingItems.theme },
          { title: settingCategories[4], body: c.webmailSettingsBody, items: settingItems.notifications },
          { title: settingCategories[5], body: c.webmailLoginBody, items: [webmail.auth.email, webmail.auth.logout] },
          { title: settingCategories[6], body: c.webmailSettingsBody, items: settingItems.security },
          { title: settingCategories[7], body: f.webmailShortcutsLead, items: settingsViewSections },
          { title: settingCategories[8], body: c.factualLimitBody, items: settingItems.advanced },
        ],
      },
      webmailApps: {
        eyebrow: webmailName,
        title: f.webmailAppsTitle,
        lead: f.webmailAppsLead,
        sections: [
          { title: 'Mail', body: c.webmailShellBody, items: [webmail.mail.inbox, webmail.mail.compose, webmail.common.search] },
          { title: 'Calendar', body: f.sourceNote, items: ['day', 'week', 'month', 'MiniCalendar', 'Calendar management'] },
          { title: 'Contacts', body: f.sourceNote, items: [webmail.auth.email, webmail.compose.newMessage, webmail.common.search] },
          { title: 'Organization', body: f.sourceNote, items: [console.nav.organization, console.nav.directory, webmail.compose.to] },
          { title: 'Drive', body: f.sourceNote, items: [console.nav.drive, webmail.mail.attachments, webmail.common.loading] },
          { title: webmail.settings.theme, body: c.webmailSettingsBody, items: [webmail.settings.theme, webmail.settings.language] },
        ],
      },
      webmailShortcuts: {
        eyebrow: webmailName,
        title: f.webmailShortcutsTitle,
        lead: f.webmailShortcutsLead,
        sections: [
          { title: 'Global', body: f.sourceNote, items: ['?', 'Cmd+K / Ctrl+K', '/', '['] },
          { title: 'App switch', body: f.sourceNote, items: ['g m', 'g c', 'g k', 'g o', 'g v', 'g ,'] },
          { title: 'Mail navigation', body: f.sourceNote, items: ['j / k', 'Enter / o', 'x', 'Ctrl+A', 'Esc'] },
          { title: 'Mail actions', body: f.sourceNote, items: ['r', 'a', 'f', 'e', 'v', '#', 's', 'm', 'Shift+M', 'z', 'l', '!'] },
          { title: webmail.mail.folders, body: f.sourceNote, items: ['g i', 'g s', 'g d', 'g t', 'g p'] },
          { title: webmail.compose.newMessage, body: f.sourceNote, items: ['c', 'Ctrl+Enter', 'Ctrl+S', 'Esc'] },
        ],
      },
    }),
  };
}

export const docsMessages = Object.fromEntries(
  localeCodes.map(locale => [locale, makeMessages(locale)])
) as Record<DocsLocale, DocsMessages>;
