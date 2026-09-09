# Production checklist

| # | Gate |
| --- | --- |
| 1 | `smoke-all.sh` green on target env |
| 2 | Strong admin password (not example default) |
| 3 | Unique `JWT_SECRET` |
| 4 | DB backup + restore drill |
| 5 | Migrations on empty database |
| 6 | PostgreSQL not publicly exposed |
| 7 | TLS / reverse proxy if on a network |
| 8 | Concurrent U-slot smoke |
| 9 | Approval & last-admin smoke |
| 10 | Rollback: previous image + dump |

```bash
# backup
docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" <postgres-container> \
  pg_dump -U cabinet -d dcimlite -Fc > backup.dump
```
