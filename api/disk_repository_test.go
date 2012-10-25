//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//
package api

import (
	"github.com/jbrukh/ggit/util"
	"testing"
)

func Test_Open(t *testing.T) {
	util.Assert(t, Open("test").path == "test/.git")
	util.Assert(t, Open("test/.git").path == "test/.git")
}