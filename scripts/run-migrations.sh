#!/bin/bash
set -e

# Wait for the database to be ready
echo "Waiting for database to be ready..."
echo "Using connection details:"
echo "Host: $DB_HOST"
echo "User: $DB_USER"
echo "Database: $DB_NAME"

for i in {1..60}; do
    if PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c '\q' >/dev/null 2>&1; then
        echo "Database is ready!"
        break
    fi
    echo "Waiting for database... ($i/60)"
    if ! PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c '\q' 2>&1; then
        echo "Connection attempt failed"
    fi
    sleep 2
done

# Check if we timed out
if ! PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c '\q' >/dev/null 2>&1; then
    echo "Error: Timed out waiting for database"
    exit 1
fi

# Run the migrations
echo "Running migrations..."
PGPASSWORD=$DB_PASSWORD psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -f /app/migrations/001_create_tasks_table.sql 