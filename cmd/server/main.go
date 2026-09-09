package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"dcim-lite/internal/config"
	"dcim-lite/internal/handler"
	"dcim-lite/internal/repository"
	"dcim-lite/internal/router"
	"dcim-lite/internal/service"
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

	resStore := repository.NewResourceStore(db)
	devStore := repository.NewDeviceStore(db)
	if err := devStore.SeedDeviceTypesIfEmpty(); err != nil {
		log.Fatalf("seed device types: %v", err)
	}
	tmplStore := repository.NewTemplateStore(db)
	if err := tmplStore.SeedSystemTemplateIfEmpty(); err != nil {
		log.Fatalf("seed template: %v", err)
	}
	pduStore := repository.NewPDUStore(db)
	apprStore := repository.NewApprovalStore(db)
	ldapStore := repository.NewLDAPStore(db)
	authSvc := service.NewAuthService(users, cfg.JWTSecret, cfg.JWTExpiresIn)
	captchaSvc := service.NewCaptchaService()
	resSvc := service.NewResourceService(resStore)
	devSvc := service.NewDeviceService(devStore, resStore)
	adminSvc := service.NewAdminService(users)
	tmplSvc := service.NewTemplateService(tmplStore, resStore)
	pduSvc := service.NewPDUService(pduStore, resStore, devStore)
	apprSvc := service.NewApprovalService(apprStore, devStore, devSvc)
	ldapSvc := service.NewLDAPService(ldapStore)
	importSvc := service.NewImportService(service.NewImportDraftStore(), devSvc)

	mode := gin.ReleaseMode
	if cfg.AppEnv == "development" {
		mode = gin.DebugMode
	}

	r := router.New(router.Deps{
		Secret:    cfg.JWTSecret,
		Users:     users,
		Health:    handler.NewHealthHandler(db),
		Auth:      handler.NewAuthHandler(authSvc, captchaSvc),
		Res:       handler.NewResourceHandler(resSvc),
		Device:    handler.NewDeviceHandler(devSvc, apprSvc),
		Admin:     handler.NewAdminHandler(adminSvc),
		Template:  handler.NewTemplateHandler(tmplSvc),
		PDU:       handler.NewPDUHandler(pduSvc),
		Approval:  handler.NewApprovalHandler(apprSvc),
		LDAP:      handler.NewLDAPHandler(ldapSvc),
		Import:    handler.NewImportHandler(importSvc),
		ImportTpl: handler.NewImportTemplateHandler(),
		GinMode:   mode,
	})

	log.Printf("dcim-lite listening on %s (env=%s)", cfg.HTTPAddr, cfg.AppEnv)
	if err := http.ListenAndServe(cfg.HTTPAddr, r); err != nil {
		log.Fatal(err)
	}
}

func runMigrations(db *gorm.DB, dir string) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`).Error; err != nil {
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
		var n int64
		if err := db.Raw(`SELECT count(*) FROM schema_migrations WHERE version = ?`, name).Scan(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		tx := db.Begin()
		if err := tx.Exec(string(body)).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit().Error; err != nil {
			return err
		}
		log.Printf("migration applied: %s", name)
	}
	return nil
}
