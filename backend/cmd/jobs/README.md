# Jobs

## Recurring expenses

Daily, 02:00 server time:

```cron
0 2 * * * cd /opt/rp-management/backend && go run ./cmd/jobs run-recurring >> /var/log/rp-jobs.log 2>&1
```

## Session cleanup

Daily, 02:10 server time, after recurring expenses:

```cron
10 2 * * * cd /opt/rp-management/backend && go run ./cmd/jobs cleanup-sessions >> /var/log/rp-jobs.log 2>&1
```
