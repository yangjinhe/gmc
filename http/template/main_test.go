package gtemplate

import (
	gcore "github.com/snail007/gmc/core"
	grouter "github.com/snail007/gmc/http/router"
	gsession "github.com/snail007/gmc/http/session"
	gview "github.com/snail007/gmc/http/view"
	gconfig "github.com/snail007/gmc/module/config"
	gctx "github.com/snail007/gmc/module/ctx"
	gi18n "github.com/snail007/gmc/module/i18n"
	gutil "github.com/snail007/gmc/util"
	"io"
	"os"
	"testing"
)

var (
	tpl *Template
)

func TestMain(m *testing.M) {


	providers := gcore.Providers

	providers.RegisterSession("", func() gcore.Session {
		return gsession.NewSession()
	})

	providers.RegisterSessionStorage("", func(ctx gcore.Ctx) (gcore.SessionStorage, error) {
		return gsession.Init(ctx.Config())
	})

	providers.RegisterView("", func(w io.Writer, tpl gcore.Template) gcore.View {
		return gview.New(w, tpl)
	})

	providers.RegisterTemplate("", func(ctx gcore.Ctx) (gcore.Template, error) {
		return  Init(ctx)
	})

	providers.RegisterHTTPRouter("", func(ctx gcore.Ctx) gcore.HTTPRouter {
		return grouter.NewHTTPRouter(ctx)
	})

	providers.RegisterConfig("", func() gcore.Config {
		return gconfig.NewConfig()
	})

	providers.RegisterI18n("", func(ctx gcore.Ctx) (gcore.I18n, error) {
		var err error
		gutil.OnceDo("gmc-i18n-init", func() {
			err = gi18n.Init(ctx.Config())
		})
		return gi18n.I18N, err
	})

	providers.RegisterCtx("", func() gcore.Ctx {
		return gctx.NewCtx()
	})

	ctx := gcore.Providers.Ctx("")()
	ctx.SetConfig(gcore.Providers.Config("")())
	tpl,_=NewTemplate(ctx,"tests/views")
	tpl.Delims("{{", "}}")
	tpl.Parse()
	os.Exit(m.Run())
}