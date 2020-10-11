package gmccompress_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	gmcerr "github.com/snail007/gmc/error"
	gmccompress "github.com/snail007/gmc/util/compress"
	assert "github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var (
	pkg = "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"
)

func TestUnTarGz(t *testing.T) {
	assert := assert.New(t)
	d, e := base64.StdEncoding.DecodeString(pkg)
	assert.Nil(e)
	var b bytes.Buffer
	b.Write(d)
	s, e := gmccompress.Unpack(&b, "test")
	fmt.Println(s, gmcerr.Stack(e))
	assert.Nil(e)
	os.RemoveAll("test")
}
