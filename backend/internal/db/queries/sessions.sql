-- name: CreateSession :exec
INSERT INTO rp.sessions (id, user_id, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessionWithUser :one
SELECT sqlc.embed(s), sqlc.embed(u)
FROM rp.sessions s
JOIN rp.users u ON u.id = s.user_id
WHERE s.id = $1
  AND s.expires_at > now()
  AND u.voided_ts IS NULL;

-- name: TouchSession :exec
UPDATE rp.sessions
SET last_used_ts = now()
WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM rp.sessions
WHERE id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM rp.sessions
WHERE expires_at < now();
