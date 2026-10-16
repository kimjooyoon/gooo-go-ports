// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2026 Gooo Go Ports contributors. All rights reserved.
// Use of this source code is governed by the BSD-style license in
// third_party/GO-LICENSE; the full original notice is retained in that file.
//
// Verification-only originals from Go1.27.2 revision
// 022c8636110ebac86a9b88724cda168ed713c21f, src/net/http/fs.go:435..467,
// SHA25621660cb61d17c258de99172683f9b41aca37d45c0af0556723f2a11179bdc45e.
// scanETag/etagStrongMatch bodies and comments below remain byte-exact originals.
// textproto.TrimString is the actual pinned stdlib implementation from
// src/net/textproto/textproto.go SHA256e4f0d94752345b5dec4fe7f4805531e8036a9cd44131d807cafa700dd5323901.
// Facts are original composition over these helpers and the actual imported
// x/net v0.59.0 httpguts.ValidHeaderFieldValue, revision540d04cfe5028e2655754591a4d3e08c586809f2,
// http/httpguts/httplex.go SHA2568cfd5819dc821d45f82fde10afff4b2e652535b0091e0559fc8ad7766051a2c2.
// Tool/module/source identities remain owned by the existing verifier pins.
package main

import (
	"net/textproto"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// scanETag determines if a syntactically valid ETag is present at s. If so,
// the ETag and remaining text after consuming ETag is returned. Otherwise,
// it returns "", "".
func scanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	// ETag is either W/"text" or "text".
	// See RFC 7232 2.3.
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		// Character values allowed in ETags.
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

// etagStrongMatch reports whether a and b match using strong ETag comparison.
// Assumes a and b are valid ETags.
func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

// Only this verifier computes expected results. Neither Gooo bodies, saved
// observations nor the literal anchor fixture supplies native expected values.
func etagScanReference(text string) map[string]any {
	tag, remain := scanETag(text)
	return map[string]any{"etag": tag, "remain": remain}
}

// This original thirteen-field composition is not full If-Match or authority
// to send/write. Validate the original raw header; compare only the parsed tag.
func etagFactsReference(raw string) map[string]any {
	tag, remain := scanETag(raw)
	headerValid := httpguts.ValidHeaderFieldValue(raw)
	present := tag != ""
	weak := present && strings.HasPrefix(tag, "W/")
	complete := present && remain == ""
	strong := etagStrongMatch(tag, tag)
	reason := "SINGLE_STRONG_TAG"
	switch {
	case !headerValid:
		reason = "HEADER_VALUE_INVALID"
	case !present:
		reason = "NO_VALID_TAG"
	case !complete:
		reason = "TRAILING_CONTENT"
	case weak:
		reason = "WEAK_TAG"
	}
	return map[string]any{
		"raw": raw, "tag": tag, "remainder": remain,
		"header_valid": headerValid, "present": present, "weak": weak,
		"complete": complete, "strong": strong,
		"single_strong": headerValid && present && complete && strong,
		"input_bytes":   int64(len(raw)), "tag_bytes": int64(len(tag)),
		"remainder_bytes": int64(len(remain)), "reason": reason,
	}
}

func etagReference(row etagInputRow) any {
	switch row.Kind {
	case "scanner":
		return etagScanReference(row.Inputs[0])
	case "strong_match":
		return etagStrongMatch(row.Inputs[0], row.Inputs[1])
	case "facts":
		return etagFactsReference(row.Inputs[0])
	default:
		panic("ETag oracle called without a validated input kind")
	}
}
