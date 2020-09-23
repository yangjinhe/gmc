package httpserver

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	gmcconfig "github.com/snail007/gmc/config/gmc"
	"github.com/snail007/gmc/http/controller"
	"github.com/snail007/gmc/http/router"
	"github.com/snail007/gmc/http/server/ctxvalue"
	"github.com/snail007/gmc/http/session"
	"github.com/snail007/gmc/http/session/filestore"
	"github.com/snail007/gmc/http/session/memorystore"
	"github.com/snail007/gmc/http/session/redisstore"
	"github.com/snail007/gmc/http/template"
	"github.com/snail007/gmc/util/logutil"
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
	tpl          *template.Template
	sessionStore session.Store
	router       *router.HTTPRouter
	logger       *log.Logger
	addr         string
	listener     net.Listener
	server       *http.Server
	connCnt      *int64
	config       *gmcconfig.GMCConfig
	handler40x   func(w http.ResponseWriter, r *http.Request, tpl *template.Template)
	handler50x   func(c *controller.Controller, err interface{})
	//just for testing
	isTestNotClosedError bool
	staticDir            string
	staticUrlpath        string
	beforeRouting        func(w http.ResponseWriter, r *http.Request, server *HTTPServer) (isContinue bool)
	routingFiliter       func(w http.ResponseWriter, r *http.Request, ps router.Params, server *HTTPServer) (isContinue bool)
}

func New() *HTTPServer {
	return &HTTPServer{}
}

//Init implements service.Services Init
func (s *HTTPServer) Init(cfg *gmcconfig.GMCConfig) (err error) {
	connCnt := int64(0)
	s.server = &http.Server{}
	s.logger = logutil.New("")
	s.connCnt = &connCnt
	s.config = cfg
	s.isTestNotClosedError = false
	s.server.ConnState = s.connState
	s.server.Handler = s

	//init base objects
	err = s.initBaseObjets()
	return
}
func (s *HTTPServer) initBaseObjets() (err error) {
	s.tpl, err = template.New(s.config.GetString("template.dir"))
	if err != nil {
		return
	}
	//init session store
	err = s.initSessionStore()
	if err != nil {
		return
	}
	//init http server tls configuration
	err = s.initTLSConfig()
	if err != nil {
		return
	}
	//init http server router
	s.router = router.NewHTTPRouter()
	s.addr = s.config.GetString("httpserver.listen")

	//init static files handler, must be after router inited
	s.initStatic()
	return
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = context.WithValue(ctx, ctxvalue.CtxValueKey, ctxvalue.CtxValue{
		Tpl:          s.tpl,
		SessionStore: s.sessionStore,
		Router:       s.router,
		Config:       s.config,
	})
	r = r.WithContext(ctx)
	//before routing
	if s.beforeRouting != nil && !s.beforeRouting(w, r, s) {
		return
	}
	h, args, _ := s.router.Lookup(r.Method, r.URL.Path)
	if h != nil {
		// routing filiter
		if s.routingFiliter != nil && !s.routingFiliter(w, r, args, s) {
			return
		}
		h(w, r, args)
	} else {
		//404
		s.handle40x(w, r, args)
	}
}
func (s *HTTPServer) SetHandler40x(fn func(w http.ResponseWriter, r *http.Request, tpl *template.Template)) *HTTPServer {
	s.handler40x = fn
	return s
}
func (s *HTTPServer) SetHandler50x(fn func(c *controller.Controller, err interface{})) *HTTPServer {
	s.handler50x = fn
	return s
}

func (s *HTTPServer) handle40x(w http.ResponseWriter, r *http.Request, ps router.Params) {
	if s.handler40x == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Page not found"))
	} else {
		s.handler40x(w, r, s.tpl)
	}
	return
}

func (s *HTTPServer) handle50x(objv *reflect.Value, err interface{}) {
	if s.handler50x == nil {
		c := objv.Interface().(*controller.Controller)
		c.Response.WriteHeader(http.StatusInternalServerError)
		c.Write("Internal Server Error")
	} else {
		c := objv.Interface().(*controller.Controller)
		s.handler50x(c, err)
	}
}

func (s *HTTPServer) SetConfig(c *gmcconfig.GMCConfig) *HTTPServer {
	s.config = c
	return s
}
func (s *HTTPServer) Config() *gmcconfig.GMCConfig {
	return s.config
}

