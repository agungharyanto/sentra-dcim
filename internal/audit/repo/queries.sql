-- name: InsertEvent :one
INSERT INTO audit_events (actor_type, actor_id, actor_username, action, target_type, target_id, result,
                          ip, user_agent, request_id, metadata)
VALUES (@actor_type, @actor_id, @actor_username, @action, @target_type, @target_id, @result,
        @ip, @user_agent, @request_id, @metadata)
RETURNING id, occurred_at;

-- ListEvents is the audit viewer (v0.3.4): newest first, keyset pagination on chain_seq (before = exclusive upper
-- bound). Empty/NULL filters are ignored; actor is a literal substring (LIKE wildcards escaped by the caller);
-- action matches exactly or as a dotted prefix ("auth" → auth.*).
-- name: ListEvents :many
SELECT chain_seq, occurred_at, actor_type, actor_id, actor_username, action, target_type, target_id, result,
       coalesce(host(ip), '')::text AS ip, user_agent, request_id, metadata, hash
FROM audit_events
WHERE (sqlc.narg('before')::bigint IS NULL OR chain_seq < sqlc.narg('before')::bigint)
  AND (sqlc.narg('since')::timestamptz IS NULL OR occurred_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR occurred_at < sqlc.narg('until')::timestamptz)
  AND (@actor::text = '' OR actor_username ILIKE '%' || @actor || '%' ESCAPE '\')
  AND (@action::text = '' OR action = @action OR starts_with(action, @action || '.'))
  AND (@result::text = '' OR result = @result)
  AND (@target_type::text = '' OR target_type = @target_type)
  AND (@target_id::text = '' OR target_id = @target_id)
ORDER BY chain_seq DESC
LIMIT @lim;

-- name: ListActions :many
SELECT DISTINCT action FROM audit_events ORDER BY action;
