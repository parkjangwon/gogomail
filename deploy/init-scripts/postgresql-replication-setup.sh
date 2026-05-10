#!/bin/bash
# PostgreSQL Replication Setup for medium deployment
# This script runs in the postgres-primary container

set -e

echo "=== PostgreSQL Primary Replication Setup ==="

# Create replication user
psql -U postgres <<EOF
CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'repl_password_change_me';
GRANT CONNECT ON DATABASE gogomail TO replicator;
EOF

echo "✓ Replicator user created"

# Verify settings
psql -U postgres <<EOF
SELECT datname, usename FROM pg_database db
  LEFT JOIN pg_user u ON true
  WHERE datname = 'gogomail';
EOF

echo "✓ PostgreSQL replication setup complete"
echo "Note: Change 'repl_password_change_me' to a secure password in production"
