//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//
package parse

import (
	"github.com/jbrukh/ggit/api/objects"
	"github.com/jbrukh/ggit/util"
	"testing"
)

func Test_ParseObjectId(t *testing.T) {
	var oid *objects.ObjectId
	oidStr := "ff6ccb68859fd52216ec8dadf98d2a00859f5369"
	t1 := ObjectParserForString(oidStr)
	oid = t1.ParseOid()
	util.Assert(t, oid.String() == oidStr)
}
