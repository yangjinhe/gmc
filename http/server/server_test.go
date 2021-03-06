// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package ghttpserver

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"github.com/snail007/gmc/core"
	gcontroller "github.com/snail007/gmc/http/controller"
	gsession "github.com/snail007/gmc/http/session"

	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//go test -coverprofile=a.out;go tool cover -html=a.out;rm a.out;

func TestNew(t *testing.T) {
	assert := assert.New(t)
	cfg := mockConfig()
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(cfg)
	s.bind(":")
	err := s.Listen()
	assert.Nil(err)
	addr := s.Listener().Addr().String()
	s0 := NewHTTPServer(gcore.Providers.Ctx("")())
	s0.Init(cfg)
	s0.bind(addr)
	err = s0.Listen()
	assert.NotNil(err)
}
func TestRouting(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.AddMiddleware0(func(ctx gcore.Ctx) (isStop bool) {
		ctx.Response().Write([]byte("error"))
		return true
	})
	s.router.HandlerFunc("GET", "/routing", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("routing"))
	})
	w, r := mockRequest("/routing")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("error", str)
}

func TestRouting_1(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.AddMiddleware1(func(ctx gcore.Ctx) (isStop bool) {
		ctx.Response().Write([]byte("error"))
		return true
	})
	s.router.HandlerFunc("GET", "/routing", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("routing"))
	})
	w, r := mockRequest("/routing")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("error", str)
}
func Test_handle50x(t *testing.T) {
	assert := assert.New(t)
	w := httptest.NewRecorder()
	ctx := gcore.Providers.Ctx("")()
	ctx.SetResponse(w)
	ctx.SetRequest(httptest.NewRequest("GET", "http://example.com/foo", nil))
	cfg := ctx.Config()
	controller := gcore.Providers.Controller("")(ctx)
	s := NewHTTPServer(ctx)
	assert.NotNil(cfg)
	err := s.Init(cfg)
	assert.Nil(err)
	s.handle50x(gcore.Providers.Ctx("")().CloneWithHTTP(w, controller.GetCtx().Request()), fmt.Errorf("aaa"))

	//response
	resp := w.Result()
	b, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(resp.StatusCode, int(http.StatusInternalServerError))
	assert.Equal("Internal Server Error", string(b))
}
func Test_handle50x_1(t *testing.T) {
	assert := assert.New(t)
	w := httptest.NewRecorder()
	ctx := gcore.Providers.Ctx("")()
	ctx.SetResponse(w)
	ctx.SetRequest(httptest.NewRequest("GET", "http://example.com/foo", nil))
	cfg := ctx.Config()
	controller := gcore.Providers.Controller("")(ctx)
	s := NewHTTPServer(ctx)
	assert.NotNil(cfg)
	err := s.Init(cfg)
	assert.Nil(err)
	s.SetErrorHandler(func(c gcore.Ctx, tpl gcore.Template, err interface{}) {
		c.Write(fmt.Errorf("%sbbb", err))
	})
	c := gcore.Providers.Ctx("")().CloneWithHTTP(w, controller.GetCtx().Request())
	s.handle50x(c, fmt.Errorf("aaa"))

	//response
	resp := w.Result()
	b, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(resp.StatusCode, int(http.StatusOK))
	assert.Equal("aaabbb", string(b))
}

func TestConnCount(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.Listen()
	_, port, _ := net.SplitHostPort(s.listener.Addr().String())
	transport := http.Transport{
		DisableKeepAlives: true,
	}
	client := http.Client{
		Transport: &transport,
	}
	resp, err := client.Get("http://127.0.0.1:" + port)
	client.CloseIdleConnections()
	transport.CloseIdleConnections()
	time.Sleep(time.Millisecond * 100)
	assert.Equal(int64(0), s.ActiveConnCount())
	assert.Nil(err)
	b, _ := ioutil.ReadAll(resp.Body)
	assert.Equal("Page not found", string(b))
}
func TestConnCount_2(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.Listen()
	_, port, _ := net.SplitHostPort(s.listener.Addr().String())
	client := http.Client{}
	client.Get("http://127.0.0.1:" + port)
	assert.Equal(int64(1), s.ActiveConnCount())
}

