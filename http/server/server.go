package ghttpserver

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	gcore "github.com/snail007/gmc/core"
	grouter "github.com/snail007/gmc/http/router"
	gfilestore "github.com/snail007/gmc/http/session/filestore"
	gmemorystore "github.com/snail007/gmc/http/session/memorystore"
	gredisstore "github.com/snail007/gmc/http/session/redisstore"
	gtemplate "github.com/snail007/gmc/http/template"
	log2 "github.com/snail007/gmc/module/log"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gerr "github.com/snail007/gmc/module/error"
	gconfig "github.com/snail007/gmc/util/config"
)

var (
	bindata = map[string][]byte{}
)

func SetBinData(data map[string]string) {
	bindata = map[string][]byte{}
	for k, v := range data {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			panic("init static bin data fail, error: " + err.Error())
		}
		bindata[k] = b
	}
}

type HTTPServer struct {
	tpl          gcore.Template
	sessionStore gcore.SessionStorage
	router       gcore.HTTPRouter
	logger       gcore.Logger
	addr         string
	listener     net.Listener
	server       *http.Server
	connCnt      *int64
	config       *gconfig.Config
	handler40x   func(ctx gcore.Ctx, tpl gcore.Template)
	handler50x   func(ctx gcore.Ctx, tpl gcore.Template, err interface{})
	//just for testing
	isTestNotClosedError bool
	staticDir            string
	staticUrlpath        string
	middleware0          []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)
	middleware1          []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)
	middleware2          []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)
	middleware3          []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)
	isShutdown           bool
	localaddr            *sync.Map
	ctx                  gcore.Ctx
}

func NewHTTPServer(ctx gcore.Ctx) *HTTPServer {
	s := &HTTPServer{
		ctx:         ctx,
		middleware0: []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool){},
		middleware1: []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool){},
		middleware2: []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool){},
		middleware3: []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool){},
		localaddr:   &sync.Map{},
	}
	ctx.SetWebServer(s)
	return s
}

//Init implements service.Service Init
func (s *HTTPServer) Init(cfg *gconfig.Config, ctx gcore.Ctx) (err error) {
	connCnt := int64(0)
	ctx.SetWebServer(s)
	s.server = &http.Server{}
	s.logger = log2.NewGMCLog()
	s.connCnt = &connCnt
	s.config = cfg
	s.isTestNotClosedError = false
	s.server.ConnState = s.connState
	s.server.Handler = s
	s.ctx = ctx

	//init base objects
	err = s.initBaseObjets()
	return
}
func (s *HTTPServer) initBaseObjets() (err error) {

	// init template
	s.tpl, err = gtemplate.New(s.config.GetString("template.dir"))
	if err != nil {
		return
	}
	s.tpl.Delims(s.config.GetString("template.delimiterleft"),
		s.config.GetString("template.delimiterright")).
		Extension(s.config.GetString("template.ext"))

	// init session store
	err = s.initSessionStore()
	if err != nil {
		return
	}

	// init http server tls configuration
	err = s.initTLSConfig()
	if err != nil {
		return
	}

	// init http server router
	s.router = grouter.NewHTTPRouter(s.ctx)
	s.addr = s.config.GetString("httpserver.listen")

	// init static files handler, must be after router inited
	s.initStatic()
	return
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// init ctx
	c0 := s.ctx.CloneWithHTTP(w, r)
	addr, _ := s.localaddr.Load(r.RemoteAddr)
	if v, ok := addr.(string); ok {
		c0.SetLocalAddr(v)
	}

	defer func() {
		// middleware3
		s.callMiddleware(c0, s.middleware3)
	}()

	//middleware0
	if s.callMiddleware(c0, s.middleware0) {
		return
	}

	h, params, _ := s.router.Lookup(r.Method, r.URL.Path)
	if h != nil {
		c0.SetParam(params)
		// middleware1
		if s.callMiddleware(c0, s.middleware1) {
			return
		}

		start := time.Now()
		status := ""
		err := s.call(func() { h(w, r, params) })
		c0.SetTimeUsed(time.Now().Sub(start))
		if err != nil {
			status = fmt.Sprintf("%s", err)
			switch status {
			case "__DIE__", "":
			default:
				//exception
				s.handle50x(c0, err)
			}
		}

		// middleware2
		if s.callMiddleware(c0, s.middleware2) {
			return
		}
	} else {
		//404
		s.handle40x(c0)
	}

}
func (s *HTTPServer) call(fn func()) (err interface{}) {
	func() {
		defer gcore.Recover(func(e interface{}) {
			err = gerr.Wrap(e)
		})
		fn()
	}()
	return
}
func (s *HTTPServer) SetHandler40x(fn func(ctx gcore.Ctx, tpl gcore.Template)) {
	s.handler40x = fn
}
func (s *HTTPServer) SetHandler50x(fn func(ctx gcore.Ctx, tpl gcore.Template, err interface{})) {
	s.handler50x = fn
}

