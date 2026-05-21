-- +goose Up
CREATE INDEX IF NOT EXISTS idx_mail_flow_logs_company_created_id
	ON mail_flow_logs(company_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mail_flow_logs_domain_created_id
	ON mail_flow_logs(domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mail_flow_logs_user_created_id
	ON mail_flow_logs(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mail_flow_logs_company_domain_created_id
	ON mail_flow_logs(company_id, domain_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_mail_flow_logs_message_created_id
	ON mail_flow_logs(message_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_mail_flow_logs_message_created_id;
DROP INDEX IF EXISTS idx_mail_flow_logs_company_domain_created_id;
DROP INDEX IF EXISTS idx_mail_flow_logs_user_created_id;
DROP INDEX IF EXISTS idx_mail_flow_logs_domain_created_id;
DROP INDEX IF EXISTS idx_mail_flow_logs_company_created_id;
