#!/bin/bash
# PostgreSQL 3-node cluster setup for large deployment
# This assumes etcd is used for cluster coordination

set -e

echo "=== PostgreSQL Cluster Setup ==="

# Wait for PostgreSQL to be ready
until pg_isready -U postgres -h localhost > /dev/null 2>&1; do
  echo "Waiting for PostgreSQL..."
  sleep 1
done

echo "✓ PostgreSQL is ready"

# Create replication user for cluster
psql -U postgres <<EOF
CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'repl_cluster_password_change_me';
GRANT CONNECT ON DATABASE gogomail TO replicator;
EOF

echo "✓ Cluster replication user created"

# Create monitoring user for Prometheus
psql -U postgres <<EOF
CREATE USER monitoring WITH ENCRYPTED PASSWORD 'monitoring_password_change_me';
GRANT CONNECT ON DATABASE gogomail TO monitoring;
EOF

echo "✓ Monitoring user created"

# Setup replication slots (useful for streaming replication)
psql -U postgres <<EOF
SELECT * FROM pg_create_physical_replication_slot('replica_slot_1', false);
SELECT * FROM pg_create_physical_replication_slot('replica_slot_2', false);
EOF

echo "✓ Physical replication slots created"

# Show cluster status
psql -U postgres <<EOF
SELECT datname, usename, application_name FROM pg_stat_replication;
EOF

echo "✓ PostgreSQL cluster setup complete"
echo "Note: Change replication and monitoring passwords in production"
