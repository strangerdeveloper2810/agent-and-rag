-- Migration: 003-user-avatar-mcp-skills
-- Mô tả: Thêm agent_avatar_url, bảng MCP servers, bảng custom skills, và bảng disabled skills.

-- 1. Thêm cột agent_avatar_url vào user_settings
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS agent_avatar_url TEXT DEFAULT NULL;

-- 2. Bảng MCP servers của user (chỉ hỗ trợ SSE transport)
CREATE TABLE IF NOT EXISTS user_mcp_servers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    transport       VARCHAR(10) NOT NULL DEFAULT 'sse' CHECK (transport IN ('sse')),
    url             TEXT NOT NULL,
    api_key         TEXT DEFAULT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

DROP TRIGGER IF EXISTS user_mcp_servers_updated_at ON user_mcp_servers;
CREATE TRIGGER user_mcp_servers_updated_at
    BEFORE UPDATE ON user_mcp_servers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- 3. Bảng custom skills của user
CREATE TABLE IF NOT EXISTS user_skills (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    when_to_use     TEXT NOT NULL DEFAULT '',
    content         TEXT NOT NULL DEFAULT '',
    triggers        TEXT[] DEFAULT '{}',
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

DROP TRIGGER IF EXISTS user_skills_updated_at ON user_skills;
CREATE TRIGGER user_skills_updated_at
    BEFORE UPDATE ON user_skills
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- 4. Bảng builtin skills bị disable của user
CREATE TABLE IF NOT EXISTS user_disabled_skills (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_name  VARCHAR(100) NOT NULL,
    PRIMARY KEY (user_id, skill_name)
);