func (s *HTTPServer) ActiveConnCount() int64 {
	return atomic.LoadInt64(s.connCnt)
}
func (s *HTTPServer) Close() *HTTPServer {
	s.server.Close()
	return s
}
func (s *HTTPServer) Listener() net.Listener {
	return s.listener
}
func (s *HTTPServer) Server() *http.Server {
	return s.server
}
func (s *HTTPServer) SetLogger(l *log.Logger) *HTTPServer {
	s.logger = l
	s.server.ErrorLog = s.logger
	return s
}
func (s *HTTPServer) Logger() *log.Logger {
	return s.logger
}
func (s *HTTPServer) SetRouter(r *router.HTTPRouter) *HTTPServer {
	s.router = r
	s.router.SetHandle50x(s.handle50x)
	return s
}
func (s *HTTPServer) Router() *router.HTTPRouter {
	return s.router
}
func (s *HTTPServer) SetTpl(t *template.Template) *HTTPServer {
	s.tpl = t
	return s
}
func (s *HTTPServer) Tpl() *template.Template {
	return s.tpl
}
func (s *HTTPServer) SetSessionStore(st session.Store) *HTTPServer {
	s.sessionStore = st
	return s
}
func (s *HTTPServer) SessionStore() session.Store {
	return s.sessionStore
}
func (s *HTTPServer) BeforeRouting(fn func(w http.ResponseWriter, r *http.Request, server *HTTPServer) (isContinue bool)) *HTTPServer {
	s.beforeRouting = fn
	return s
}
func (s *HTTPServer) RoutingFiliter(fn func(w http.ResponseWriter, r *http.Request, ps router.Params, server *HTTPServer) (isContinue bool)) *HTTPServer {
	s.routingFiliter = fn
	return s
}

//just for testing
func (s *HTTPServer) bind(addr string) *HTTPServer {
	s.addr = addr
	return s
}
func (s *HTTPServer) createListener() (err error) {
	s.listener, err = net.Listen("tcp", s.addr)
	if err == nil {
		s.addr = s.listener.Addr().String()
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
					s.logger.Printf("http server closed on %s", s.addr)
					s.server.Close()
					break
				} else {
					s.logger.Printf("http server Serve fail on %s , error : %s", s.addr, err)
					time.Sleep(time.Second * 3)
					continue
				}
			}
		}
	}()
	s.logger.Printf("http server listen on >>> %s", s.listener.Addr())
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
					s.logger.Printf("https server closed.")
					s.server.Close()
					break
				} else {
					s.logger.Printf("http server ServeTLS fail , error : %s", err)
					time.Sleep(time.Second * 3)
					continue
				}
			}
		}
	}()
	s.logger.Printf("https server listen on >>> %s", s.listener.Addr())
	return
}

//ConnState count the active conntions
func (s *HTTPServer) connState(c net.Conn, st http.ConnState) {
	switch st {
	case http.StateNew:
		atomic.AddInt64(s.connCnt, 1)
	case http.StateClosed:
		atomic.AddInt64(s.connCnt, -1)
	}
}

// must be called after router inited
func (s *HTTPServer) initStatic() {
	s.staticDir = s.config.GetString("static.dir")
	s.staticUrlpath = s.config.GetString("static.urlpath")
	if s.staticDir != "" && s.staticUrlpath != "" {
		if !strings.HasSuffix(s.staticUrlpath, "/") {
			s.staticUrlpath += "/"
		}
		s.router.HandlerFunc("GET", s.staticUrlpath, s.serveStatic)
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
			err = errors.New("failed to parse tls clients root certificate")
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
		cfg := filestore.NewConfig()
		cfg.TTL = ttl
		cfg.Dir = s.config.GetString("session.file.dir")
		cfg.GCtime = s.config.GetInt("session.file.gctime")
		cfg.Prefix = s.config.GetString("session.file.prefix")
		s.sessionStore, err = filestore.New(cfg)
	case "memory":
		cfg := memorystore.NewConfig()
		cfg.TTL = ttl
		cfg.GCtime = s.config.GetInt("session.memory.gctime")
		s.sessionStore, err = memorystore.New(cfg)
	case "redis":
		cfg := redisstore.NewRedisStoreConfig()
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
		s.sessionStore, err = redisstore.New(cfg)
	default:
		err = fmt.Errorf("unknown session store type %s", typ)
	}
	return
}

func (s *HTTPServer) serveStatic(w http.ResponseWriter, r *http.Request) {
	pathA := strings.Split(r.URL.Path, "?")
	path := router.CleanPath(pathA[0])
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

//Start implements service.Services Start
func (s *HTTPServer) Start() (err error) {
	if s.config.GetBool("httpserver.tlsenable") {
		return s.ListenTLS()
	}
	return s.Listen()
}

//Stop implements service.Services Stop
func (s *HTTPServer) Stop() {
	s.Close()
	return
}

//GracefulStop implements service.Services GracefulStop
func (s *HTTPServer) GracefulStop() {
	s.Close()
	return
}

//SetLog implements service.Services SetLog
func (s *HTTPServer) SetLog(l *log.Logger) {
	s.logger = l
	return
}