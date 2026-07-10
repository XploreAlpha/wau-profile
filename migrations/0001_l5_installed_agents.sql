-- =============================================================================
-- wau-profile migration 0001: ⭐L5 包管理器 3 表 (per D73, 2026-07-10)
-- =============================================================================
-- D73 = A 拍板:wau-profile 升级范围 = 只新加 3 表,不动 v0.9.0 旧表
--   - installed_agents (user × agent 一行)
--   - user_skill_pool  (user × skill 一行)
--   - agent_memory     (user × agent × key 一行)
-- 跟 WAU-core-kernel /v1/l5/* 5 RPC 配合,作为 wau-agent sandbox 外的元数据真相源。
-- D60 additive:0 改 v0.9.0 旧表 / 0 改 ProfileStore interface
-- ahead of W10 末 deadline: ~3 个月
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 表 1:installed_agents — 每个用户装了哪些 agent + version + sandbox 路径
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS installed_agents (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            VARCHAR(64)  NOT NULL,
    agent_name         VARCHAR(128) NOT NULL,
    agent_version      VARCHAR(32)  NOT NULL,
    manifest_yaml      JSONB        NOT NULL,  -- 完整 manifest 备份(per InstallAgentRequest.config)
    sandbox_docker_id  VARCHAR(128),           -- 当前运行的 Docker container (per D68 sandbox)
    enabled            BOOLEAN      DEFAULT true,
    installed_at       TIMESTAMPTZ  DEFAULT now(),
    uninstalled_at     TIMESTAMPTZ,            -- 软删除时间(默认 null)
    snapshot_path      VARCHAR(512),            -- 卸不丢数据的 snapshot 路径(per uninstall --purge=false)
    UNIQUE(user_id, agent_name, agent_version)
);
CREATE INDEX IF NOT EXISTS idx_installed_user     ON installed_agents(user_id);
CREATE INDEX IF NOT EXISTS idx_installed_enabled  ON installed_agents(user_id, enabled);
CREATE INDEX IF NOT EXISTS idx_installed_uninst   ON installed_agents(user_id, uninstalled_at) WHERE uninstalled_at IS NOT NULL;

COMMENT ON TABLE installed_agents IS
'v1.0.0 M11 P4.5 ⭐L5 installed agents (per D73, 2026-07-10) — 0 改 v0.9.0 旧表';

-- -----------------------------------------------------------------------------
-- 表 2:user_skill_pool — 每个用户的 skill 池(per-user enabled + version)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_skill_pool (
    id            BIGSERIAL PRIMARY KEY,
    user_id       VARCHAR(64)  NOT NULL,
    skill_name    VARCHAR(128) NOT NULL,
    skill_version VARCHAR(32)  NOT NULL,
    source_agent  VARCHAR(128),                -- 通过哪个 agent 注入的
    enabled       BOOLEAN      DEFAULT true,
    created_at    TIMESTAMPTZ  DEFAULT now(),
    UNIQUE(user_id, skill_name)
);
CREATE INDEX IF NOT EXISTS idx_skill_user    ON user_skill_pool(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_enabled ON user_skill_pool(user_id, enabled);

COMMENT ON TABLE user_skill_pool IS
'v1.0.0 M11 P4.5 ⭐L5 user skill pool (per D73) — per-user 维度的 skill 池';

-- -----------------------------------------------------------------------------
-- 表 3:agent_memory — per-user × per-agent key-value 存储
-- (类比 ~/Library/Application Support,卸不丢数据的关键机制)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_memory (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(64)  NOT NULL,
    agent_name  VARCHAR(128) NOT NULL,
    mem_key     VARCHAR(128) NOT NULL,
    mem_value   JSONB        NOT NULL,
    created_at  TIMESTAMPTZ  DEFAULT now(),
    updated_at  TIMESTAMPTZ  DEFAULT now(),
    UNIQUE(user_id, agent_name, mem_key)
);
CREATE INDEX IF NOT EXISTS idx_memory_user_agent ON agent_memory(user_id, agent_name);

COMMENT ON TABLE agent_memory IS
'v1.0.0 M11 P4.5 ⭐L5 agent memory (per D73) — per-user × per-agent KV 存储,卸不丢';

-- -----------------------------------------------------------------------------
-- Verification queries (for ops)
-- -----------------------------------------------------------------------------
-- SELECT count(*) FROM installed_agents WHERE user_id = 'alice' AND enabled = true;
-- SELECT count(*) FROM user_skill_pool WHERE user_id = 'alice' AND enabled = true;
-- SELECT count(*) FROM agent_memory WHERE user_id = 'alice' AND agent_name = 'weather-agent';
