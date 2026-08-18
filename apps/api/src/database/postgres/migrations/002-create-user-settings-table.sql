-- Migration: 002-create-user-settings-table
-- Mô tả: Bảng cấu hình cá nhân hóa (Agent Persona, Custom Instructions) cho từng người dùng.

CREATE TABLE IF NOT EXISTS user_settings (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    persona_preset      VARCHAR(50) NOT NULL DEFAULT 'default',
    formality           VARCHAR(20) NOT NULL DEFAULT 'neutral' CHECK (formality IN ('casual', 'neutral', 'formal')),
    verbosity           VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK (verbosity IN ('concise', 'normal', 'detailed')),
    humor               VARCHAR(20) NOT NULL DEFAULT 'none' CHECK (humor IN ('none', 'dry', 'playful')),
    custom_instructions TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS user_settings_updated_at ON user_settings;
CREATE TRIGGER user_settings_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
