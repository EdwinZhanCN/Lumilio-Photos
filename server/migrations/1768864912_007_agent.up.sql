CREATE TABLE agent_checkpoints (
    -- id 通常是 thread_id 或 session_id，由业务层决定
    id TEXT PRIMARY KEY,
    -- 存储序列化后的 Session 和 Agent 状态
    data BYTEA NOT NULL,
    -- 记录最后更新时间，方便清理过期会话
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
)
