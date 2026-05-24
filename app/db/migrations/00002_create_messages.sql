-- +goose Up
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    line_user_id VARCHAR(64) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    message_type VARCHAR(32) NOT NULL,
    content TEXT,
    received_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_group_id_received_at ON messages(group_id, received_at DESC);

-- +goose Down
DROP INDEX idx_messages_group_id_received_at;
DROP TABLE messages;
