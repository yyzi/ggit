//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//
package api

import (
	"crypto/sha1"
	"fmt"
	"github.com/jbrukh/ggit/api/format"
	"github.com/jbrukh/ggit/api/objects"
	"github.com/jbrukh/ggit/api/token"
	"hash"
)

// the hash object used to build
// hashes of our objects
var sha hash.Hash = sha1.New()

// produce the SHA1 hash for any Object.
func MakeHash(o objects.Object) (hash.Hash, error) {
	sha.Reset()
	kind := string(o.Header().Type())
	f := format.NewStrFormat()
	if _, err := f.Object(o); err != nil {
		return nil, err
	}
	content := f.String()
	len := len([]byte(content))
	value := kind + string(token.SP) + fmt.Sprint(len) + string(token.NUL) + content
	toHash := []byte(value)
	sha.Write(toHash)
	return sha, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
