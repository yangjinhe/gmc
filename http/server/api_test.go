// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package ghttpserver

import (
	gcore "github.com/snail007/gmc/core"
	"testing"

	ghttputil "github.com/snail007/gmc/internal/util/http"

	"github.com/stretchr/testify/assert"
)

func TestNewAPIServer(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	assert.NotNil(api.server)
	assert.Equal(len(api.address), 1)
}
func TestBefore(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.AddMiddleware0(func(c gcore.Ctx) bool {
		c.Write("okay")
		return true
	})
	api.API("/hello", func(c gcore.Ctx) {
		c.Write("a")
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("okay", data)
}

func TestAPI(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.API("/hello", func(c gcore.Ctx) {
		c.Write("a")
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("a", data)
}

func TestAfter(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.AddMiddleware2(func(c gcore.Ctx) bool {
		c.Write("okay")
		return false
	})
	api.API("/hello", func(c gcore.Ctx) {
		c.Write("a")
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("aokay", data)
}
func TestStop(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.API("/hello", func(c gcore.Ctx) {
		c.Write("a")
		ghttputil.Stop(c.Response(), "c")
		c.Write("b")
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("ac", data)
}
func TestHandle404(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.SetNotFoundHandler(func(c gcore.Ctx) {
		c.Write("404")
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("404", data)
}
func TestHandle404_1(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("Page not found", data)
}
func TestHandle500(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.SetErrorHandler(func(c gcore.Ctx, err interface{}) {
		c.Write("500")
	})
	api.API("/hello", func(c gcore.Ctx) {
		a := 0
		a /= a
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("500", data)
}
func TestHandle500_1(t *testing.T) {
	assert := assert.New(t)
	api := NewAPIServer(gcore.Providers.Ctx("")(), ":")
	api.ShowErrorStack(false)
	api.API("/hello", func(c gcore.Ctx) {
		a := 0
		a /= a
	})
	w, r := mockRequest("/hello")
	api.ServeHTTP(w, r)
	data, _, err := mockResponse(w)
	assert.Nil(err)
	assert.Equal("Internal Server Error", data)
}
