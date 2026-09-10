-- 登录失败锁定：连续失败达到阈值后临时锁定账户（P0 安全加固）
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until timestamptz;
