//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
//

/*
builtin.go implements the general framework for ggit builtin commands.

Code in this package originally based on https://github.com/jordanorelli/multicommand.
*/
package builtin

import (
	"fmt"
	"github.com/jbrukh/ggit/api"
	"io"
	"sort"
	"strings"
)

// ================================================================= //
// GGIT BUILTIN FRAMEWORK
// ================================================================= //

// the set of supported builtin commands for ggit
var builtins = make(map[string]Builtin)

type builtinByName []Builtin

func (s builtinByName) Len() int           { return len(s) }
func (s builtinByName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s builtinByName) Less(i, j int) bool { return s[i].Info().Name < s[j].Info().Name }

// add a builtin command
func Add(b Builtin) {
	builtins[b.Info().Name] = b
}

func Get(name string) (Builtin, bool) {
	b, ok := builtins[name]
	return b, ok
}

func All() []Builtin {
	b := make([]Builtin, 0, len(builtins))
	for _, v := range builtins {
		b = append(b, v)
	}
	sort.Sort(builtinByName(b))
	return b
}

// ================================================================= //
// BUILTIN
// ================================================================= //

type Params struct {
	Repo api.Repository
	Wout io.Writer
	Werr io.Writer
}

type Builtin interface {
	Info() *HelpInfo
	Execute(p *Params, args []string)
}

// Builtin describes a built-in command
type HelpInfo struct {
	// Name is the name of the command, a string with no spaces, 
	// usually consistng of lowercase letters.
	Name string

	// one line description of the command
	Description string

	// UsageLine is the one-line usage message.
	UsageLine string

	// ManPage display's this command's man page.
	ManPage string
}

func (info *HelpInfo) WriteUsage(w io.Writer) {
	// TODO: review
	fmt.Fprintf(w, "usage: %s %s\n\n", info.Name, info.UsageLine)
	fmt.Fprintf(w, "%s\n", strings.TrimSpace(info.ManPage))
}

// passing this method onto the individual builtins.
func (info *HelpInfo) Info() *HelpInfo {
	return info
}
