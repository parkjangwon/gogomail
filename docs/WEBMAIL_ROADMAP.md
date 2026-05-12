# gogomail webmail frontend roadmap

**Product Direction**: Notion Mail / Linear / Superhuman inspired, but original. Clean minimalist UI on the surface, packed with enterprise-grade power features underneath — so users never need to ask for more.

**Target**: Korean market primarily, global enterprise use. Original implementation only. Next.js 15 + React 19 + Tailwind v4 + TipTap v2, port 3003.

**Current MVP**: Login + 3-pane layout (Sidebar / MessageList / ReadingPane) + TipTap compose modal.

## Development verification loop

For webmail beta work, run the frontend checks alongside the repository Go test
suite before committing:

- `go test ./...` from the repository root.
- `pnpm type-check` from `apps/webmail`.
- Use `pnpm test:compose-helpers` as the canonical command from `apps/webmail`
  when compose helper copy,
  scheduled-send datetime formatting, send button labels, or close-save prompts
  are touched.
- `pnpm test:datetime-local` remains as a compatibility alias for the same
  compose-helper runtime checks because the datetime-local formatter was the
  first helper covered by this script.

---

## Phase 1: Core Mail (MVP+)

### 1.1 Keyboard Shortcuts & Command Palette

**Description**: Discoverable, customizable keyboard navigation. Every action has a shortcut. `?` key shows cheat sheet overlay.

**Features**:
- Global keyboard listener with conflict detection
- Shortcut profiles: Gmail-style, Superhuman-style, custom
- Built-in shortcuts:
  - `J` / `K`: navigate previous/next message
  - `C`: compose new mail
  - `R`: reply to current message
  - `A`: reply-all
  - `F`: forward
  - `E`: archive current message
  - `#`: delete to trash
  - `U`: mark unread
  - `*`: star/unstar
  - `[` / `]`: navigate previous/next folder
  - `/`: focus search bar
  - `?`: show shortcut cheat sheet
  - `Ctrl+Enter` / `Cmd+Enter`: send (compose modal)
  - `Escape`: close compose modal, exit search
  - `V`: move to folder (opens folder picker)
- Cheat sheet modal: grid layout, grouped by category (Navigation / Actions / Compose), searchable, keyboard-navigable
- Conflict warnings: alert if user shortcut overlaps with system/browser shortcuts
- Custom shortcut editor in Settings (Phase 3)

**Implementation Notes**:
- Use a keyboard event hook library (e.g., `react-hotkeys-hook` or custom listener)
- Store shortcut map in context/zustand state
- Cheat sheet component with accessible modal
- Warn on system shortcut collision (e.g., `Cmd+Q` on macOS)

**Complexity**: 🟡 Medium

---

### 1.2 Thread View / Conversation Grouping

**Description**: Group related messages by subject/references into a single conversation. Users see message count per thread in list view.

**Features**:
- Auto-group messages with same subject prefix and In-Reply-To/References headers
- Thread list shows: sender, subject, preview of latest message, thread message count, date of latest
- Reading pane expands to show all messages in thread, newest at bottom
- Collapse/expand individual messages within thread
- Mark entire thread as read/unread with single action
- Star/unstar whole thread
- Move/delete actions apply to thread or single message (toggle)
- Thread participants shown as avatars at top of reading pane
- "Expand all" / "Collapse all" buttons in thread header

**Implementation Notes**:
- Backend provides `thread_id` in message list responses (Phase 1 backend task)
- MessageList component groups by thread_id
- ReadingPane thread view iterates messages in thread, renders each message header/body
- Collapse state per message stored in local component state or localStorage
- Thread header shows unique sender avatars/names from thread participants

**Complexity**: 🟡 Medium

---

### 1.3 Reply / Reply-All / Forward with Quoted Content

**Description**: Full quote preservation with original sender/date/time. Quoted content editable via TipTap.

**Features**:
- Reply mode: quotes sender only
- Reply-All mode: quotes all original recipients (To/Cc, excluding self)
- Forward mode: original subject prefixed with "Fwd: ", full message quoted
- Quoted content inserted as blockquote in TipTap (with gray background, left border)
- Quote shows original sender, timestamp, subject as header
- Users can delete/edit quoted sections before sending
- Quoted images/attachments shown as references (not re-uploaded)
- Forward allows adding new recipients before sending
- Compose modal remembers mode (reply/reply-all/forward) until sent or closed

**Implementation Notes**:
- Message detail view has "Reply", "Reply-All", "Forward" buttons
- Clicking opens compose modal with appropriate initial state
- Quote formatter: parse message HTML/plain-text, wrap in `<blockquote>`, add attribution line
- TipTap can render quoted content as read-only or editable blocks
- Store `reply_to_message_id`, `forward_from_message_id` in draft metadata
- On send, backend marks source message as `answered` or `forwarded`

**Complexity**: 🟡 Medium

---

### 1.4 Draft Autosave (3-second interval)

**Description**: Save compose drafts automatically every 3 seconds to backend. Show save indicator (dot animation).

**Features**:
- Compose modal shows "Saving..." indicator during autosave
- Debounced autosave: waits 3 seconds after last keystroke
- Saves: to/cc/bcc, subject, body (TipTap HTML), attachments metadata
- Draft persists across browser close/refresh
- Return to draft by clicking on Drafts folder item or reopening compose
- Prevents data loss on accidental page close (browser "Are you sure?" dialog if unsaved changes)
- Discard draft button in compose modal
- Success indicator: subtle green checkmark briefly appears

**Implementation Notes**:
- Use React Query `useMutation` for POST /api/v1/drafts/{id} updates
- Debounce with `useDeferredValue` or custom hook (300ms delay before API call)
- Show saving state in modal header
- BeforeUnloadEvent listener warns if draft has unsaved changes
- TipTap onChange triggers autosave debounce
- Attachment uploads include draft_id metadata (backend task)

**Complexity**: 🟢 Easy

---

### 1.5 Attachment Upload & Download

**Description**: Drag-and-drop file upload to compose, inline preview, download from reading pane.

**Features**:
- Drag-and-drop zone in compose modal (hover shows overlay)
- File input button as fallback
- Upload progress bar per file (% uploaded)
- Show file name, size, MIME type as pills
- Remove button per attachment (x button)
- Size warnings (over 25MB shows caution, over limit shows error)
- Inline preview for images in compose (thumbnail, 120px)
- Attachment list in reading pane shows: icon (by MIME type), name, size, download button
- Download opens new tab or saves to device (native download)
- Animated checkmark after successful upload

**Implementation Notes**:
- Compose modal has drop zone with `onDrop` / `onDragOver` handlers
- FormData with file array, POST /api/v1/attachments/upload
- Backend returns upload_id, storage path, MIME type, size
- Progress via fetch/upload events or XMLHttpRequest
- Image preview: generate data URL with FileReader
- Reading pane attachment list maps over message.attachments array
- Download: navigate to /api/v1/attachments/{id}/download (backend routes files)

**Complexity**: 🟡 Medium

---

### 1.6 Infinite Scroll / Pagination for Message List

**Description**: Load more messages as user scrolls down. Smooth, fast, no visible pagination controls.

**Features**:
- Message list loads initial 20-50 messages
- Scroll sentinel near bottom triggers load next page
- Loading indicator (spinner) while fetching
- Error state with "Retry" button if load fails
- `has_more` boolean in list response determines if more pages exist
- Cursor-based pagination: opaque `next_cursor` from backend
- No jumping or layout shift during load
- Cached pages: React Query handles dedup if user scrolls back up
- Mobile: pull-to-refresh gesture optionally refreshes current page

**Implementation Notes**:
- Use React Query `useInfiniteQuery` with `queryKey: ['messages', folderId]`
- `getNextPageParam: (lastPage) => lastPage.next_cursor if lastPage.has_more`
- Intersection Observer to detect scroll near bottom, trigger `fetchNextPage()`
- Message list renders `pages.flatMap(p => p.messages)`
- Loader component at list bottom shows during `isFetchingNextPage`
- Error state renders within message list, not full-screen error

**Complexity**: 🟡 Medium

---

### 1.7 Mark Read/Unread, Star/Unstar

**Description**: Single-click flag updates. Instant UI feedback, silent background save.

**Features**:
- Message list row shows read/unread indicator (dot, bold subject if unread)
- Star icon (outline / filled) next to sender name
- Mark read/unread via keyboard shortcut, button, or context menu
- Star/unstar via keyboard shortcut or button
- Clicking on unread dot toggles read state instantly
- Read indicator disappears, subject weight changes
- Star fills in, outline disappears
- Batch actions: select multiple messages, mark all read/unread
- Move to trash: soft-delete (not permanent removal, recoverable)
- Folder unread badge updates in real-time

