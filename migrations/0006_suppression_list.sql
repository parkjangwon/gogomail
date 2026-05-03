-- +goose Up
CREATE TABLE suppression_list (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id uuid REFERENCES domains(id) ON DELETE CASCADE,
  email text NOT NULL,
  reason text NOT NULL,
  source_message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_suppression_scope_email
  ON suppression_list (
    COALESCE(domain_id, '00000000-0000-0000-0000-000000000000'::uuid),
    lower(email)
  );

-- +goose Down
DROP TABLE IF EXISTS suppression_list;
