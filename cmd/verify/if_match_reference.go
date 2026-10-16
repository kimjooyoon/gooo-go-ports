// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by the BSD-style license reproduced in
// third_party/GO-LICENSE. Verifier glue is also covered by LICENSE.
// Original Go1.27.2 revision022c8636110ebac86a9b88724cda168ed713c21f,
// src/net/http/fs.go lines475..513, SHA256
// 21660cb61d17c258de99172683f9b41aca37d45c0af0556723f2a11179bdc45e.
// Exactly three substitutions: the function signature accepts two selected
// strings; Header.Get("If-Match") becomes selectedIfMatchHeader; private
// w.Header().get("Etag") becomes currentETag. No Go loop/enum is changed.
// scanETag and etagStrongMatch are the unchanged originals in etag_reference.go;
// textproto.TrimString is the actual pinned Go standard-library function.
package main

import "net/textproto"

// condResult is the result of an HTTP request precondition check.
// See https://tools.ietf.org/html/rfc7232 section 3.
type condResult int

const (
	condNone condResult = iota
	condTrue
	condFalse
)

func checkIfMatch(selectedIfMatchHeader, currentETag string) condResult {
	im := selectedIfMatchHeader
	if im == "" {
		return condNone
	}
	for {
		im = textproto.TrimString(im)
		if len(im) == 0 {
			break
		}
		if im[0] == ',' {
			im = im[1:]
			continue
		}
		if im[0] == '*' {
			return condTrue
		}
		etag, remain := scanETag(im)
		if etag == "" {
			break
		}
		if etagStrongMatch(etag, currentETag) {
			return condTrue
		}
		im = remain
	}

	return condFalse
}

// Only the independent Go verifier supplies finite expectations. Native actuals,
// saved receipts, or the Gooo source never substitute for this value.
func ifMatchReference(selected, current string) map[string]any {
	return map[string]any{"condition": int64(checkIfMatch(selected, current))}
}