**Implementation Notes**:
- Message component shows read indicator conditionally
- Star button uses icon from icon library (lucide-react, heroicons)
- Click handlers call `useMutation` for PATCH /api/v1/messages/{id}/read, PATCH .../starred
- Optimistic updates: update local state instantly, revert on error
- Folder counts (unread, starred) updated via React Query cache invalidation
- Select state managed by checkbox per message, state in MessageList component

**Complexity**: 🟢 Easy

---

### 1.8 Folder Management (Create, Rename, Delete Custom Folders)

**Description**: User-defined label-like folders for custom organization. Separate from system folders (Inbox, Sent, Drafts, Trash).

**Features**:
- Sidebar shows system folders: Inbox, Sent, Drafts, Trash (always visible, cannot delete)
- "Create folder" button/context menu in sidebar
- Modal or inline input to name new folder
- Rename folder via right-click context menu or settings edit
- Delete folder (move messages to parent or Inbox, offer choice)
- Folder count badge (total messages, not unread)
- Folders can be nested (optional, depends on backend support)
- Reorder folders by drag-and-drop (optional Phase 2)
- Move message to folder: drag-and-drop or "Move" action

**Implementation Notes**:
- Sidebar component maps over `folders` array (from `GET /api/v1/folders`)
- Create folder: POST /api/v1/folders with name, parent_id
- Rename: PATCH /api/v1/folders/{id} with new name
- Delete: DELETE /api/v1/folders/{id} (backend handles message cleanup or return error if not empty)
- Folder selection uses context or zustand state
- MessageList queries by `folderId` param

**Complexity**: 🟡 Medium

---

### 1.9 Search (Full-Text, Instant Results)

**Description**: Search across all messages. Results appear as-you-type with highlighting.

**Features**:
- Search input in sidebar top area (always visible)
- Search applies to sender, subject, body, attachments filename
- Keyboard shortcut `/` to focus search input
- Instant results as user types (debounced 300ms after last keystroke)
- Results shown in message list, replaces current folder view
- Result count shown ("42 results in Inbox and All Mail")
- Highlight matching terms in preview text (bold or highlight color)
- Search operators (optional Phase 2): from:, to:, subject:, has:attachment, before:, after:
- "Clear search" button to return to folder view
- Search history (recent 5 searches, clickable)

**Implementation Notes**:
- Search input controlled component in Sidebar
- POST /api/v1/search with query param, returns paginated results
- Debounce search API calls with `useDeferredValue` or custom hook
- Results treated as pseudo-folder in message list
- Highlight matching terms: split body by regex, wrap matches in `<mark>` tags
- Recent searches: localStorage with search terms
- Mobile: search input takes full width when focused

**Complexity**: 🟡 Medium

---

### 1.10 Empty States (Illustrations / Nice Text)

**Description**: Helpful, non-scary empty states when folders are empty or search has no results.

**Features**:
- Inbox empty: "You're all caught up!" with illustration
- Sent folder empty: "No sent mail yet. Compose your first message." with illustration
- Drafts empty: "No drafts. Start composing." with illustration
- Search no results: "No messages match [query]. Try different keywords." with suggestions
- Trash empty: "Your trash is empty."
- All states include simple SVG illustration (original, minimal art style)
- Text is conversational, not robotic
- Optional link/button to action (e.g., "Compose mail" button in empty Inbox)

**Implementation Notes**:
- EmptyState component with illustration prop, heading, description
- Render in MessageList when `messages.length === 0 && !loading && !error`
- Illustrations: simple SVG, 200x200px, drawn in Figma or code (use lucide-react icons as fallback)
- Heading color: text-primary, description: text-secondary

**Complexity**: 🟢 Easy

---

### 1.11 Mobile Responsive (Collapsed Sidebar, Swipe Gestures)

**Description**: Full mobile experience. Sidebar collapses to icon nav, swipe to navigate.

**Features**:
- Breakpoint: collapse sidebar below 900px width
- Collapsed sidebar shows only icons (inbox, sent, drafts, folders as dots)
- Hamburger menu icon to toggle sidebar open/close
- Message list full-width when sidebar closed
- Reading pane hidden on mobile, tap message to open full-screen detail view
- Swipe left on message: archive
- Swipe right on message: mark read / unread
- Swipe up on message row: more actions menu
- Back button in reading pane header returns to list
- Compose modal: full-screen overlay on mobile, header with close/send buttons

**Implementation Notes**:
- Responsive CSS with `@media (max-width: 900px)` breakpoints
- Sidebar toggle state in App-level context
- Swipe gestures: use `react-use-gesture` or custom touch listeners
- `onTouchStart` / `onTouchEnd` to detect swipe direction and distance
- Reading pane rendered as modal/overlay on mobile (absolute positioning or Modal component)
- Compose modal: `max-height: 100vh - header` on mobile

**Complexity**: 🟡 Medium

---

### 1.12 Error States & Offline Detection Banner

**Description**: Graceful error handling. Network status indicator. Retry buttons.

**Features**:
- Red banner at top when offline (connection lost)
- Banner shows: "No internet connection. Some features unavailable."
- Banner auto-hides when connection restored
- Error toast for failed actions (send, delete, archive): red toast with error message + "Retry" button
- "Something went wrong" full-screen error for critical failures (show error code for support)
- 404 page if message/folder not found
- Network error handling: automatic retry with exponential backoff (3 attempts max)
- Offline page: "You're offline. Cached messages available. Some actions disabled."
- Service Worker caches message list, reading pane for offline access (Phase 5)

**Implementation Notes**:
- Detect offline: `navigator.onLine` boolean, listen to online/offline window events
- Banner component at App root, conditional render
- Error toast via toast library (e.g., `react-hot-toast`, `sonner`)
- Error boundary component for critical errors
- React Query retry config: `retry: 2, retryDelay: attempt => Math.pow(2, attempt) * 1000`
- Offline page: check `navigator.onLine` before rendering app shell

**Complexity**: 🟡 Medium

---

### 1.13 Toast Notifications for Actions

**Description**: Brief, non-intrusive feedback for user actions.

**Features**:
- Toast shows for: message sent, archived, deleted, starred, moved, marked read, draft saved, attachment uploaded
- Success toasts: green, white checkmark icon, message like "Message archived"
- Error toasts: red, X icon, show error detail
- Warning toasts: yellow, show caution message
- Info toasts: blue, show neutral info
- Toast auto-dismisses after 3 seconds (user can close manually)
- Multiple toasts stack vertically, top-right corner (or position configurable in Phase 3)
- Toast includes "Undo" button for reversible actions (archive, delete, move) — available for 5 seconds only

**Implementation Notes**:
- Use toast library: `react-hot-toast` or `sonner` (Sonner recommended for better UX)
- Call `toast.success()`, `toast.error()`, etc. after action mutations complete
- Store action in state (last action + timestamp) to enable undo
- Undo button calls API again with reverse action (e.g., un-archive)
- Position: `top-right` on desktop, `bottom-center` on mobile

**Complexity**: 🟢 Easy

---

## Phase 2: Power User Features

### 2.1 Labels / Tags (Multi-Label per Message, Color-Coded)

**Description**: Flexible message organization beyond folders. Multiple labels per message. Visual color coding.

**Features**:
- Create custom labels with color (6 presets + custom hex picker)
- Add/remove multiple labels to message via context menu or button
- Label pills shown in message list row (colored background, white text)
- Label pills shown in reading pane message header
- Filter by label: click label pill in message, or use search (label:name syntax in Phase 2.9)
- Bulk label action: select multiple messages, apply label to all
- Label management page (Phase 3): rename, delete, change color, merge labels
- Labels synced across devices instantly

**Implementation Notes**:
- Backend: `messages.labels` JSONB array or separate `message_labels` junction table
- Label model: id, user_id, name, color_hex, created_at
- API: GET /api/v1/labels, POST /api/v1/labels, PATCH, DELETE
- API: PATCH /api/v1/messages/{id}/labels with array of label IDs
- Label pills component: renders as colored badge, click to remove
- Label picker modal: checkboxes for all user labels, apply/cancel buttons
- Color palette: 6 presets + custom picker input

**Complexity**: 🟡 Medium

---

### 2.2 Filters & Rules (Auto-Label, Auto-Archive, Auto-Forward)

**Description**: Declarative rules engine. "If sender/subject/keywords, then auto-label/archive/forward."

**Features**:
- Rule builder: visual form with IF/THEN structure
- IF conditions: sender (exact/contains), subject contains, body contains, label, recipient, domain
- THEN actions: add label, archive, delete, forward-to, mark-read, move-to-folder, apply-signature
- Rules applied to new incoming mail (not retroactive to existing messages, unless triggered manually)
- Dry-run mode: preview which messages would match rule
- Rules listed with enable/disable toggle, edit, delete buttons
- Rule priority/order (applied in sequence)
- Import/export rules as JSON for backup or sharing
- Skip rule for a single message (via context menu)

