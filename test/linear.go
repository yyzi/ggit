//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//

/*
linear.go implements a repo test case.
*/
package test

import (
	"errors"
	"fmt"
	"github.com/jbrukh/ggit/util"
	"strings"
)

// ================================================================= //
// TEST CASE: A FEW LINEAR COMMITS
// ================================================================= //

type CommitAndParent struct {
	Oid       string
	ParentOid string
}

type OutputCommits struct {
	Commits []*CommitAndParent
	N       int
}

var Linear = NewRepoTestCase(
	"__linear",
	func(testCase *RepoTestCase) (err error) {
		n := paramLinearN
		err = createRepo(testCase)
		if err != nil {
			return err
		}
		if n < 1 {
			return errors.New("n must be > 0")
		}
		repo := testCase.repo
		output := &OutputCommits{
			Commits: make([]*CommitAndParent, n),
			N:       n,
		}
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("%d.txt", i)
			err = util.TestFile(repo, name, string(i))
			if err != nil {
				return errors.New("could not create test file for repo: " + err.Error())
			}
			// create a few commits
			err = util.GitExecMany(repo,
				[]string{"add", "--all"},
				[]string{"commit", "-a", "-m", fmt.Sprintf("\"Commit: %d\"", i)},
			)
			if err != nil {
				return fmt.Errorf("could not commit to repo: %s", err)
			}

			// get the output data
			var oid, parentOid string
			oid = RevOid(repo, "HEAD")
			if i != 0 {
				parentOid = RevOid(repo, "HEAD~1")
			}
			output.Commits[i] = &CommitAndParent{
				oid,
				parentOid,
			}
		}
		testCase.output = output
		return
	},
)

func RevOid(repo string, rev string) string {
	oid, err := util.GitExec(repo, "rev-parse", rev)
	if err != nil {
		panic("can't get oid for: " + rev)
	}
	return strings.TrimSpace(oid)
}
