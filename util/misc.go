// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package util

import "fmt"

// MapSS2SI converts map[string]string to map[string]interface{}
func MapSS2SI(a map[string]string) (b map[string]interface{}) {
	b = map[string]interface{}{}
	for k, v := range a {
		b[k] = v
	}
	return
}

// MapSI2SS converts map[string]interface{} to map[string]string
func MapSI2SS(a map[string]interface{}) (b map[string]string) {
	b = map[string]string{}
	for k, v := range a {
		switch vv := v.(type) {
		case string:
			b[k] = vv
		default:
			b[k] = fmt.Sprintf("%v", vv)
		}
	}
	return
}