**Implementation Notes**:
- Backend: `rules` table with user_id, conditions (JSONB), actions (JSONB), enabled flag
- API: POST /api/v1/rules, GET /api/v1/rules, PATCH, DELETE
- Rule format (JSON):
  ```json
  {
    "id": "rule-1",
    "name": "Newsletter auto-archive",
    "enabled": true,
    "conditions": [
      {"type": "from_contains", "value": "newsletter@"}
    ],
    "actions": [
      {"type": "archive"},
      {"type": "add_label", "label_id": "abc"}
    ]
  }
  ```
- Frontend: ConditionBuilder and ActionBuilder components
- Condition types: dropdown select, value input, operator (contains/exact/regex)
- Action types: dropdown select, parameter input (e.g., label picker for add_label)
- Dry-run API: POST /api/v1/rules/preview with rule object, returns matching messages

**Complexity**: 🔴 Hard

---

### 2.3 Snooze Messages (Hide Until Specific Time, Reappear)

**Description**: Temporarily hide a message, automatically return to inbox at set time.

**Features**:
- Snooze button on message (reading pane and list row)
- Quick snooze options: 1 hour, 4 hours, tomorrow 9am, next Monday, custom date/time
- Custom snooze: date picker + time picker modal
- Snoozed messages removed from Inbox view (added to Snoozed folder or hidden from current folder)
- "Snoozed" folder shows all currently snoozed messages, grouped by reappear time
- Notification (toast or browser push) when snoozed message reappears
- Unsnoze option: remove snooze before time arrives
- Timezone aware: snooze times calculated in user's timezone

**Implementation Notes**:
- Backend: `messages.snoozed_until` timestamp column
- API: POST /api/v1/messages/{id}/snooze with `until_timestamp` or `preset: "1h"/"4h"/"tomorrow"/"next_monday"`
- API: POST /api/v1/messages/{id}/unsnooze to cancel snooze
- Message list filters: exclude snoozed messages unless viewing Snoozed folder
- Backend background job: check snoozed_until daily, move messages back to Inbox
- Frontend snooze picker: quick options (buttons) + custom (opens date/time picker)
- Reappear notification: push notification or toast in-app

**Complexity**: 🟡 Medium

---

### 2.4 Schedule Send (Pick Date/Time to Deliver)

**Description**: Compose a message now, deliver it later.

**Features**:
- "Schedule" button in compose modal (alternative to "Send")
- Date/time picker: date input + time input (respects user timezone)
- Scheduled message moved to Drafts folder with "Scheduled" badge
- Scheduled list shows scheduled time, recipient, subject
- Edit scheduled message (update time, recipient, body) before send
- Cancel scheduled message (delete it, no send)
- Send reminder 5 minutes before scheduled time (optional toast)
- Message sent exactly at scheduled time (or closest tick if offline)
- Analytics: track scheduled vs immediate sends

**Implementation Notes**:
- Backend: `messages.scheduled_for` timestamp column, draft state for scheduled
- API: POST /api/v1/drafts/{id}/schedule with `scheduled_for` timestamp
- API: PATCH /api/v1/messages/{id}/scheduled_for to reschedule
- API: DELETE /api/v1/messages/{id}/scheduled to cancel
- Backend job: daily/hourly check for messages where `scheduled_for <= now`, send them
- Frontend compose modal: "Schedule" button opens date/time picker
- Date/time picker: calendar + time input, timezone selector, "Send at [time] [timezone]"
- Scheduled drafts shown with badge in Drafts folder

**Complexity**: 🟡 Medium

---

### 2.5 Undo Send (5/10/30 Second Window)

**Description**: Cancel send within a grace period. Message returns to Drafts.

**Features**:
- After sending, show toast: "Message sent" with "Undo" button
- Undo window configurable: 5s (default), 10s, 30s (user choice in Phase 3 settings)
- Click "Undo": message pulled back, returned to Drafts folder for re-editing
- If undo window expires, message actually sent (no undo available)
- Undo only works if recipient server hasn't accepted yet (backend-dependent)
- Toast timer shows remaining undo time (or simple "Undo" button without timer)
- Can only undo one send per session (or allow multiple with state tracking)

**Implementation Notes**:
- Backend: `messages.send_at` column (optional, default `now()`), `undo_until` timestamp
- Draft sent: INSERT into messages with state='sent', `undo_until = now() + grace_period`
- API: POST /api/v1/messages/{id}/undo before `undo_until`
- Frontend: After send success, start toast with "Undo" button
- Button click calls PATCH /api/v1/messages/{id}/undo, moves message back to Drafts
- Undo disabled after grace period (toast disappears or button disabled)
- Grace period: 5s default, settable in settings (Phase 3)

**Complexity**: 🟡 Medium

---

### 2.6 Send Receipts / Read Receipts

**Description**: Request confirmation when recipient opens or receives mail.

**Features**:
- Compose modal: checkbox "Request read receipt"
- Read receipt header in outgoing message (RFC 3798)
- Recipient client respects receipt request, sends receipt back
- Receipt shows in reading pane: "John opened this message on May 10, 2:34 PM"
- Delivery receipt (simpler, server-side): "Delivered to mail server at [time]"
- Read receipts list per message: who read it, when
- Can disable read receipts globally (Phase 3 setting)
- Recipients can choose to send or decline receipt (client-side)

**Implementation Notes**:
- Backend: `messages.request_read_receipt` boolean flag
- Outgoing SMTP: add `Disposition-Notification-To` header if flag set
- Incoming receipt message: parse RFC 3798 return, extract original message ID
- DB: `message_read_receipts` table with message_id, recipient_email, opened_at, read_at
- API: GET /api/v1/messages/{id}/read-receipts returns list
- Reading pane: show "Read by" section with recipient list and timestamps
- Compose modal: checkbox for read receipt request

**Complexity**: 🟡 Medium

---

### 2.7 Priority Inbox (Smart Sorting: Important / Other)

**Description**: Automatic categorization of messages into Important and Other tabs.

**Features**:
- Message list tab bar: "Important" / "Other" tabs (not separate folders)
- Important tab shows high-priority messages first (algorithm below)
- Other tab shows remaining messages
- Smart algorithm (heuristic, not ML):
  - Direct to me (To/Cc, not BCC)
  - From contacts (known senders)
  - Sender has recent conversation history with user
  - User stars messages from this sender frequently
  - Subject contains user's name
  - No marketing keywords (Unsubscribe, Limited Time, etc.)
- Trainable: user marks message as important/not-important, algorithm learns
- Can disable Priority Inbox (show all messages) in Phase 3 settings

**Implementation Notes**:
- Backend: compute importance score per message in `messages.importance_score` (0-100)
- Scoring rules in database or config, evaluated during message insert or as background job
- API: GET /api/v1/messages?folder=inbox&importance=important|other (or boolean flag)
- Frontend: MessageList has tabs or filter buttons for Important/Other
- User can manually move between tabs (updates importance_score)
- Feedback loop: track user actions (star, archive, delete) per sender to retrain

**Complexity**: 🔴 Hard

---

### 2.8 Muted Threads (Never Show Notifications)

**Description**: Silence notifications for specific threads. Messages still arrive but don't alert.

**Features**:
- Mute button on message (reading pane)
- Mute thread: all future replies to this thread are muted
- Muted thread badge in list ("Muted")
- Muted threads don't send browser notifications
- Muted threads don't increment unread badge (or option to still show)
- Unmute option: via context menu or reading pane button
- Muted threads still appear in inbox (not hidden)

**Implementation Notes**:
- Backend: `messages.thread_muted` boolean, or `threads.muted` table
- API: PATCH /api/v1/messages/{id}/mute, PATCH .../unmute
- Message detail: show muted indicator, mute button toggles state
- Notifications: check message.thread_muted before sending push/toast
- Message list: filter badge shows if thread is muted

**Complexity**: 🟢 Easy

---

### 2.9 Contact Autocomplete in To/Cc/Bcc Fields

**Description**: Smart recipient suggestions as user types.

**Features**:
- To/Cc/Bcc input fields are autocomplete inputs, not plain text
- User types partial email/name, suggestions appear
- Suggestions from:
  - Recent contacts (sent mail to in past 6 months, highest frequency first)
  - All contacts (synced from Contacts app or imported)
  - Address book (Phase 5)
- Suggestion pills show email and name (if available)
- Click or press Enter to add suggestion
- Remove recipient: click X on pill
- Validate email format before adding
- Typing raw email address (user@example.com) auto-validates, adds when user presses Tab/comma

**Implementation Notes**:
- API: GET /api/v1/contacts?query=search_term for autocomplete
- API: GET /api/v1/contacts/recent?limit=20 for initial suggestions
- Recipient input component: use autocomplete UI library or custom
- Input triggers autocomplete dropdown after 1+ character typed
- Filter suggestions by query client-side or server-side
- Recipient pills: show email + name, click X to remove
- Email validation: regex or validator library before adding

**Complexity**: 🟡 Medium

---

### 2.10 Multiple Recipient Chips with Inline Validation