func TestInit_0(t *testing.T) {
	assert := assert.New(t)
	cfg := gcore.Providers.Config("")()
	cfg.SetConfigFile("../../module/app/app.toml")
	err := cfg.ReadInConfig()
	assert.Nil(err)
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(cfg)
	s.bind(":")
	err = s.Listen()
	assert.Nil(err)
}
func TestInit_1(t *testing.T) {
	assert := assert.New(t)
	cfg := gcore.Providers.Config("")()
	cfg.SetConfigFile("../../module/app/app.toml")
	err := cfg.ReadInConfig()
	assert.Nil(err)
	cfg.Set("session.store", "file")
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(cfg)
	s.bind(":")
	err = s.Listen()
	assert.Nil(err)
}
func TestInit_2(t *testing.T) {
	assert := assert.New(t)
	cfg := gcore.Providers.Config("")()
	cfg.SetConfigFile("../../module/app/app.toml")
	err := cfg.ReadInConfig()
	assert.Nil(err)
	cfg.Set("session.store", "redis")
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(cfg)
	s.bind(":")
	err = s.Listen()
	assert.Nil(err)
}
func TestInit_3(t *testing.T) {
	assert := assert.New(t)
	cfg := mockConfig()
	cfg.Set("session.store", "none")
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	err := s.Init(cfg)
	assert.Equal("unknown session store type none", err.Error())
}
func TestInit_4(t *testing.T) {
	assert := assert.New(t)
	cfg := gcore.Providers.Config("")()
	cfg.SetConfigFile("../../module/app/app.toml")
	err := cfg.ReadInConfig()
	assert.Nil(err)
	cfg.Set("session.store", "")
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(cfg)
	s.bind(":")
	err = s.Listen()
	assert.Nil(err)
}
func TestHelper(t *testing.T) {
	assert := assert.New(t)
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(gcore.Providers.Config("")())
	s.bind(":")
	err := s.Listen()
	assert.Nil(err)
	assert.NotNil(s.Server().Addr)
	s.SetLogger(gcore.Providers.Logger("")(s.ctx, ""))
	assert.NotNil(s.logger)
	r := gcore.Providers.HTTPRouter("")(s.ctx)
	s.SetRouter(r)
	assert.NotNil(s.router)
	tpl, err := gcore.Providers.Template("")(s.ctx, "views")
	assert.Nil(err)
	//tpl, err := gtemplate.NewTemplate(s.ctx,"views")
	//assert.Nil(err)
	s.SetTpl(tpl)
	assert.NotNil(s.tpl)
	st, err := gsession.NewMemoryStore(gsession.NewMemoryStoreConfig())
	assert.Nil(err)
	s.SetSessionStore(st)
	assert.NotNil(s.sessionStore)
	s.Close()
}

func TestGetter(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	assert.NotNil(s.Config())
	assert.NotNil(s.Logger())
	assert.NotNil(s.Router())
	assert.NotNil(s.Tpl())
	assert.NotNil(s.SessionStore())
}
func TestListenTLS(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)
	s.config.Set("httpserver.tlsclientauth", true)
	s.config.Set("httpserver.tlsclientsca", "server.crt")
	err := s.initTLSConfig()
	assert.Nil(err)
	err = s.ListenTLS()
	assert.Nil(err)
	s.Close()
}
func TestListenTLS_0(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)
	s.config.Set("httpserver.tlsclientauth", true)
	s.config.Set("httpserver.tlsclientsca", "server.crt")
	err := s.initTLSConfig()
	assert.Nil(err)
	err = s.ListenTLS()
	assert.Nil(err)
	s.isTestNotClosedError = true
	s.Close()
}
func TestListenTLS_1(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)
	s.config.Set("httpserver.tlsclientauth", true)
	s.config.Set("httpserver.tlsclientsca", "none.crt")
	err := s.initTLSConfig()
	assert.NotNil(err)
}
func TestListenTLS_2(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)
	s.config.Set("httpserver.tlsclientauth", true)
	s.config.Set("httpserver.tlsclientsca", "server.key")
	err := s.initTLSConfig()
	assert.NotNil(err)
	s.initBaseObjets()
}
func TestListen(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	err := s.Listen()
	assert.Nil(err)
	s0 := mockHTTPServer()
	s0.addr = s.listener.Addr().String()
	err = s0.Listen()
	assert.NotNil(err)
}
func TestListen_1(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	err := s.Listen()
	assert.Nil(err)
	s.isTestNotClosedError = true
	s.Close()
}
func TestListen_2(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)
	err := s.ListenTLS()
	assert.Nil(err)

	s0 := mockHTTPServer()
	s.SetConfig(mockConfig())
	s.config.Set("httpserver.listen", ":")
	s.config.Set("httpserver.tlscert", "server.crt")
	s.config.Set("httpserver.tlskey", "server.key")
	s.config.Set("httpserver.tlsenable", true)

	s0.addr = s.listener.Addr().String()
	err = s0.ListenTLS()
	assert.NotNil(err)
}

