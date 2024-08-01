# Gophermart

# DB Migrations

DB migrations stored in ./db/migrations path.

```bash
export DATABASE_URI=""
docker run --rm \
    -v $(realpath ./db/migrations):/migrations \
    migrate/migrate:v4.16.2 \
        -path=/migrations \
        -database $DATABASE_URI \
        up
```

Rollback all DB migrations:
```bash
export DATABASE_URI=""
docker run --rm \
    -v $(realpath ./db/migrations):/migrations \
    migrate/migrate:v4.16.2 \
        -path=/migrations \
        -database $DATABASE_URI \
        down -all
```