**Description**: Visual recipient pills. Validate addresses in real-time.

**Features**:
- Each recipient shown as colored pill/chip
- Invalid email highlighted in red (shows error tooltip)
- Hover shows full email address (if truncated)
- X button removes recipient
- Tab/comma separates recipients (auto-adds to pill list)
- Duplicate detection: warn if same email added twice
- Group detection (mailing lists): show special icon
- Click pill to edit email address inline
- Drag to reorder recipients (optional, Phase 2.9+)

**Implementation Notes**:
- RecipientInput component renders pills + autocomplete input
- State: array of recipient objects {email, name, validated: bool}
- Validation: regex or `email-validator` npm package
- Invalid emails: style with red border, show tooltip "Invalid email"
- Duplicate check: filter recipients by email, warn if duplicate
- onChange handler updates compose state
- Pills use small size, readable font

**Complexity**: 🟡 Medium

---

### 2.11 Cc/Bcc Toggle in Compose

**Description**: Show/hide Cc and Bcc fields. Not visible by default.

**Features**:
- Compose modal header: "To" field always visible
- Link/button "Add Cc" / "Add Bcc" below To field
- Click to expand respective field
- Once expanded, field stays visible in this compose session
- Remove Bcc recipients, Bcc field collapses (optional, or leave expanded)
- State persists if user starts typing in Cc/Bcc

**Implementation Notes**:
- ComposeModal state: showCc, showBcc booleans
- "Add Cc" button: onClick sets showCc=true
- Render `{showCc && <CcInput />}`
- Cc/Bcc input components same as To (autocomplete, pills)

**Complexity**: 🟢 Easy

---

### 2.12 Rich Compose: Inline Images, File Attachments, Signatures

**Description**: TipTap editor with drag-drop image insert, attachment linking, signature insertion.

**Features**:
- TipTap toolbar buttons: Bold, Italic, Underline, Link, Image, Attachment
- Image upload: drag-drop or file picker, auto-resized to max 600px width
- Inline image preview in compose editor
- Attachment handling: files shown as links in body or separate attachment list
- Signature insertion: button inserts signature at cursor (or appends at bottom)
- Rich formatting: lists, quotes, code blocks (all TipTap features)
- Mentions (optional Phase 2.9+): @name to mention contact
- Emoji picker (optional Phase 2.9+)
- Undo/redo in compose (via TipTap)
- Character count: show total characters / estimated bytes (for recipient server limits)

**Implementation Notes**:
- TipTap editor with default extensions: Paragraph, Heading, Bold, Italic, Link, Image, etc.
- Image extension: handle paste/drop events, upload to server, insert as `<img>`
- Attachment link: custom Tiptap extension or plain file list UI
- Signature: either inline in editor or separate append
- Character counter: compute from TipTap HTML + attachments metadata

**Complexity**: 🟡 Medium

---

### 2.13 Email Signature Manager (Multiple Signatures, Default per Domain)

**Description**: Create, store, select multiple signatures. Apply to outgoing mail automatically.

**Features**:
- Signature manager (Phase 3 settings): list of signatures with edit/delete
- Create signature: name + rich text editor (TipTap)
- Set default signature (used for all outgoing mail by default)
- Per-domain default signature (if multiple sender domains)
- In compose: dropdown to select which signature to append
- Or disable signature for this send only
- Signature variables (optional Phase 2.9+): {{name}}, {{email}}, {{title}}, {{company}}
- Preview signature before saving

**Implementation Notes**:
- Backend: `signatures` table with user_id, name, body (HTML), is_default, enabled_domains (JSONB)
- API: GET/POST/PATCH/DELETE /api/v1/signatures
- Compose modal: dropdown select signature, or "No signature"
- Selected signature appended to body before send (with `<br>` divider)
- Signature editor: TipTap modal, click "Edit" signature in manager

**Complexity**: 🟡 Medium

---

### 2.14 Templates / Canned Responses

**Description**: Save message templates for reuse. Quick-insert into compose.

**Features**:
- Template manager (Phase 3): create, list, edit, delete templates
- Template has title, category, body (rich text via TipTap)
- "Load template" button in compose modal
- Select template, insert body into compose (appends or replaces)
- Variables in templates: {{recipient_name}}, {{date}}, {{my_name}} (auto-replaced)
- Can mark template as "Quick reply" for fast access
- Search templates by title/category
- Template categories: Support, Sales, HR, Custom, etc.

**Implementation Notes**:
- Backend: `templates` table with user_id, title, category, body (HTML)
- API: GET/POST/PATCH/DELETE /api/v1/templates
- Compose modal: "Load template" button, opens template picker modal
- Template picker: grid or list view, searchable
- Click template: inserts body, replaces variables with actual values
- Variable substitution: regex replace {{VAR}} with values from context

**Complexity**: 🟡 Medium

---

### 2.15 Out-of-Office / Vacation Responder

**Description**: Auto-reply to incoming mail while away.

**Features**:
- Settings page (Phase 3): enable OOO with date range and message
- OOO responder subject: "Re: [original subject]"
- OOO message body: user-provided text + system footer "(Auto-reply, I return on May 20)"
- Enable/disable toggle in settings
- Set active date range: from date/time to date/time
- Send once per contact or once per day (user choice)
- Don't send OOO to mailing lists or newsletters (heuristic or explicit filter)
- OOO status shown in sidebar/header (e.g., "Out until May 20")
- Preview OOO message before activating

**Implementation Notes**:
- Backend: `vacation_responder` table with user_id, active, start_date, end_date, body, mode (once_per_contact / once_per_day)
- API: POST /api/v1/vacation-responder to enable/set, GET to check status, DELETE to disable
- Background job (or Submission/Inbound hook): check vacation_responder, send auto-reply if active + not sent yet to this contact
- Tracking: `vacation_responses` table with user_id, sender_email, last_sent_at (to prevent spam)
- Frontend: settings page with toggle, date range picker, rich text editor

**Complexity**: 🟡 Medium

---

## Phase 3: Personalization & Settings

### 3.1 Settings Panel (Accessible via Keyboard `/settings`)

**Description**: Unified settings UI. All user preferences in one place. Keyboard-navigable.

**Features**:
- Keyboard shortcut: `/settings` or `Shift+?` (customizable in Phase 3.5)
- Settings modal/page with tabs: General, Appearance, Notifications, Keyboard, Filters, Accounts, Signature, Vacation
- Each tab is its own component/page
- Settings auto-save (debounced 1 second after change)
- Success indicator: brief green checkmark
- Error handling: show error toast if save fails, "Retry" button
- Reset to defaults button (with confirmation)
- Settings synced across devices instantly (WebSocket or polling)
- Sidebar: Settings link at bottom, always accessible
- Mobile: settings in main menu or separate section

**Implementation Notes**:
- SettingsModal component with nested route (use React Router or Next.js dynamic routes)
- Each settings section (General, Appearance, etc.) as separate component
- Global settings context (zustand or React Context) for user preferences
- useMutation for PATCH /api/v1/settings with partial update
- Debounce setting changes before API call
- Error boundary wraps settings modal

**Complexity**: 🟡 Medium

---

### 3.2 General Settings

**Description**: Language, timezone, date format, density, preview lines, reading pane position, conversation view toggle.

**Features**:
- Language: dropdown with ko, en, ja, zh-CN (load translations with next-intl)
- Timezone: searchable dropdown (select2 or custom), detect auto
- Date format: dropdown (MM/DD/YYYY, DD/MM/YYYY, etc.)
- Message density: comfortable (52px rows), compact (36px), ultra-compact (28px)
- Message preview lines: 1, 2, or 3 lines (affects message list height)
- Reading pane position: right (default), bottom, hidden
- Conversation view toggle: on/off (group messages by thread or show flat list)
- Default reply behavior: reply (to sender only) or reply-all
- External images: auto-load (default), ask each time, block (for privacy)
- Undo send timer: disabled, 5s, 10s, 30s

**Implementation Notes**:
- Settings object:
  ```json
  {
    "language": "en",
    "timezone": "America/New_York",
    "date_format": "MM/DD/YYYY",
    "message_density": "comfortable",
    "preview_lines": 2,
    "reading_pane_position": "right",
    "conversation_view_enabled": true,
    "default_reply_mode": "reply",
    "external_images": "auto_load",
    "undo_send_timer_seconds": 5
  }
  ```
- Language change: reload i18n context
- Timezone: used for snooze/schedule send time calculations
- Reading pane position: CSS layout changes (flex direction, grid columns)
- Preview lines: control message list item height/overflow

**Complexity**: 🟡 Medium

---

### 3.3 Appearance Settings

**Description**: Theme, accent color, font size, message font, sidebar width, custom CSS.

