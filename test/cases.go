//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//

/*
cases.go instantiates all the repo test cases.
*/
package test

import (
	"fmt"
	"github.com/jbrukh/ggit/util"
	"os"
	"time"
)

// ================================================================= //
// ALL TEST CASES
// ================================================================= //

// mapping of name => RepoTestCase
var repoTestCases = []*RepoTestCase{
	Empty,
	Blobs, // do not try to pack this, it won't work on just loose blobs
	Linear,
	LinearPacked,
	Derefs,
	DerefsPacked,
	Refs,
	Tree,
	TreeDiff,
}

// init initializes all the repo test cases, if they haven't been
// initialized already. An unsuccessful initialization will cause
// the entire process to exit.
func init() {
	fmt.Println("Creating repo test cases...\n")
	for _, testCase := range repoTestCases {
		start := time.Now()
		err := testCase.Build()
		if err != nil {
			fmt.Printf("error (exiting!): %s\n", err)
			RemoveTestCases()
			os.Exit(1)
		}
		fmt.Printf("Created case: %s (%d ms)\n\n", testCase.Name(), int64(time.Since(start))/int64(time.Millisecond))
	}
	fmt.Println("Done.\n")
}

func RemoveTestCases() {
	fmt.Println("Cleaning.")
	for _, testCase := range repoTestCases {
		fmt.Println(testCase.Name(), "\t", testCase.Repo())
		testCase.Remove()
	}
}

// ================================================================= //
// REPO TEST CASE
// ================================================================= //

type RepoTestCase struct {
	name    string
	repo    string // path
	builder RepoBuilder
	info    interface{}
}

func (tc *RepoTestCase) Repo() string {
	return tc.repo
}

func (tc *RepoTestCase) Name() string {
	return tc.name
}

func (tc *RepoTestCase) Info() interface{} {
	return tc.info
}

func (tc *RepoTestCase) Remove() {
	err := os.RemoveAll(tc.repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: error removing repo: %s\n", err)
	}
}

func (tc *RepoTestCase) Build() error {
	return tc.builder(tc)
}

func NewRepoTestCase(name string, builder RepoBuilder) *RepoTestCase {
	return &RepoTestCase{
		name:    name,
		builder: builder,
	}
}

// ================================================================= //
// REPO BUILDER
// ================================================================= //

type RepoBuilder func(testCase *RepoTestCase) error

// ================================================================= //
// UTIL
// ================================================================= //

func createRepo(testCase *RepoTestCase) (repo string, err error) {
	repo = util.TempRepo(testCase.name)

	// clean that shit
	os.RemoveAll(repo)
	_, err = util.CreateGitRepo(repo)
	if err != nil {
		return repo, fmt.Errorf("Could not create case '%s': %s", testCase.name, err.Error())
	}
	testCase.repo = repo
	return
}
