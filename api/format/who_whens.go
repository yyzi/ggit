//
// Unless otherwise noted, this project is licensed under the Creative
// Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License. Please
// see the README file.
//
// Copyright (c) 2012 The ggit Authors
package format

import (
	"fmt"
	"github.com/jbrukh/ggit/api/objects"
	"github.com/jbrukh/ggit/api/token"
)

// ================================================================= //
// FORMATTING
// ================================================================= //

func (f *formatter) WhoWhenDate(ww *objects.WhoWhen) (int, error) {
	return fmt.Fprintf(f.Writer, "%s <%s> %s %s", ww.Name(), ww.Email(), ww.Date(), zone(ww.Offset()))
}

func (f *formatter) WhoWhen(ww *objects.WhoWhen) (int, error) {
	return fmt.Fprintf(f.Writer, "%s <%s> %d %s", ww.Name(), ww.Email(), ww.Seconds(), zone(ww.Offset()))
}

// ================================================================= //
// UTIL
// ================================================================= //

func zone(offset int) string {
	sign := ""
	if offset < 0 {
		sign = token.MINUS
		offset = -offset
	} else {
		sign = token.PLUS
	}
	hours := int(offset / 60)
	minutes := offset - hours*60
	return fmt.Sprintf("%s%02d%02d", sign, hours, minutes)
}
