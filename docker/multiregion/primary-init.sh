#!/bin/sh
# Runs once on first boot of postgres-primary (docker-entrypoint-initdb.d).
# Creates the physical-replication role and permits the replica to connect.
#
# For a failover drill only — production uses per-region networking, TLS, and
# a secrets manager rather than a hard-coded replication password.
set -eu

: "${REPLICATION_USER:=replicator}"
: "${REPLICATION_PASSWORD:=replicator}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
	CREATE ROLE ${REPLICATION_USER} WITH REPLICATION LOGIN PASSWORD '${REPLICATION_PASSWORD}';
SQL

# Allow the replica (any host on the compose network) to open a replication
# connection. Restrict this CIDR in production.
cat >> "${PGDATA}/pg_hba.conf" <<-HBA
	host    replication    ${REPLICATION_USER}    0.0.0.0/0    scram-sha-256
HBA

echo "primary: replication role '${REPLICATION_USER}' created and pg_hba.conf updated"
