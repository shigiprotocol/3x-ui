package controller

import (
	"github.com/mhsanaei/3x-ui/v3/internal/aether"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// AetherController exposes the panel's Aether core: read its saved settings and
// live process status, save new settings, and restart Xray so a routed inbound
// picks up the change.
type AetherController struct {
	settingService service.SettingService
	xrayService    service.XrayService
}

// aetherStatusView is what the settings page renders: the saved config plus the
// live state of the core process.
type aetherStatusView struct {
	Settings aether.Settings `json:"settings"`
	Status   aether.Status   `json:"status"`
}

// NewAetherController creates the controller and registers its routes under
// /panel/api/aether.
func NewAetherController(g *gin.RouterGroup) *AetherController {
	a := &AetherController{}
	a.initRouter(g)
	return a
}

func (a *AetherController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/aether")

	g.GET("/status", a.status)
	g.POST("/settings", a.saveSettings)
	g.POST("/restart", a.restart)
}

func (a *AetherController) status(c *gin.Context) {
	settings, err := a.settingService.GetAetherSettings()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.getSettings"), err)
		return
	}
	jsonObj(c, aetherStatusView{
		Settings: settings,
		Status:   aether.GetManager().Status(),
	}, nil)
}

func (a *AetherController) saveSettings(c *gin.Context) {
	form, ok := middleware.BindAndValidate[aether.Settings](c)
	if !ok {
		return
	}
	if err := a.settingService.SetAetherSettings(*form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.saveSettings"), err)
		return
	}
	// Rebuild the config so the core reconciles to the new settings; the
	// generated outbound/routing follow in the same rebuild.
	if err := a.xrayService.RestartXray(false); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.restart"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.aether.toasts.saveSettings"), nil)
}

func (a *AetherController) restart(c *gin.Context) {
	if err := a.xrayService.RestartXray(true); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.restart"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.aether.toasts.restart"), nil)
}
