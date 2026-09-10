package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"gorm.io/gorm"

	"dcim-lite/internal/app"
	"dcim-lite/internal/config"
	"dcim-lite/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("db sql: %v", err)
	}
	// 连接池上限：避免突发流量打满 Postgres 连接
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		if err := runMigrations(db, dir); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	users := repository.NewUserStore(db)
	if err := users.EnsureAdmin(cfg.AdminUser, cfg.AdminPass, cfg.AdminDisplayName); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := repository.NewDeviceStore(db).SeedDeviceTypesIfEmpty(); err != nil {
		log.Fatalf("seed device types: %v", err)
	}
	if err := repository.NewTemplateStore(db).SeedSystemTemplateIfEmpty(); err != nil {
		log.Fatalf("seed template: %v", err)
	}

	application := app.Build(db, cfg)

	// 生产级 HTTP 生命周期：显式超时 + 优雅停机（SIGTERM/SIGINT 内限期 drain）
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           application.Engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		log.Printf("dcim-lite listening on %s (env=%s)", cfg.HTTPAddr, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("shutting down (drain up to 10s)...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	_ = sqlDB.Close()
	log.Printf("stopped")
}

// runMigrations 在单个数据库事务内完成：建表、取事务级 advisory lock、逐个执行迁移、
// 记录 checksum，任一失败整体回滚（零部分应用状态）。
//
// 复评 P1-07 修复：此前的 session 级 pg_advisory_lock 通过连接池执行，锁、迁移、
// unlock 可能落在三个不同物理连接上，锁形同虚设。GORM 事务绑定单一连接，
// 事务级 xact lock 与迁移语句因此必然同连接，且随事务提交/回滚自动释放，无 unlock 泄漏。
// 前提：迁移 SQL 文件内不得包含事务控制语句（BEGIN/COMMIT），如需引入须改用独立 migration job。
func runMigrations(db *gorm.DB, dir string) error {
	var appliedList []string
	if err := db.Transaction(func(tx *gorm.DB) error {
		// 先取锁再做任何 DDL：CREATE TABLE IF NOT EXISTS 在多副本同时冷启动时
		// 存在 pg_type 目录并发插入的竞态（23505），必须在锁内串行执行
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(872341001)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			checksum text,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum text`).Error; err != nil {
			return err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
				continue
			}
			if strings.HasSuffix(e.Name(), ".down.sql") {
				continue
			}
			files = append(files, e.Name())
		}
		sort.Strings(files)

		for _, name := range files {
			body, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			sum := fmt.Sprintf("%x", sha256.Sum256(body))

			var applied struct {
				Version  string
				Checksum *string
			}
			if err := tx.Raw(`SELECT version, checksum FROM schema_migrations WHERE version = ?`, name).Scan(&applied).Error; err != nil {
				return err
			}
			if applied.Version != "" {
				// 已应用：内容被改动则拒绝启动，防止环境间 schema 漂移
				if applied.Checksum != nil && *applied.Checksum != sum {
					return fmt.Errorf("%s: already applied with different content (schema drift detected)", name)
				}
				// 历史行缺 checksum（如老版本写入）：以当前文件内容回填，避免永久 NULL
				if applied.Checksum == nil {
					if err := tx.Exec(`UPDATE schema_migrations SET checksum = ? WHERE version = ?`, sum, name).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Exec(string(body)).Error; err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if err := tx.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (?, ?)`, name, sum).Error; err != nil {
				return err
			}
			appliedList = append(appliedList, name)
		}
		return nil
	}); err != nil {
		return err
	}
	// 事务已提交，此时打印才是真实的
	for _, name := range appliedList {
		log.Printf("migration applied: %s", name)
	}
	return nil
}
