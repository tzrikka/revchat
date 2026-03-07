#!/bin/sh
set -eu

echo 'Starting PostgreSQL schema setup...'
echo 'Waiting for PostgreSQL port to be available...'
nc -z -w 10 ${SQL_HOST} ${SQL_PORT}
echo 'PostgreSQL port is available'

# Create and setup temporal database
temporal-sql-tool create-database
temporal-sql-tool setup-schema -v 0.0
temporal-sql-tool update-schema -d /etc/temporal/schema/postgresql/v12/temporal/versioned

# Create and setup visibility database
temporal-sql-tool --db ${SQL_DATABASE}_visibility create-database
temporal-sql-tool --db ${SQL_DATABASE}_visibility setup-schema -v 0.0
temporal-sql-tool --db ${SQL_DATABASE}_visibility update-schema -d /etc/temporal/schema/postgresql/v12/visibility/versioned

echo 'PostgreSQL schema setup complete'
