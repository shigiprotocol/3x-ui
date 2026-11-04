package controller

import (
	"github.com/mhsanaei/3x-ui/v3/internal/aether"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// AetherController exposes the panel's Aether profiles: read the saved profiles
// and the live state of each core, save the profile list, and restart Xray so
// routed inbounds pick up the change.
type AetherController struct {
	settingService service.SettingService
	xrayService    service.XrayService
}

// aetherView is what the settings page renders: the saved profiles plus the live
// state of each running core.
type aetherView struct {
	Profiles []aether.Profile       `json:"profiles"`
	Statuses []aether.ProfileStatus `json:"statuses"`
}

// aetherProfilesForm is the save payload: the whole profile list.
type aetherProfilesForm struct {
	Profiles []aether.Profile `json:"profiles"`
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
	profiles, err := a.settingService.GetAetherProfiles()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.getSettings"), err)
		return
	}
	jsonObj(c, aetherView{
		Profiles: profiles,
		Statuses: aether.GetManager().Statuses(),
	}, nil)
}

func (a *AetherController) saveSettings(c *gin.Context) {
	form, ok := middleware.BindAndValidate[aetherProfilesForm](c)
	if !ok {
		return
	}
	if err := a.settingService.SetAetherProfiles(form.Profiles); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.aether.toasts.saveSettings"), err)
		return
	}
	// Rebuild the config so the cores reconcile to the new profiles; the
	// generated outbounds/routing follow in the same rebuild.
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