// called in httpserver
func (s *HTTPServer) handle40x(ctx gcore.Ctx) {
	if s.handler40x == nil {
		ctx.Response().WriteHeader(http.StatusNotFound)
		ctx.Response().Write([]byte("Page not found"))
	} else {
		s.handler40x(ctx, s.tpl)
	}
	return
}

// called in httprouter
func (s *HTTPServer) handle50x(c gcore.Ctx, err interface{}) {
	if s.handler50x == nil {
		c.WriteHeader(http.StatusInternalServerError)
		c.Response().Header().Set("Content-Type", "text/plain")
		c.Write([]byte("Internal Server Error"))
		if err != nil && s.config.GetBool("httpserver.showerrorstack") {
			c.Write([]byte("\n" + gerr.Stack(err)))
		}
	} else {
		s.handler50x(c, s.tpl, err)
	}
}

// AddFuncMap adds helper functions to template
func (s *HTTPServer) AddFuncMap(f map[string]interface{}) {
	s.tpl.Funcs(f)
}

func (s *HTTPServer) SetConfig(c *gconfig.Config) {
	s.config = c
}
func (s *HTTPServer) Config() *gconfig.Config {
	return s.config
}

func (s *HTTPServer) ActiveConnCount() int64 {
	return atomic.LoadInt64(s.connCnt)
}
func (s *HTTPServer) Close() {
	s.server.Close()
}
func (s *HTTPServer) Listener() net.Listener {
	return s.listener
}

//Listeners implements service.Service Listeners
func (s *HTTPServer) Listeners() []net.Listener {
	return []net.Listener{s.listener}
}

//InjectListeners implements service.Service InjectListeners
func (s *HTTPServer) InjectListeners(l []net.Listener) {
	s.listener = l[0]
}
func (s *HTTPServer) Server() *http.Server {
	return s.server
}
func (s *HTTPServer) SetLogger(l gcore.Logger) {
	s.logger = l
	s.server.ErrorLog = func() *log.Logger {
		ns := s.logger.Namespace()
		if ns != "" {
			ns = "[" + ns + "]"
		}
		l := log.New(s.logger.Writer(), ns, log.Lmicroseconds|log.LstdFlags)
		return l
	}()
}
func (s *HTTPServer) Logger() gcore.Logger {
	return s.logger
}
func (s *HTTPServer) SetRouter(r gcore.HTTPRouter) {
	s.router = r
}
func (s *HTTPServer) Router() gcore.HTTPRouter {
	return s.router
}
func (s *HTTPServer) SetTpl(t gcore.Template) {
	s.tpl = t
}
func (s *HTTPServer) Tpl() gcore.Template {
	return s.tpl
}
func (s *HTTPServer) SetSessionStore(st gcore.SessionStorage) {
	s.sessionStore = st
}
func (s *HTTPServer) SessionStore() gcore.SessionStorage {
	return s.sessionStore
}
func (s *HTTPServer) AddMiddleware0(m func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)) {
	s.middleware0 = append(s.middleware0, m)
}
func (s *HTTPServer) AddMiddleware1(m func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)) {
	s.middleware1 = append(s.middleware1, m)
}
func (s *HTTPServer) AddMiddleware2(m func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)) {
	s.middleware2 = append(s.middleware2, m)
}
func (s *HTTPServer) AddMiddleware3(m func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)) {
	s.middleware3 = append(s.middleware3, m)
}