func Test_handler40x(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.SetNotFoundHandler(func(ctx gcore.Ctx, tpl gcore.Template) {
		ctx.Response().Write([]byte("404"))
	})
	w, r := mockRequest("/foo")
	s.handle40x(gcore.Providers.Ctx("")().CloneWithHTTP(w, r, nil))
	str, _ := result(w)
	assert.Equal("404", str)
}

func Test_Handle(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.router.HandleAny("/user/url", func(w http.ResponseWriter, r *http.Request, params gcore.Params) {
		c := gcore.Providers.Ctx("")().CloneWithHTTP(w, r)
		c.Write("/user/url")
	})
	h, _, _ := s.router.Lookup("GET", "/user/url")
	assert.NotNil(h)
	w, r := mockRequest("/user/url")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("/user/url", str)
}

func Test_Handle500(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.router.HandlerFunc("GET", "/user/url", func(w http.ResponseWriter, r *http.Request) {
		a := 0
		a /= a
	})
	s.SetErrorHandler(func(ctx gcore.Ctx, tpl gcore.Template, err interface{}) {
		ctx.Write("500")
	})
	h, _, _ := s.router.Lookup("GET", "/user/url")
	assert.NotNil(h)
	w, r := mockRequest("/user/url")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("500", str)
}

func Test_SetBindata(t *testing.T) {
	assert := assert.New(t)
	str := base64.StdEncoding.EncodeToString([]byte("{}"))
	SetBinData(map[string]string{
		"js/juqery.js": str,
	})
	assert.NotNil(bindata["js/juqery.js"])
	s := mockHTTPServer()
	w, r := mockRequest("/static/js/juqery.js")
	r.Header.Set("Accept-Encoding", "gzip")
	s.initStatic()
	s.serveStatic(w, r)
	resp := w.Result()
	b, e := ioutil.ReadAll(resp.Body)
	assert.Nil(e)
	gz, e := gzip.NewReader(bytes.NewReader(b))
	assert.Nil(e)
	buf := make([]byte, 128)
	n, _ := gz.Read(buf)
	assert.Equal("{}", string(buf[:n]))
	bindata = nil
}
func Test_SetBindata_1(t *testing.T) {
	assert := assert.New(t)
	cfg := mockConfig()
	cfg.Set("static.urlpath", "/static")
	s := mockHTTPServer(cfg)
	w, r := mockRequest("/static/js/juqery.js")
	r.Header.Set("Accept-Encoding", "gzip")
	s.serveStatic(w, r)
	resp := w.Result()
	assert.Equal(404, resp.StatusCode)
}