**Features**:
- Theme: light, dark, system (follows OS preference)
- Accent color: 6 presets (blue, purple, red, green, orange, pink) + custom hex picker
- Font size: small, normal, large (affects all text)
- Message font: system default, Inter, Serif (affects message body only)
- Sidebar width: slider (200-300px) or fixed presets
- Sidebar resizable (drag right edge to resize, width persists)
- Custom CSS injection (power users): text area for custom CSS (with disclaimer)
- Live preview: changes apply instantly without save/reload
- Color palette preview: show how accent color looks in buttons, highlights

**Implementation Notes**:
- Theme setting: store in localStorage, apply `[data-theme="dark"]` on html element
- Accent color: CSS custom property `--color-accent`, editable via color picker
- Font size: CSS custom property `--text-base`, recalculate other scales
- Sidebar width: stored in localStorage, applied via inline style or CSS variable
- Custom CSS: append to <style> tag in head or inject via emotion/styled-components
- Live preview: use CSS-in-JS or state updates to reflect changes immediately
- Warn user: "Custom CSS can break the UI" disclaimer before allowing injection

**Complexity**: 🟡 Medium

---

### 3.4 Notification Settings

**Description**: Push notifications, sound, preview level, DND schedule, badge count.

**Features**:
- Browser push notification toggle (enable/disable)
- Push notifications per folder: Inbox (default on), Promotions (off), custom folders (user choice)
- Notification sound: off, soft chime, default (beep)
- Notification preview: show subject line, show sender only, no preview (privacy)
- Do Not Disturb schedule: enable/disable with start time and end time (e.g., 10pm-8am)
- Desktop badge count: unread messages (default), all messages, none
- Browser tab title badge: (12) Inbox showing unread count (if enabled)
- Email me notifications (optional Phase 3.4+): daily digest, weekly digest, or off

**Implementation Notes**:
- Settings object:
  ```json
  {
    "push_notifications_enabled": true,
    "push_per_folder": {"inbox": true, "promotions": false, "folder_123": true},
    "notification_sound": "soft",
    "notification_preview": "subject",
    "dnd_enabled": true,
    "dnd_start": "22:00",
    "dnd_end": "08:00",
    "badge_count_mode": "unread"
  }
  ```
- DND check: before sending notification, check if current time falls in DND range
- Browser badge: use favicon with number (or native Badging API if available)
- Sound: play audio file from public/sounds/notification.mp3

**Complexity**: 🟡 Medium

---

### 3.5 Keyboard Shortcuts (Customizable)

**Description**: View, edit, profile presets (Gmail-style, Superhuman-style, custom).

**Features**:
- Shortcuts list: all shortcuts, grouped by category (Navigation, Actions, Compose, etc.)
- Each shortcut shows: key combination, action description, custom toggle
- Edit shortcut: click to change key combination (with conflict warning)
- Keyboard input: press the key combination desired, it records the keys
- Conflict detection: warn if new shortcut conflicts with browser/system/existing shortcuts
- Shortcut profiles: dropdown to switch between Gmail-style, Superhuman-style, custom
- Switching profiles: modal confirmation (override current custom shortcuts)
- Preset profiles: built-in definitions for Gmail, Superhuman, Vim-style (Phase 3.5+)
- Export/import: save custom shortcuts as JSON, restore later
- Reset to profile: revert to selected preset profile

**Implementation Notes**:
- Shortcuts stored in settings: `custom_shortcuts: {[action]: key_combo}`
- Key combo format: "J", "Ctrl+R", "Shift+Cmd+K" (normalized)
- Profiles: enum or object with preset shortcut maps
- Conflict detection: check against browser shortcuts, OS shortcuts (hardcoded list), custom shortcuts
- Keyboard listener: capture `onKeyDown`, normalize modifiers, match against shortcut map
- Export: JSON download with custom shortcuts
- Import: file upload, validate structure, merge with existing

**Complexity**: 🟡 Medium

---

### 3.6 Filters & Rules Management Page

**Description**: Full CRUD for rules created in Phase 2.2. Visual rule builder with dry-run.

**Features**:
- Rules list: table with Name, Conditions (summary), Actions (summary), Enabled toggle, Edit, Delete buttons
- New rule button: opens rule builder modal
- Rule builder form: IF/THEN structure (see Phase 2.2)
- Drag-to-reorder rules (optional): changes execution order
- Dry-run button: show which messages in current folder match this rule
- Dry-run modal: list of matching messages, affected actions (colorized)
- Preview effect: show what would change (labels added, folder moved, etc.)
- Disable rule: toggle without deleting
- Export/import rules: JSON file download/upload for backup
- Bulk actions: enable/disable all, delete all (with confirmation)

**Implementation Notes**:
- RulesManager component with rulesQuery and rulesMutation hooks
- RuleBuilder modal with ConditionBuilder and ActionBuilder
- Dry-run: POST /api/v1/rules/{id}/preview
- Reorder: PUT /api/v1/rules/reorder with array of IDs in new order
- Export: client-side JSON.stringify(rules)
- Import: file input, JSON.parse, validate schema, POST /api/v1/rules/import

**Complexity**: 🔴 Hard

---

### 3.7 Accounts & Security

**Description**: Multi-account support, sessions, 2FA, app passwords, login history, trusted devices.

**Features**:
- Connected email accounts: list of linked accounts, active account indicator
- Add account: button to add another email account (redirects to login flow)
- Remove account: delete button (keep at least one account)
- Switch account: dropdown in sidebar or settings to switch active account
- Session management: list active sessions (device, location, last activity, sign-out button)
- 2FA setup: enable/disable, QR code for authenticator app, backup codes (show once)
- App passwords: generate for IMAP/SMTP clients, revoke individual passwords
- Login history: table of recent logins (date, device, location, IP, status)
- Trusted devices: list of trusted devices, revoke button (skip 2FA on trusted device)
- Download my data: GDPR-compliant export (mailbox, contacts, calendar, attachments)
- Delete account: irreversible, requires password confirmation, shows countdown

**Implementation Notes**:
- Multi-account: store user list in sidebar context, switch via query param or context state
- Sessions: backend GET /api/v1/sessions, DELETE /api/v1/sessions/{id}
- 2FA: GET /api/v1/2fa/setup (returns QR code), POST /api/v1/2fa/verify (confirm setup), DELETE (disable)
- Backup codes: POST /api/v1/2fa/backup-codes (returns array, show once only)
- App passwords: POST /api/v1/app-passwords, GET, DELETE
- Login history: GET /api/v1/login-history?limit=50
- Trusted devices: POST /api/v1/trusted-devices, GET, DELETE
- Download data: POST /api/v1/export-data (queues job, email download link)

**Complexity**: 🔴 Hard

---

### 3.8 Signature Settings

**Description**: Rich text signature editor, multiple signatures, auto-insert rules.

**Features**:
- Signature manager (see Phase 2.13)
- Default signature selector: which signature to use by default
- Auto-insert rule: always, never, new emails only, replies only
- Per-domain signature: set default signature per sender domain
- Signature variables: {{name}}, {{email}}, {{title}}, {{company}}, {{phone}} (auto-substituted)
- Test signature: preview in modal showing variable substitutions
- Import from file: paste/upload signature HTML
- Export signature: download as HTML or plain text

**Implementation Notes**:
- Auto-insert rule: stored as setting, checked during compose init
- Variables: replace via regex before appending signature
- Per-domain: separate signature mapping in settings

**Complexity**: 🟡 Medium

---

### 3.9 Vacation Responder Settings

**Description**: Configure OOO auto-reply (see Phase 2.15).

**Features**:
- Enable/disable toggle
- Active date range: date/time pickers (from, to)
- Vacation message: rich text editor (TipTap)
- Response mode: once per contact, once per day, always
- Exclude mailing lists: checkbox (heuristic filter)
- Preview: show how message will look
- Indicators: sidebar/header shows "Out until [date]" when active

**Implementation Notes**:
- Reuses vacation responder API from Phase 2.15
- Settings object includes vacation_responder config
- Date/time picker: use date + time inputs or single datetime input

**Complexity**: 🟡 Medium

---

## Phase 4: Productivity & Intelligence

### 4.1 Smart Reply Suggestions (3 Quick-Reply Chips)

**Description**: AI-powered quick-reply suggestions. Three chips below message for fast responses.

**Features**:
- Message detail view shows 3 suggested replies below message body
- Suggestions like "Thanks!", "Got it!", "Can you clarify?" (contextual to message)
- User clicks chip: text inserted into compose reply (auto-opens compose)
- Chip styling: gray background, rounded corners, small text
- Loading state: show placeholder chips while generating (AI latency)
- Disable suggestions: toggle in Phase 3 settings
- Model: on-device (fast) or server-side (better quality)
- Privacy: no message sent to external AI API (all-local or own backend)

**Implementation Notes**:
- Backend: POST /api/v1/messages/{id}/smart-replies returns array of 3 suggestions
- Model: lightweight local model (transformers.js) or server-side inference
- Frontend: show spinners while loading, then chips with suggestions
- Click chip: text goes into compose reply, auto-open compose modal
- Toggle in settings: disables suggestions for this user

