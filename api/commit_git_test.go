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
	"strings"
	"testing"
)

func Test_commitReads(t *testing.T) {
	repo := util.TempRepo("test_commits")

	util.AssertCreateGitRepo(t, repo)
	defer util.AssertRemoveGitRepo(t, repo)

	testFile := "a.txt"
	util.TestFile(repo, testFile, "a")

	// create a few commits
	err := util.GitExecMany(repo,
		[]string{"add", "--all"},
		[]string{"commit", "-a", "-m", "First commit."},
	)
	util.AssertNoErrOrDie(t, err)

	var head, dashP string
	head, err = util.GitExec(repo, "rev-parse", "HEAD")
	util.AssertNoErr(t, err)
	dashP, err = util.GitExec(repo, "cat-file", "-p", "HEAD")
	util.AssertNoErr(t, err)

	// create a ggit repo
	ggrepo := Open(repo)

	var o Object
	head = strings.TrimSpace(head)
	o, err = ggrepo.ObjectFromShortOid(head)
	util.AssertNoErr(t, err)
	util.Assert(t, o.Header().Type() == ObjectCommit)

	c := o.(*Commit)
	f := NewStrFormat()
	f.Commit(c)
	util.AssertEqualString(t, f.String(), dashP)
}