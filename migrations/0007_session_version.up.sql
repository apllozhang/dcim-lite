-- 会话版本：密码重置/停用/删除用户时原子递增，使该用户全部已签发 JWT 立即失效
-- （旧 token 的 sv 与库中值不等即 401；复评 P1-11 / §10「密码重置撤销」）
ALTER TABLE users ADD COLUMN IF NOT EXISTS session_version bigint NOT NULL DEFAULT 1;