//just for testing
func (s *HTTPServer) bind(addr string) {
	s.addr = addr
}
func (s *HTTPServer) createListener() (err error) {
	if s.listener == nil {
		s.listener, err = net.Listen("tcp", s.addr)
		if err == nil {
			s.addr = s.listener.Addr().String()
		}
	}
	return
}
func (s *HTTPServer) Listen() (err error) {
	err = s.createListener()
	if err != nil {
		return
	}
	go func() {
		for {
			err := s.server.Serve(s.listener)
			if err != nil {
				if !s.isTestNotClosedError && strings.Contains(err.Error(), "closed") {
					if s.isShutdown {
						s.logger.Infof("http server graceful shutdown on http://%s", s.addr)
					} else {
						s.logger.Infof("http server closed on http://%s", s.addr)
						s.server.Close()
					}
					break
				} else {
					s.logger.Warnf("http server Serve fail on http://%s , error : %s", s.addr, err)
					time.Sleep(time.Second * 3)
					continue
				}
			}
		}
	}()
	s.logger.Infof("http server listen on http://%s", s.listener.Addr())
	return
}
func (s *HTTPServer) ListenTLS() (err error) {
	err = s.createListener()
	if err != nil {
		return
	}
	go func() {
		for {
			err := s.server.ServeTLS(s.listener, s.config.GetString("httpserver.tlscert"),
				s.config.GetString("httpserver.tlskey"))
			if err != nil {
				if !s.isTestNotClosedError && strings.Contains(err.Error(), "closed") {
					if s.isShutdown {
						s.logger.Infof("https server graceful shutdown on https://%s", s.addr)
					} else {
						s.logger.Infof("https server closed on https://%s", s.addr)
						s.server.Close()
					}
					break
				} else {
					s.logger.Warnf("http server ServeTLS fail , error : %s", err)
					time.Sleep(time.Second * 3)
					continue
				}
			}
		}
	}()
	s.logger.Infof("https server listen on https://%s", s.listener.Addr())
	return
}

//ConnState count the active conntions
func (s *HTTPServer) connState(c net.Conn, st http.ConnState) {
	switch st {
	case http.StateNew:
		s.localaddr.Store(c.RemoteAddr().String(), c.LocalAddr().String())
		atomic.AddInt64(s.connCnt, 1)
	case http.StateClosed:
		s.localaddr.Delete(c.RemoteAddr().String())
		atomic.AddInt64(s.connCnt, -1)
	}
}

// must be called after router inited
func (s *HTTPServer) initStatic() {
	s.staticDir = s.config.GetString("static.dir")
	s.staticUrlpath = s.config.GetString("static.urlpath")
	if s.staticDir != "" && s.staticUrlpath != "" {
		if strings.HasSuffix(s.staticUrlpath, "/") {
			s.staticUrlpath = strings.TrimRight(s.staticUrlpath, "/")
		}
		s.router.HandlerFunc("GET", s.staticUrlpath+"/*filepath", s.serveStatic)
		s.staticUrlpath += "/"
	}
}
func (s *HTTPServer) initTLSConfig() (err error) {
	if s.config.GetBool("httpserver.tlsenable") {
		tlsCfg := &tls.Config{}
		if s.config.GetBool("httpserver.tlsclientauth") {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		}
		clientCertPool := x509.NewCertPool()
		caBytes, e := ioutil.ReadFile(s.config.GetString("httpserver.tlsclientsca"))
		if e != nil {
			return e
		}
		ok := clientCertPool.AppendCertsFromPEM(caBytes)
		if !ok {
			err = gerr.New("failed to parse tls clients root certificate")
			return
		}
		tlsCfg.ClientCAs = clientCertPool
		s.server.TLSConfig = tlsCfg
	}
	return
}
func (s *HTTPServer) initSessionStore() (err error) {
	if !s.config.GetBool("session.enable") {
		return
	}
	typ := s.config.GetString("session.store")
	if typ == "" {
		typ = "memory"
	}
	ttl := s.config.GetInt64("session.ttl")
	switch typ {
	case "file":
		cfg := gfilestore.NewConfig()
		cfg.TTL = ttl
		cfg.Dir = s.config.GetString("session.file.dir")
		cfg.GCtime = s.config.GetInt("session.file.gctime")
		cfg.Prefix = s.config.GetString("session.file.prefix")
		s.sessionStore, err = gfilestore.New(cfg)
	case "memory":
		cfg := gmemorystore.NewConfig()
		cfg.TTL = ttl
		cfg.GCtime = s.config.GetInt("session.memory.gctime")
		s.sessionStore, err = gmemorystore.New(cfg)
	case "redis":
		cfg := gredisstore.NewRedisStoreConfig()
		cfg.RedisCfg.Addr = s.config.GetString("session.redis.address")
		cfg.RedisCfg.Password = s.config.GetString("session.redis.password")
		cfg.RedisCfg.Prefix = s.config.GetString("session.redis.prefix")
		cfg.RedisCfg.Debug = s.config.GetBool("session.redis.debug")
		cfg.RedisCfg.Timeout = time.Second * s.config.GetDuration("session.redis.timeout")
		cfg.RedisCfg.DBNum = s.config.GetInt("session.redis.dbnum")
		cfg.RedisCfg.MaxIdle = s.config.GetInt("session.redis.maxidle")
		cfg.RedisCfg.MaxActive = s.config.GetInt("session.redis.maxactive")
		cfg.RedisCfg.MaxConnLifetime = time.Second * s.config.GetDuration("session.redis.maxconnlifetime")
		cfg.RedisCfg.Wait = s.config.GetBool("session.redis.wait")
		cfg.TTL = ttl
		s.sessionStore, err = gredisstore.New(cfg)
	default:
		err = fmt.Errorf("unknown session store type %s", typ)
	}
	return
}

