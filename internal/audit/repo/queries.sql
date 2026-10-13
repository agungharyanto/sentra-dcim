-- name: InsertEvent :one
INSERT INTO audit_events (actor_type, actor_id, actor_username, action, target_type, target_id, result,
                          ip, user_agent, request_id, metadata)
VALUES (@actor_type, @actor_id, @actor_username, @action, @target_type, @target_id, @result,
        @ip, @user_agent, @request_id, @metadata)
RETURNING id, occurred_at;
