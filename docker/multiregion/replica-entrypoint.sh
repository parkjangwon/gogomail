#!/bin/sh
# Entrypoint for postgres-replica in the failover-practice stack.
#
# On first boot (empty data dir) it runs pg_basebackup against the primary to
# clone the cluster and configure streaming replication, then starts Postgres
# as a hot standby (read-only). On subsequent boots it just starts Postgres.
#
# Practice-only: production replicas are provisioned via the cloud provider's
# managed read-replica feature or an automation tool, not this script.
set -eu

: "${PRIMARY_HOST:=postgres-primary}"
: "${PRIMARY_PORT:=5432}"
: "${REPLICATION_USER:=replicator}"
: "${REPLICATION_PASSWORD:=replicator}"
: "${PGDATA:=/var/lib/postgresql/data}"

if [ ! -s "${PGDATA}/PG_VERSION" ]; then
	echo "replica: cloning from ${PRIMARY_HOST}:${PRIMARY_PORT} via pg_basebackup"
	# Wait until the primary accepts replication connections.
	until pg_isready -h "${PRIMARY_HOST}" -p "${PRIMARY_PORT}" -U "${REPLICATION_USER}" >/dev/null 2>&1; do
		echo "replica: waiting for primary to accept connections..."
		sleep 2
	done

	rm -rf "${PGDATA:?}/"* 2>/dev/null || true
	export PGPASSWORD="${REPLICATION_PASSWORD}"
	pg_basebackup \
		--host="${PRIMARY_HOST}" \
		--port="${PRIMARY_PORT}" \
		--username="${REPLICATION_USER}" \
		--pgdata="${PGDATA}" \
		--wal-method=stream \
		--write-recovery-conf \
		--progress \
		--verbose
	unset PGPASSWORD

	# --write-recovery-conf already writes primary_conninfo + standby.signal.
	echo "replica: base backup complete; starting as hot standby"
fi

exec postgres
