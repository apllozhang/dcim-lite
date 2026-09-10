// Package app 负责应用装配：数据库 -> 仓储/服务 -> 路由引擎。
// main 与集成测试共用同一装配，保证被测对象与生产一致。
package app

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"dcim-lite/internal/config"
	"dcim-lite/internal/handler"
	"dcim-lite/internal/middleware"
	"dcim-lite/internal/repository"
	"dcim-lite/internal/router"
	"dcim-lite/internal/service"
)

type App struct {
	Engine   *gin.Engine
	Revoker  *middleware.MemoryTokenRevoker
	Stores   *Stores
	Services *Services
}

type Stores struct {
	Users *repository.UserStore
	Res   *repository.ResourceStore
	Dev   *repository.DeviceStore
	Tmpl  *repository.TemplateStore
	PDU   *repository.PDUStore
	Aprv  *repository.ApprovalStore
	LDAP  *repository.LDAPStore
}

type Services struct {
	Auth   *service.AuthService
	Res    *service.ResourceService
	Dev    *service.DeviceService
	Admin  *service.AdminService
	Tmpl   *service.TemplateService
	PDU    *service.PDUService
	Aprv   *service.ApprovalService
	Import *service.ImportService
}

// Build 装配完整应用（不含 HTTP server 生命周期与迁移）。
func Build(db *gorm.DB, cfg *config.Config) *App {
	users := repository.NewUserStore(db)
	resStore := repository.NewResourceStore(db)
	devStore := repository.NewDeviceStore(db)
	tmplStore := repository.NewTemplateStore(db)
	pduStore := repository.NewPDUStore(db)
	apprStore := repository.NewApprovalStore(db)
	ldapStore := repository.NewLDAPStore(db)

	revoker := middleware.NewMemoryTokenRevoker()
	authSvc := service.NewAuthService(users, cfg.JWTSecret, cfg.JWTExpiresIn, revoker)
	captchaSvc := service.NewCaptchaService()
	resSvc := service.NewResourceService(resStore)
	devSvc := service.NewDeviceService(devStore, resStore)
	adminSvc := service.NewAdminService(users)
	tmplSvc := service.NewTemplateService(tmplStore, resStore)
	pduSvc := service.NewPDUService(pduStore, resStore, devStore)
	apprSvc := service.NewApprovalService(apprStore, devStore, devSvc)
	importSvc := service.NewImportService(service.NewImportDraftStore(), devSvc)

	mode := gin.ReleaseMode
	if cfg.AppEnv == "development" {
		mode = gin.DebugMode
	}

	engine := router.New(router.Deps{
		Secret:          cfg.JWTSecret,
		LoginRatePerMin: cfg.LoginRatePerMin,
		Users:           users,
		Revoker:         revoker,
		Audit:           resStore,
		Health:          handler.NewHealthHandler(db),
		Auth:            handler.NewAuthHandler(authSvc, captchaSvc),
		Res:             handler.NewResourceHandler(resSvc),
		Device:          handler.NewDeviceHandler(devSvc, apprSvc),
		Admin:           handler.NewAdminHandler(adminSvc),
		Template:        handler.NewTemplateHandler(tmplSvc),
		PDU:             handler.NewPDUHandler(pduSvc),
		Approval:        handler.NewApprovalHandler(apprSvc),
		LDAP:            handler.NewLDAPHandler(service.NewLDAPService(ldapStore)),
		Import:          handler.NewImportHandler(importSvc),
		ImportTpl:       handler.NewImportTemplateHandler(),
		GinMode:         mode,
	})

	return &App{
		Engine:   engine,
		Revoker:  revoker,
		Stores:   &Stores{Users: users, Res: resStore, Dev: devStore, Tmpl: tmplStore, PDU: pduStore, Aprv: apprStore, LDAP: ldapStore},
		Services: &Services{Auth: authSvc, Res: resSvc, Dev: devSvc, Admin: adminSvc, Tmpl: tmplSvc, PDU: pduSvc, Aprv: apprSvc, Import: importSvc},
	}
}