func (s *HTTPServer) serveStatic(w http.ResponseWriter, r *http.Request) {
	pathA := strings.Split(r.URL.Path, "?")
	path := filepath.Clean(pathA[0])
	path = strings.TrimPrefix(path, s.staticUrlpath)
	var b []byte
	var ok bool
	//1. find in bindata
	if len(bindata) > 0 {
		b, ok = bindata[path]
	}
	//2. find in system path
	if !ok {
		var e error
		b, e = ioutil.ReadFile(filepath.Join(s.staticDir, path))
		ok = e == nil
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
		return
	}
	cacheSince := time.Now().Format(http.TimeFormat)
	cacheUntil := time.Now().AddDate(66, 0, 0).Format(http.TimeFormat)
	ext := filepath.Ext(path)
	typ := mime.TypeByExtension(ext)
	w.Header().Set("Cache-Control", "max-age:290304000, public")
	w.Header().Set("Last-Modified", cacheSince)
	w.Header().Set("Expires", cacheUntil)
	w.Header().Set("Content-Type", typ)
	gizpCheck := map[string]bool{".js": true, ".css": true}
	if _, ok := gizpCheck[ext]; ok {
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			gz.Write(b)
			return
		}
	}
	w.Write(b)
}

// PrintRouteTable dump all routes into `w`, if `w` is nil, os.Stdout will be used.
func (s *HTTPServer) PrintRouteTable(w io.Writer) {
	s.router.PrintRouteTable(w)
}

//Start implements service.Service Start
func (s *HTTPServer) Start() (err error) {
	defer func() {
		if err == nil && s.config.GetBool("httpserver.printroute") {
			s.PrintRouteTable(s.logger.Writer())
		}
	}()
	// delay template Parse
	err = s.tpl.Parse()
	if err != nil {
		return
	}
	if s.config.GetBool("httpserver.tlsenable") {
		return s.ListenTLS()
	}
	return s.Listen()
}

//Stop implements service.Service Stop
func (s *HTTPServer) Stop() {
	s.Close()
	return
}

//GracefulStop implements service.Service GracefulStop
func (s *HTTPServer) GracefulStop() {
	if s.isShutdown {
		return
	}
	s.isShutdown = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s.server.Shutdown(ctx)
	return
}

//SetLog implements service.Service SetLog
func (s *HTTPServer) SetLog(l gcore.Logger) {
	s.logger = l
	return
}
func (s *HTTPServer) callMiddleware(ctx gcore.Ctx, middleware []func(ctx gcore.Ctx, server gcore.HTTPServer) (isStop bool)) (isStop bool) {
	for _, fn := range middleware {
		func() {
			defer gcore.Recover(func(e interface{}) {
				s.logger.Warnf("middleware panic error : %s", gerr.Stack(e))
				isStop = false
			})
			isStop = fn(ctx, s)
		}()
		if isStop {
			return
		}
	}
	return
}