**Complexity**: 🔴 Hard

---

### 4.2 Summary AI (1-Sentence TL;DR for Long Threads)

**Description**: AI-generated summary of thread. One sentence, helpful context.

**Features**:
- Long threads (3+ messages) show summary pill above thread
- Summary: "John asked about project status, you replied with timeline, waiting for approval"
- Click pill to expand/hide summary
- Summary updated if new message arrives in thread
- Disable: toggle in Phase 3 settings
- Privacy: no external API calls (local or backend inference)

**Implementation Notes**:
- Backend: POST /api/v1/threads/{id}/summary returns one-sentence summary
- Trigger: display if thread message_count > 3
- Model: local transformers.js or backend inference service
- Frontend: show as pill/badge above thread, clickable to toggle display

**Complexity**: 🔴 Hard

---

### 4.3 Unsubscribe Detector (Banner on Newsletters with 1-Click Unsubscribe)

**Description**: Identify newsletter messages and provide easy unsubscribe.

**Features**:
- Newsletter message shows banner: "This is a newsletter. Unsubscribe?"
- Unsubscribe button: one-click unsubscribe (follows RFC 8058)
- After unsubscribe: archive message, add sender to "Unsubscribed" filter (auto-delete future)
- Detection: heuristic analysis of message headers (List-Unsubscribe, marketing keywords)
- Disable detection: toggle in settings
- Unsubscribe list: view/manage unsubscribed senders in settings

**Implementation Notes**:
- Backend: parse message headers for List-Unsubscribe, extract URL or email
- Newsletter detection: heuristic scoring (X-List headers, bulk keywords, from patterns)
- API: PATCH /api/v1/messages/{id}/unsubscribe executes unsubscribe action
- Action: POST to List-Unsubscribe URL, or send email to unsubscribe address
- Frontend: show banner if message is newsletter, unsubscribe button
- Post-unsubscribe: archive message, add filter "from:[sender] -> archive"

**Complexity**: 🟡 Medium

---

### 4.4 Bulk Actions (Select All, Archive All, Delete All in Folder)

**Description**: Fast operations on multiple messages without individual clicks.

**Features**:
- Message list: checkbox header to "Select all" in current folder/search
- Bulk action bar: appears when messages selected
- Actions: archive, delete, mark-read, mark-unread, add label, move to folder
- Quick actions: archive all with one click (from bulk bar)
- Select limit: warn if trying to select 10,000+ messages
- Undo bulk action: with 30-second window (state permitting)
- Clear selection: button to deselect all

**Implementation Notes**:
- Message list state: selectedIds set, isAllSelected boolean
- Checkbox header: onClick toggles isAllSelected
- Each message row: checkbox, onClick toggles message in selectedIds
- Bulk action bar: conditional render if selectedIds.size > 0
- Action buttons call API with message IDs: PATCH /api/v1/messages/bulk with {ids: [], action: "archive"}
- API limits bulk size: max 1000 messages per request
- Undo: track last bulk action, replay opposite action with same IDs

**Complexity**: 🟡 Medium

---

### 4.5 Swipe Actions on Mobile (Left=Archive, Right=Delete)

**Description**: Quick actions via swipe gestures. Mobile-optimized.

**Features**:
- Swipe left on message row: show archive action
- Swipe right on message row: show delete action (move to trash)
- Animated swipe indicator: icon slides in as user swipes
- Tap action icon: confirm action (or complete swipe)
- Undo: swipe action shows undo button in toast
- Settings: customize swipe actions (left/right can be different actions)
- Swipe threshold: 50px minimum swipe distance

**Implementation Notes**:
- Touch listeners: onTouchStart / onTouchEnd on message row
- Calculate swipe distance: endX - startX
- Animate action buttons under message row on swipe
- Complete action: onClick archive/delete button
- Undo: show toast with undo button for 5 seconds

**Complexity**: 🟡 Medium

---

### 4.6 Quick Actions on Hover (Archive, Snooze, Mark-Read, Label)

**Description**: Hover-revealed action buttons. Desktop optimized.

**Features**:
- Message row on hover shows action buttons: archive, snooze, mark-read, label (4-5 buttons)
- Buttons appear on right side of row with subtle background
- Tooltips on each button (key combination shown)
- Order: archive (E), snooze (S), mark-read (U), label (+)
- Click button or press keyboard shortcut to execute action
- Visual feedback: button highlight on click, action executes immediately

**Implementation Notes**:
- Message row component: onMouseEnter/onMouseLeave state
- Actions buttons: conditional render if hovered
- Buttons styled as small icons with background on hover
- Accessibility: buttons focusable via Tab, clickable
- Keyboard shortcuts: same as Phase 1.1

**Complexity**: 🟡 Medium

---

### 4.7 Message Print View

**Description**: Print-friendly view of message. Clean layout, no chrome.

**Features**:
- Print button in reading pane header
- Print view: full-width, no sidebar, clean typography
- Sender info, date, subject clearly visible
- Body with original formatting preserved
- Attachments listed with download links
- Footer with print timestamp and "gogomail" branding
- CSS media query: @media print { ... } optimizes for printing
- Browser print dialog: user chooses printer, orientation, etc.

**Implementation Notes**:
- Print button onClick: open new window/tab with print view URL
- Print view component: receives message ID, renders message cleanly
- CSS media queries: hide sidebar, search, etc. in print
- Print styles: serif font for body, clear headings, black text
- Footer: auto-added via CSS @page rule or component

**Complexity**: 🟢 Easy

---

### 4.8 Export Messages as .eml / PDF

**Description**: Download message in standard formats.

**Features**:
- Export button in reading pane (or context menu)
- Format selection: .eml (raw message), PDF (formatted)
- .eml export: raw RFC 5322 message from backend
- PDF export: rendered message + attachments listed
- Filename: "From - Subject - Date.eml" or ".pdf"
- Bulk export: select messages, export all as .zip file
- Thread export: download entire thread as .eml or PDF

**Implementation Notes**:
- .eml download: GET /api/v1/messages/{id}/download?format=eml (backend returns raw message)
- PDF export: GET /api/v1/messages/{id}/download?format=pdf (backend uses print CSS to PDF)
- Or client-side: use `html2pdf` library to convert print view to PDF
- Filename: computed from message subject/date
- Bulk: POST /api/v1/messages/bulk-export with message IDs, returns .zip

**Complexity**: 🟡 Medium

---

### 4.9 Conversation Timeline View

**Description**: Alternative to thread view. Chronological timeline of messages.

**Features**:
- Timeline layout: messages shown vertically with time indicator on left
- Each message: avatar, sender, time, subject (if changed), preview/full body
- Collapsed mode: show only headers, click to expand body
- Date separators: "Today", "Yesterday", "May 10"
- Scroll to date: jump to message by date/time
- Timeline view toggle: switch between normal thread view and timeline
- Highlight own messages: different background color

**Implementation Notes**:
- Timeline component: renders messages with left-aligned time indicator
- Styling: message bubble with shadow, avatar on left, time in gray
- Toggle: checkbox or button in reading pane header
- State: store preference in settings (default view)

**Complexity**: 🟡 Medium

---

### 4.10 Message Source Viewer (Raw Headers)

**Description**: Show raw RFC 5322 message headers for debugging/analysis.

**Features**:
- Source viewer button in reading pane (advanced options menu)
- Modal shows raw message headers and body (monospace font)
- Copy button to copy entire source
- Search within source (Ctrl+F)
- Syntax highlighting: headers in one color, body in another (optional)
- Info icon: explain what raw source is, use cases

**Implementation Notes**:
- Source viewer modal: displays message.raw_source or fetches from backend
- GET /api/v1/messages/{id}/source returns full raw message
- Styling: <pre> tag with monospace font, max-height with scroll
- Copy: use Clipboard API to copy source to clipboard

**Complexity**: 🟢 Easy

---

## Phase 5: Integration & Advanced

### 5.1 Calendar Integration (Inline Event Invites, Accept/Decline)

**Description**: Show meeting invitations inline. One-click accept/decline.

**Features**:
- Calendar invite message shows event details inline: title, date/time, location, attendees
- Action buttons: "Accept", "Tentative", "Decline" (RSVP options)
- Event detail: brief popup showing full event info
- "Add to calendar" button: imports event to user's calendar app (Phase 5 Calendar app)
- Attendee list: show other attendees, their RSVP status (if available)
- Recurring event: show "This event" vs "All events in series" toggle
- After RSVP: show confirmation toast, update event in calendar

**Implementation Notes**:
- Backend: parse MIME part of type application/ics (iCalendar format)
- Extract event: title, date, time, location, organizer, attendees
- RSVP: generate iCalendar response, email back to organizer
- API: POST /api/v1/messages/{id}/rsvp with status (accepted/tentative/declined)
- Frontend: render inline invite component with event details and action buttons

**Complexity**: 🔴 Hard

---