func Test_SetBindata_2(t *testing.T) {
	assert := assert.New(t)
	str := base64.StdEncoding.EncodeToString([]byte("{}"))
	SetBinData(map[string]string{
		"js/juqery.js": str,
	})
	assert.NotNil(bindata["js/juqery.js"])
	s := mockHTTPServer()
	w, r := mockRequest("/static/js/juqery.js")
	s.serveStatic(w, r)
	resp := w.Result()
	b, _ := ioutil.ReadAll(resp.Body)
	assert.Equal("{}", string(b))
	bindata = nil
}
func Test_SetBindata_3(t *testing.T) {
	assert := assert.New(t)
	assert.Panics(func() {
		SetBinData(map[string]string{
			"/js/juqery.js": ".",
		})
	})
}
func Test_SetBindata_4(t *testing.T) {
	assert := assert.New(t)
	cfg := mockConfig()
	cfg.Set("static.dir", "tests")
	cfg.Set("static.urlpath", "/static")
	s := mockHTTPServer(cfg)
	w, r := mockRequest("/static/d.txt")
	r.Header.Set("Accept-Encoding", "gzip")
	s.serveStatic(w, r)
	resp := w.Result()
	b, _ := ioutil.ReadAll(resp.Body)
	assert.Equal("d", string(b))
}
func Test_Service(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s0 := (gcore.Service)(s)
	err := s0.Start()
	assert.Nil(err)
	s0.Stop()
	s0.GracefulStop()
}
func Test_Service_2(t *testing.T) {
	assert := assert.New(t)
	cfg := mockConfig()
	cfg.Set("httpserver.tlsenable", true)
	s := mockHTTPServer(cfg)
	s0 := (gcore.Service)(s)
	err := s0.Start()
	assert.Nil(err)
	s.SetLog(s.logger)
}
func Test_PrintRouteTable(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.router.HandlerFunc("GET", "/", func(http.ResponseWriter, *http.Request) {})
	var b bytes.Buffer
	s.PrintRouteTable(&b)
	assert.Containsf(b.String(), "GET", "")
}
func result(w *httptest.ResponseRecorder) (str string, resp *http.Response) {
	resp = w.Result()
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	str = string(body)
	return
}

func Test_Controller(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.router.Controller("/user/", new(User))
	h, _, _ := s.router.Lookup("GET", "/user/url")
	assert.NotNil(h)
	w, r := mockRequest("/user/url")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("/user/url", str)
}
func Test_Controller_Args(t *testing.T) {
	assert := assert.New(t)
	s := mockHTTPServer()
	s.router.ControllerMethod("/user/:args", new(User), "Ps")
	h, _, _ := s.router.Lookup("GET", "/user/hello")
	assert.NotNil(h)
	w, r := mockRequest("/user/hello")
	s.ServeHTTP(w, r)
	str, _ := result(w)
	assert.Equal("hello/user/:args", str)
}

type User struct {
	gcontroller.Controller
}

func (this *User) URL() {
	a := "a"
	if this.Param != nil {
		a = ""
	}
	this.Write(this.Request.URL.Path + a)
}
func (this *User) Ps() {
	this.Write(this.Param.ByName("args") + this.Param.MatchedRoutePath())
}

func mockConfig() gcore.Config {
	cfg := gcore.Providers.Config("")()
	cfg.SetConfigFile("../../module/app/app.toml")
	cfg.ReadInConfig()
	cfg.Set("template.dir", "")
	return cfg
}

func mockRequest(uri string) (w *httptest.ResponseRecorder, r *http.Request) {
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://example.com"+uri, nil)
	return
}
func mockResponse(w *httptest.ResponseRecorder) (data string, resp *http.Response, err error) {
	resp = w.Result()
	b, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		data = string(b)
	}
	return
}
func mockHTTPServer(cfg ...gcore.Config) *HTTPServer {
	c := mockConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	s := NewHTTPServer(gcore.Providers.Ctx("")())
	s.Init(c)
	s.bind(":")
	s.SetRouter(gcore.Providers.HTTPRouter("")(s.ctx))
	s.SetLogger(gcore.Providers.Logger("")(s.ctx, ""))
	st, _ := gsession.NewMemoryStore(gsession.NewMemoryStoreConfig())
	s.SetSessionStore(st)
	tpl, _ := gcore.Providers.Template("")(s.ctx, "../template/tests/views")
	s.SetTpl(tpl)
	return s
}
