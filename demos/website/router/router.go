// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package router

import (
	"github.com/snail007/gmc/module/middleware/accesslog"
	httppprof "github.com/snail007/gmc/util/pprof"
	"strings"

	"github.com/snail007/gmc"

	"github.com/snail007/gmc/demos/website/controller"
)

func InitRouter(s *gmc.HTTPServer) {

	//enable http pprof
	httppprof.BindRouter(s.Router(), "/gmcdebug")

	// sets pre routing handler, it be called with any request.
	s.AddMiddleware0(filterAll)

	// sets post routing handler, it be called only when url's path be found in router.
	s.AddMiddleware1(filter)

	s.AddMiddleware2(logging)

	// middleware: accesslog
	s.AddMiddleware3(accesslog.NewFromConfig(s.Config()))

	// acquire router object
	r := s.Router()
	r.Ext(".json")

	// bind a controller, /demo is path of controller, after this you can visit http://127.0.0.1:7080/demo/hello
	// "hello" is full lower case name of controller method.
	r.Controller("/demo", new(controller.Demo))
	r.ControllerMethod("/", new(controller.Demo), "Index__")
	r.ControllerMethod("/index.html", new(controller.Demo), "Index__")

	// indicates router initialized
	s.Logger().Infof("router inited.")
}

func filterAll(c gmc.C) bool {
	c.WebServer().Logger().Infof(c.Request().RequestURI)
	return false
}

func filter(c gmc.C) bool {
	path := strings.TrimRight(c.Request().URL.Path, "/\\")

	// we want to prevent user to access method `controller.Demo.Protected`
	if strings.Contains(path, "protected") {
		c.Write([]byte("404"))
		return true
	}

	//server.Logger().Printf("%v %s",c.TimeUsed(),path)
	return false
}

func logging(c gmc.C) bool {
	c.WebServer().Logger().Infof("after request %s %d %d %s %s", c.Request().Method, c.StatusCode(), c.WriteCount(), c.TimeUsed(), c.Request().RequestURI)
	return false
}
