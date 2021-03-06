// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package gtemplate

import (
	gcore "github.com/snail007/gmc/core"
	grouter "github.com/snail007/gmc/http/router"
	gsession "github.com/snail007/gmc/http/session"
	gview "github.com/snail007/gmc/http/view"
	gconfig "github.com/snail007/gmc/module/config"
	gctx "github.com/snail007/gmc/module/ctx"
	gi18n "github.com/snail007/gmc/module/i18n"
	glog "github.com/snail007/gmc/module/log"
	"io"
	"os"
	"sync"
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

	providers.RegisterTemplate("", func(ctx gcore.Ctx, rootDir string) (gcore.Template, error) {
		if ctx.Config().Sub("template") != nil {
			return Init(ctx)
		}
		return NewTemplate(ctx, rootDir)
	})

	providers.RegisterHTTPRouter("", func(ctx gcore.Ctx) gcore.HTTPRouter {
		return grouter.NewHTTPRouter(ctx)
	})

	providers.RegisterConfig("", func() gcore.Config {
		return gconfig.NewConfig()
	})

	providers.RegisterI18n("", func(ctx gcore.Ctx) (gcore.I18n, error) {
		var err error
		OnceDo("gmc-i18n-init", func() {
			err = gi18n.Init(ctx.Config())
		})
		return gi18n.I18N, err
	})

	providers.RegisterLogger("", func(ctx gcore.Ctx, prefix string) gcore.Logger {
		if ctx == nil {
			return glog.NewLogger(prefix)
		}
		return glog.NewFromConfig(ctx.Config(), prefix)
	})

	providers.RegisterCtx("", func() gcore.Ctx {
		return gctx.NewCtx()
	})

	ctx := gcore.Providers.Ctx("")()
	ctx.SetConfig(gcore.Providers.Config("")())
	tpl, _ = NewTemplate(ctx, "tests/views")
	tpl.Delims("{{", "}}")
	tpl.Parse()
	os.Exit(m.Run())
}

var onceDoDataMap = sync.Map{}

func OnceDo(uniqueKey string, f func()) {
	once, _ := onceDoDataMap.LoadOrStore(uniqueKey, &sync.Once{})
	once.(*sync.Once).Do(f)
	return
}
