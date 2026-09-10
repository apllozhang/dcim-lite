package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// 迁移目录定位回归：CI 的仓库路径是嵌套的（/home/runner/work/<repo>/<repo>/…），
// 一旦查找逻辑写死相对层级就会在 CI 才炸，这里在无数据库环境下先行拦截。
func TestFindMigrationsDir(t *testing.T) {
	dir, err := FindMigrationsDir()
	if err != nil {
		t.Fatalf("FindMigrationsDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "0001_core.up.sql")); err != nil {
		t.Fatalf("marker not found under %s: %v", dir, err)
	}
	abs, _ := filepath.Abs(dir)
	t.Logf("resolved migrations dir: %s", abs)
}
