// 迁移目录与通用路径定位工具（无构建标签：普通单测与集成测试共用）。
package integration

import (
	"fmt"
	"os"
	"path/filepath"
)

const migrationMarker = "0001_core.up.sql"

// FindMigrationsDir 定位 migrations 目录，兼容三种运行环境：
//   - go test ./internal/integration/（CWD=包目录，需向上找到仓库根）
//   - CI 嵌套检出（/home/runner/work/<repo>/<repo>/...）
//   - 编译后的测试二进制（CWD=部署目录，migrations 与二进制同级）
func FindMigrationsDir() (string, error) {
	var tried []string
	var bases []string
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, wd)
		up := wd
		for i := 0; i < 5; i++ {
			parent := filepath.Dir(up)
			if parent == up {
				break
			}
			up = parent
			bases = append(bases, up)
		}
	}
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}
	for _, base := range bases {
		dir := filepath.Join(base, "migrations")
		tried = append(tried, dir)
		if _, err := os.Stat(filepath.Join(dir, migrationMarker)); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found; tried: %v", tried)
}