### 5.2 Drive Integration (Attach Files from Drive, Save Attachments to Drive)

**Description**: Connect to GoGoMail Drive. Attach files, save attachments.

**Features**:
- Compose modal: "Attach from Drive" button
- File picker: browse Drive folders, select files to attach
- Large files (>100MB): warn user about size
- Save attachment: context menu "Save to Drive" on attachment
- Drive folder picker: choose destination folder
- Sync: attached files linked to Drive (not duplicated), show Drive icon

**Implementation Notes**:
- Integration with `apps/drive` module (Phase 5)
- API: GET /api/v1/drive/files for browsing, POST to attach (links file)
- Attachment context menu: "Save to Drive" calls POST /api/v1/drive/save-attachment
- Frontend: file picker UI reuses Drive UI component library

**Complexity**: 🔴 Hard

---

### 5.3 Contact Card Sidebar (When Email Selected, Show Sender's Card)

**Description**: Show contact info for message sender in right sidebar.

**Features**:
- Reading pane shows small contact card next to sender name
- Card shows: avatar, email, name, phone (if available), job title, company, recent message frequency
- Click card: open full contact detail page (Phase 5 Contacts app)
- Add to contacts: quick button if sender not in contacts
- Message history: list of last 5 messages from this sender
- Block sender: option to auto-archive future mail from this sender

**Implementation Notes**:
- Contact card component: shows when message selected
- API: GET /api/v1/contacts?email=sender@example.com
- If contact exists, show details; else show "Add to contacts" button
- Message history: filter messages by sender address
- Block sender: add rule to auto-archive from this sender

**Complexity**: 🟡 Medium

---

### 5.4 Presence Indicators (Show If Contact Is Online)

**Description**: Optional real-time status for contacts (future enhancement).

**Features**:
- Contact card shows online/offline status (green dot = online)
- Presence sync: fetch status every 30 seconds or via WebSocket
- Works if recipient also uses GoGoMail
- Gracefully degrades if recipient not available

**Implementation Notes**:
- WebSocket: subscribe to presence for contacts
- Or polling: GET /api/v1/presence?contacts=[emails] every 30s
- Show indicator: small dot next to contact avatar (green/gray)

**Complexity**: 🟡 Medium

---

### 5.5 External IMAP Account Import (One-Time Migration Wizard)

**Description**: Import mail from Gmail, Outlook, etc. via IMAP.

**Features**:
- Settings: "Import mail" button
- Wizard: step-by-step IMAP setup
- Step 1: email address, password (or app password)
- Step 2: IMAP server (auto-detect common providers)
- Step 3: select folders to import
- Step 4: confirm, start import (shows progress)
- Import: runs in background, shows progress notifications
- Imported mail: added to local "Imported" folder, original labels preserved

**Implementation Notes**:
- Wizard component: multi-step form
- API: POST /api/v1/import/imap with credentials
- Backend: connect via IMAP, download messages, parse, store locally
- Progress: WebSocket or polling for import status

**Complexity**: 🔴 Hard

---

### 5.6 Browser Extension Support (Gmail-Style mailto: Handling)

**Description**: Browser extension for external email composition.

**Features**:
- Extension intercepts `mailto:` links, opens GoGoMail compose
- Example: click "Email me" link on website → GoGoMail compose opens with To address pre-filled
- Extension installed: small icon in browser bar
- Settings: make GoGoMail default mail handler

**Implementation Notes**:
- Create browser extension (Chrome/Firefox manifest)
- Content script: intercept mailto: links
- Background script: send message to app
- App: listen for extension messages, open compose modal with email
- Or: use OS-level mailto handler (deep linking)

**Complexity**: 🔴 Hard

---

### 5.7 PWA (Installable, Offline Support for Cached Messages)

**Description**: Progressive Web App. Install to home screen. Works offline.

**Features**:
- Web manifest: app name, icon, theme color
- Install prompt: browser suggests "Install GoGoMail"
- App mode: runs fullscreen, no address bar
- Offline: show cached inbox, read cached messages, queue actions for sync
- Service Worker: cache message list, reading pane on first visit
- Sync: when back online, sync queued actions (send, archive, etc.)
- Background sync: enqueue actions while offline, sync on reconnect
- Update prompt: notify user when new version available

**Implementation Notes**:
- Next.js: create public/manifest.json with app metadata
- Service Worker: register in _app.tsx or _document.tsx
- Cache strategy: cache-first for assets, network-first for API calls
- Offline page: show cached messages, queue actions in localStorage
- Background sync API: register sync tag, replay queued actions on sync

**Complexity**: 🔴 Hard

---

### 5.8 Accessibility (WCAG 2.2 AA Compliance, Full Screen-Reader Support)

**Description**: Full accessibility audit and implementation.

**Features**:
- WCAG 2.2 Level AA compliance across all pages
- Screen reader support: semantic HTML, ARIA labels, landmarks
- Keyboard navigation: all functionality accessible via keyboard only
- Color contrast: minimum 4.5:1 text contrast
- Focus indicators: clear focus outlines on all interactive elements
- Form labels: associated with inputs via <label> or aria-label
- Alt text: all images have descriptive alt text
- Skip links: "Skip to main content" at top of page
- Page structure: proper heading hierarchy (h1, h2, h3)
- ARIA live regions: toast notifications announced to screen reader
- Test: Lighthouse audit, axe accessibility scanner, screen reader testing

**Implementation Notes**:
- Audit: run Lighthouse, axe DevTools on all pages
- Semantic HTML: use <main>, <nav>, <section>, etc.
- ARIA: aria-label, aria-labelledby, aria-live, aria-current, role attributes
- Focus management: focus on modal open, trap focus within modal
- Color contrast: test with contrast checker, adjust token colors if needed
- Keyboard shortcuts: all documented, no key conflicts
- Screen reader testing: use NVDA (Windows) or VoiceOver (macOS)

**Complexity**: 🔴 Hard

---

## Phase 6: Technical Debt & Infrastructure

### 6.1 React Query for All Server State

**Description**: No more useEffect fetches. All server state through React Query.

**Features**:
- Hooks for all data: useMessages, useMessageDetail, useFolders, useDrafts, useLabels, useSettings, etc.
- Automatic caching, deduplication, background refetch
- Optimistic updates: show changes immediately, revert on error
- Error handling: retry logic, error boundaries
- Loading states: useIsFetching hook for global loading indicator
- Stale-while-revalidate: serve stale data while fetching fresh
- DevTools: @tanstack/react-query-devtools for debugging

**Implementation Notes**:
- Install: `npm install @tanstack/react-query @tanstack/react-query-devtools`
- Provider: wrap app with `<QueryClientProvider>`
- Hooks: define custom hooks for each data type
- Query keys: consistent naming (e.g., ['messages', folderId])
- Mutations: useMutation for POST/PATCH/DELETE
- Invalidation: invalidateQueries after mutations

**Complexity**: 🟡 Medium

---

### 6.2 Optimistic Updates for All Mutations

**Description**: UI updates before API response. Instant feedback.

**Features**:
- Archive message: immediately remove from list, restore on error
- Mark read: instant visual change, revert if API fails
- Star: instant fill/outline, revert on error
- Label add/remove: instant pill add/remove
- Delete: toast with undo button (optimistic delete)
- Reply sent: toast immediately, revert if send fails

**Implementation Notes**:
- useMutation onSuccess: invalidate cache
- onMutate: manually update React Query cache before API call
- onError: revert cache to previous state
- Example:
  ```ts
  const archiveMessage = useMutation({
    mutationFn: (id) => api.patch(`/messages/${id}/archive`),
    onMutate: (id) => {
      queryClient.setQueryData(['messages'], (old) => 
        old.filter(m => m.id !== id)
      );
    },
    onError: (err, id) => {
      queryClient.invalidateQueries(['messages']);
    }
  });
  ```

**Complexity**: 🟡 Medium

---

### 6.3 WebSocket / SSE for Real-Time New-Message Push

**Description**: Real-time mail notifications. Messages appear as they arrive.

**Features**:
- WebSocket connection: maintain open connection to server
- New message event: server sends event when message arrives
- UI update: message appears in list without refresh
- Folder count update: unread badge updates in real-time
- Typing indicator: (optional) show who's composing reply
- Presence: (optional) show online status of contacts
- Reconnect: auto-reconnect on disconnect with exponential backoff
- Fallback: if WebSocket unavailable, poll via HTTP

**Implementation Notes**:
- Library: `ws` or `socket.io` on backend, `socket.io-client` on frontend
- Or use native WebSocket API
- Connect on app mount: `const socket = io(API_URL)`
- Listeners: `socket.on('message.new', handleNewMessage)`
- Emit: send ACK back to server after processing
- Reconnect: auto-reconnect every 5s with backoff (max 60s)
- Test: disconnect network, verify reconnect behavior

**Complexity**: 🔴 Hard

---

### 6.4 Service Worker for Offline Cache + Background Sync

