-- +goose Up
CREATE TABLE mail_flow_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  direction text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
  company_id uuid,
  domain_id uuid,
  user_id uuid,

  message_id uuid,
  rfc_message_id text NOT NULL DEFAULT '',
  from_addr text NOT NULL DEFAULT '',
  from_name text NOT NULL DEFAULT '',
  to_addrs jsonb NOT NULL DEFAULT '[]'::jsonb,
  subject text NOT NULL DEFAULT '',

  flow_status text NOT NULL,
  enhanced_status text NOT NULL DEFAULT '',
  error_message text NOT NULL DEFAULT '',

  spam_score double precision,
  dkim_result text NOT NULL DEFAULT '',
  spf_result text NOT NULL DEFAULT '',
  dmarc_result text NOT NULL DEFAULT '',

  transport text NOT NULL DEFAULT '',
  farm text NOT NULL DEFAULT '',
  size bigint NOT NULL DEFAULT 0,

  received_at timestamptz,
  processed_at timestamptz,

  in_reply_to text NOT NULL DEFAULT '',
  references text NOT NULL DEFAULT '',
  thread_id uuid,

  ip_address inet,
  mail_from text NOT NULL DEFAULT '',
  rcpt_to text NOT NULL DEFAULT '',

  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_mail_flow_logs_scope_time ON mail_flow_logs(company_id, domain_id, created_at DESC);
CREATE INDEX idx_mail_flow_logs_user_time ON mail_flow_logs(user_id, created_at DESC);
CREATE INDEX idx_mail_flow_logs_direction_time ON mail_flow_logs(direction, created_at DESC);
CREATE INDEX idx_mail_flow_logs_message_id ON mail_flow_logs(message_id);
CREATE INDEX idx_mail_flow_logs_rfc_message_id ON mail_flow_logs(rfc_message_id);
CREATE INDEX idx_mail_flow_logs_status_time ON mail_flow_logs(flow_status, created_at DESC);
CREATE INDEX idx_mail_flow_logs_received_at ON mail_flow_logs(received_at DESC);

-- +goose Down
DROP TABLE IF EXISTS mail_flow_logs;
