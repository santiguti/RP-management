# Jobs

## Recurring expenses

Daily, 02:00 server time:

```cron
0 2 * * * cd /opt/rp-management/backend && go run ./cmd/jobs run-recurring >> /var/log/rp-recurring.log 2>&1
```