**Description**: Cache messages for offline reading. Queue actions offline.

**Features**:
- Service Worker registration: register on app mount
- Cache strategy: cache-first for static assets, network-first for API
- Offline message list: cached messages show with "offline" indicator
- Offline reading: read cached messages without network
- Offline actions: queue archive/delete/star for later sync
- Background sync: when online, sync queued actions
- Periodic sync: sync inbox every hour (if online, user permits)

**Implementation Notes**:
- SW registration: `navigator.serviceWorker.register('/sw.js')`
- Cache API: use `caches.open()`, `cache.addAll(urls)`
- Fetch intercept: `event.respondWith(cacheFirst() || networkFirst())`
- Offline detection: `navigator.onLine` check
- Queue storage: localStorage for offline actions
- Background sync: Service Worker `sync` event listener

**Complexity**: 🔴 Hard

---

### 6.5 i18n (next-intl) for All UI Strings

**Description**: Full internationalization. Korean primary, English, Japanese, Chinese support.

**Features**:
- next-intl setup: config in next.config.js
- Message files: locales/ko.json, en.json, ja.json, zh-CN.json
- All UI strings: dates, times, labels, buttons, messages translated
- Language selector: Phase 3.2 settings
- Automatic locale detection: browser language preference on first visit
- RTL support: (optional) prepare CSS for future RTL languages
- Formatting: dates/times respect locale (e.g., "5/10/2026" vs "10.5.2026")
- Pluralization: handle singular/plural correctly per language

**Implementation Notes**:
- Install: `npm install next-intl`
- Config: `next.config.js` with locales array and defaultLocale
- Middleware: redirect to locale-prefixed routes (e.g., /en/, /ko/)
- Usage: `const t = useTranslations()`, `t('messages.archived')`
- Message files: JSON with nested keys (e.g., `{"messages": {"archived": "메시지 보관됨"}}`)
- Date formatting: `new Intl.DateTimeFormat(locale).format(date)`
- Testing: verify all languages have complete strings, no missing keys

**Complexity**: 🟡 Medium

---

### 6.6 E2E Tests (Playwright) for Critical Paths

**Description**: Automated E2E testing of core mail workflows.

**Features**:
- Test scenarios:
  - Login → Inbox view → select message → read
  - Compose new mail → add recipient → type subject → send
  - Archive message, verify folder count updates
  - Search messages, verify results
  - Reply to message, send reply
  - Mark as read/unread
  - Create folder, move message to folder
  - Snooze message, verify it reappears
  - Offline scenario: compose offline, send when online
- Browser coverage: Chrome (main), Firefox, Safari (optional)
- Mobile testing: responsive layout, swipe gestures
- Performance: lighthouse checks for key pages

**Implementation Notes**:
- Setup: `npm install @playwright/test`
- Config: playwright.config.ts with base URL, browsers, headless flag
- Test structure: describe blocks for feature areas, it blocks for scenarios
- Fixtures: reusable setup (login, create test account)
- Selectors: use data-testid attributes for stable element selection
- CI/CD: run tests on every commit (GitHub Actions)
- Reports: HTML report, screenshots on failure

**Complexity**: 🔴 Hard

---

### 6.7 Storybook for Component Library

**Description**: Isolated component development and documentation.

**Features**:
- Storybook setup: .storybook config, MDX documentation
- Components in Storybook: MessageRow, ComposeModal, RecipientInput, Label, etc.
- Stories: primary story, interactive args, different states (empty, loading, error)
- Docs: component props, usage examples, design guidelines
- Visual regression testing: (optional) screenshot tests for visual changes
- Accessibility testing: (optional) a11y plugin checks accessibility

**Implementation Notes**:
- Setup: `npx sb init` in Next.js project
- Config: .storybook/main.ts with Next.js preset
- Story files: ComponentName.stories.tsx in same folder
- Story template: export Meta and Story with args
- Accessibility: install @storybook/addon-a11y, use it in .storybook/main.ts
- Docs: enable autodocs in story config

**Complexity**: 🟡 Medium

---

### 6.8 Bundle Analysis & Code Splitting per Route

**Description**: Optimize bundle size. Code splitting for faster initial load.

**Features**:
- Bundle analysis: @next/bundle-analyzer to visualize bundle
- Code splitting: automatic per-route code splitting via Next.js
- Dynamic imports: `import dynamic from 'next/dynamic'` for heavy components
- Lazy routes: defer loading SettingsModal, SettingsPage until opened
- Critical path: optimize above-the-fold (MessageList should load fast)
- Images: optimize with next/image, lazy load
- Fonts: load only used weights/languages of Inter font
- Target: <200KB main bundle, <100KB per route

**Implementation Notes**:
- Bundle analyzer: `npm install --save-dev @next/bundle-analyzer`
- Config: wrap next config with BundleAnalyzer in next.config.js
- Dynamic imports: `const SettingsPage = dynamic(() => import('./SettingsPage'), { ssr: false })`
- Image optimization: use `<Image>` component instead of `<img>`
- Font loading: use `next/font` to load Inter locally
- Monitor: run analyzer on each major change, keep baseline

**Complexity**: 🟡 Medium

---

### 6.9 CSP Headers & XSS Protection Audit

**Description**: Security hardening. Content Security Policy, input validation.

**Features**:
- CSP header: strict policy for scripts, styles, images, fonts
- XSS protection: sanitize user input, escape output
- CSRF protection: verify CSRF tokens on forms
- Input validation: validate all form inputs client and server-side
- Output encoding: HTML-encode user data before rendering
- Content-Security-Policy header: script-src 'self', style-src 'self', etc.
- Security audit: OWASP Top 10 review, penetration testing (Phase 6+)

**Implementation Notes**:
- CSP header: set in next.config.js via headers() function or middleware
- Sanitization: use `dompurify` or `sanitize-html` for user-provided HTML
- Validation: zod or yup for schema validation on forms
- Escape: React auto-escapes, but be careful with dangerouslySetInnerHTML
- Test: use CSP violation reports, browser console checks

**Complexity**: 🟡 Medium

---

### 6.10 Lighthouse Score Targets (Performance 95+, Accessibility 100)

**Description**: Performance and accessibility monitoring. Automated audits.

**Features**:
- Lighthouse CI: run on every PR, fail if scores drop
- Target scores: Performance 95+, Accessibility 100, SEO 100, Best Practices 95+
- Core Web Vitals: monitor LCP, FID, CLS
- Performance budget: set bundle size limits per route
- Accessibility: automated checks for color contrast, form labels, etc.
- Mobile optimization: test on slow 4G network
- Reporting: Lighthouse report as CI artifact

**Implementation Notes**:
- Setup: `npm install @lhci/cli`
- Config: lighthouserc.json with targets and thresholds
- CI: run Lighthouse on PRs, block merge if score drops
- Monitoring: use web-vitals library to track scores in production
- Optimization: follow Lighthouse suggestions for improvements

**Complexity**: 🟡 Medium

---

## Implementation Priority & Timeline

**Recommended Phase Sequence**:
1. **Phase 1** (MVP+): Foundations for power users. ~3-4 weeks.
2. **Phase 2** (Power Features): Advanced productivity. ~4-5 weeks.
3. **Phase 3** (Settings): Full personalization. ~3-4 weeks.
4. **Phase 4** (Intelligence): AI features + bulk actions. ~2-3 weeks.
5. **Phase 5** (Integration): Calendar, Drive, PWA, Accessibility. ~3-4 weeks.
6. **Phase 6** (Infrastructure): Quality, performance, testing. Ongoing.

**Quick Wins (Phase 1)**: keyboard shortcuts, thread view, mark read/unread, search, draft autosave.

**High-Impact (Phase 2-3)**: labels, filters & rules, snooze, signature manager.

**Polish (Phase 4-6)**: AI summaries, PWA, E2E tests, Storybook, bundle optimization.

---

## Notes for Developers

- **Design System**: Follow `/DESIGN.md` for all components. Reuse token colors, typography scales.
- **API Contract**: Backend provides all endpoints in `/api/v1/`. Update OpenAPI docs as new features land.
- **Testing**: Phase 1 should have >80% unit test coverage. Phase 6 adds E2E.
- **Performance**: Prioritize message list performance (infinite scroll, lazy rendering). Monitor bundle size.
- **Accessibility**: Every interactive element keyboard-accessible. Test with screen reader.
- **Mobile-First**: Design responsive from the start. Mobile must work, not feel shoehorned.
- **Localization**: Plan for Korean, English, Japanese, Chinese from Phase 1. Don't hardcode strings.
- **Error Messages**: Clear, actionable error messages. Show validation errors inline, not in modals.
- **Loading States**: Show spinners, skeleton screens, optimistic updates. Never leave UI frozen.

---

**Status**: Ready for frontend implementation once backend API stabilizes. Start Phase 1 once `/api/v1` contracts are locked.

Last updated: 2026-05-11
