package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Exact original manual-contract.json bytes, frozen before the private oracle
// and native run. This literal makes future CI independent of private K paths.
const literalCutFrozenManualContract = `{
  "schema": "gooo/literal-cut-manual-contract/v1",
  "status": "FROZEN_BEFORE_ORACLE_OR_NATIVE",
  "cases": [
    {
      "name": "empty-both",
      "text": "",
      "needle": "",
      "expected": {
        "text": "",
        "needle": "",
        "cut_before": "",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "",
        "prefix_found": true,
        "suffix_before": "",
        "suffix_found": true,
        "text_byte_length": 0,
        "needle_byte_length": 0
      }
    },
    {
      "name": "empty-source",
      "text": "",
      "needle": "x",
      "expected": {
        "text": "",
        "needle": "x",
        "cut_before": "",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "",
        "prefix_found": false,
        "suffix_before": "",
        "suffix_found": false,
        "text_byte_length": 0,
        "needle_byte_length": 1
      }
    },
    {
      "name": "empty-needle",
      "text": "abc",
      "needle": "",
      "expected": {
        "text": "abc",
        "needle": "",
        "cut_before": "",
        "cut_after": "abc",
        "cut_found": true,
        "prefix_after": "abc",
        "prefix_found": true,
        "suffix_before": "abc",
        "suffix_found": true,
        "text_byte_length": 3,
        "needle_byte_length": 0
      }
    },
    {
      "name": "exact-source",
      "text": "abc",
      "needle": "abc",
      "expected": {
        "text": "abc",
        "needle": "abc",
        "cut_before": "",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "",
        "prefix_found": true,
        "suffix_before": "",
        "suffix_found": true,
        "text_byte_length": 3,
        "needle_byte_length": 3
      }
    },
    {
      "name": "absent",
      "text": "abc",
      "needle": "x",
      "expected": {
        "text": "abc",
        "needle": "x",
        "cut_before": "abc",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "abc",
        "prefix_found": false,
        "suffix_before": "abc",
        "suffix_found": false,
        "text_byte_length": 3,
        "needle_byte_length": 1
      }
    },
    {
      "name": "longer-needle",
      "text": "abc",
      "needle": "abcd",
      "expected": {
        "text": "abc",
        "needle": "abcd",
        "cut_before": "abc",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "abc",
        "prefix_found": false,
        "suffix_before": "abc",
        "suffix_found": false,
        "text_byte_length": 3,
        "needle_byte_length": 4
      }
    },
    {
      "name": "prefix-only",
      "text": "abc",
      "needle": "a",
      "expected": {
        "text": "abc",
        "needle": "a",
        "cut_before": "",
        "cut_after": "bc",
        "cut_found": true,
        "prefix_after": "bc",
        "prefix_found": true,
        "suffix_before": "abc",
        "suffix_found": false,
        "text_byte_length": 3,
        "needle_byte_length": 1
      }
    },
    {
      "name": "suffix-only",
      "text": "abc",
      "needle": "c",
      "expected": {
        "text": "abc",
        "needle": "c",
        "cut_before": "ab",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "abc",
        "prefix_found": false,
        "suffix_before": "ab",
        "suffix_found": true,
        "text_byte_length": 3,
        "needle_byte_length": 1
      }
    },
    {
      "name": "middle-only",
      "text": "abc",
      "needle": "b",
      "expected": {
        "text": "abc",
        "needle": "b",
        "cut_before": "a",
        "cut_after": "c",
        "cut_found": true,
        "prefix_after": "abc",
        "prefix_found": false,
        "suffix_before": "abc",
        "suffix_found": false,
        "text_byte_length": 3,
        "needle_byte_length": 1
      }
    },
    {
      "name": "duplicate-aa",
      "text": "aaaa",
      "needle": "aa",
      "expected": {
        "text": "aaaa",
        "needle": "aa",
        "cut_before": "",
        "cut_after": "aa",
        "cut_found": true,
        "prefix_after": "aa",
        "prefix_found": true,
        "suffix_before": "aa",
        "suffix_found": true,
        "text_byte_length": 4,
        "needle_byte_length": 2
      }
    },
    {
      "name": "overlap-aba",
      "text": "ababa",
      "needle": "aba",
      "expected": {
        "text": "ababa",
        "needle": "aba",
        "cut_before": "",
        "cut_after": "ba",
        "cut_found": true,
        "prefix_after": "ba",
        "prefix_found": true,
        "suffix_before": "ab",
        "suffix_found": true,
        "text_byte_length": 5,
        "needle_byte_length": 3
      }
    },
    {
      "name": "odd-aa",
      "text": "aaaaa",
      "needle": "aa",
      "expected": {
        "text": "aaaaa",
        "needle": "aa",
        "cut_before": "",
        "cut_after": "aaa",
        "cut_found": true,
        "prefix_after": "aaa",
        "prefix_found": true,
        "suffix_before": "aaa",
        "suffix_found": true,
        "text_byte_length": 5,
        "needle_byte_length": 2
      }
    },
    {
      "name": "repeated-multibyte-separator",
      "text": "one::two::three",
      "needle": "::",
      "expected": {
        "text": "one::two::three",
        "needle": "::",
        "cut_before": "one",
        "cut_after": "two::three",
        "cut_found": true,
        "prefix_after": "one::two::three",
        "prefix_found": false,
        "suffix_before": "one::two::three",
        "suffix_found": false,
        "text_byte_length": 15,
        "needle_byte_length": 2
      }
    },
    {
      "name": "repeated-equals",
      "text": "a=a=a",
      "needle": "=",
      "expected": {
        "text": "a=a=a",
        "needle": "=",
        "cut_before": "a",
        "cut_after": "a=a",
        "cut_found": true,
        "prefix_after": "a=a=a",
        "prefix_found": false,
        "suffix_before": "a=a=a",
        "suffix_found": false,
        "text_byte_length": 5,
        "needle_byte_length": 1
      }
    },
    {
      "name": "whitespace-preserved",
      "text": " a ",
      "needle": " ",
      "expected": {
        "text": " a ",
        "needle": " ",
        "cut_before": "",
        "cut_after": "a ",
        "cut_found": true,
        "prefix_after": "a ",
        "prefix_found": true,
        "suffix_before": " a",
        "suffix_found": true,
        "text_byte_length": 3,
        "needle_byte_length": 1
      }
    },
    {
      "name": "nul-repeated",
      "text": "A\u0000B\u0000",
      "needle": "\u0000",
      "expected": {
        "text": "A\u0000B\u0000",
        "needle": "\u0000",
        "cut_before": "A",
        "cut_after": "B\u0000",
        "cut_found": true,
        "prefix_after": "A\u0000B\u0000",
        "prefix_found": false,
        "suffix_before": "A\u0000B",
        "suffix_found": true,
        "text_byte_length": 4,
        "needle_byte_length": 1
      }
    },
    {
      "name": "crlf-repeated",
      "text": "a\r\nb\r\n",
      "needle": "\r\n",
      "expected": {
        "text": "a\r\nb\r\n",
        "needle": "\r\n",
        "cut_before": "a",
        "cut_after": "b\r\n",
        "cut_found": true,
        "prefix_after": "a\r\nb\r\n",
        "prefix_found": false,
        "suffix_before": "a\r\nb",
        "suffix_found": true,
        "text_byte_length": 6,
        "needle_byte_length": 2
      }
    },
    {
      "name": "tab-both-ends",
      "text": "\tfield\t",
      "needle": "\t",
      "expected": {
        "text": "\tfield\t",
        "needle": "\t",
        "cut_before": "",
        "cut_after": "field\t",
        "cut_found": true,
        "prefix_after": "field\t",
        "prefix_found": true,
        "suffix_before": "\tfield",
        "suffix_found": true,
        "text_byte_length": 7,
        "needle_byte_length": 1
      }
    },
    {
      "name": "cr-is-not-crlf",
      "text": "\r\n",
      "needle": "\r",
      "expected": {
        "text": "\r\n",
        "needle": "\r",
        "cut_before": "",
        "cut_after": "\n",
        "cut_found": true,
        "prefix_after": "\n",
        "prefix_found": true,
        "suffix_before": "\r\n",
        "suffix_found": false,
        "text_byte_length": 2,
        "needle_byte_length": 1
      }
    },
    {
      "name": "korean-both-ends",
      "text": "가x가",
      "needle": "가",
      "expected": {
        "text": "가x가",
        "needle": "가",
        "cut_before": "",
        "cut_after": "x가",
        "cut_found": true,
        "prefix_after": "x가",
        "prefix_found": true,
        "suffix_before": "가x",
        "suffix_found": true,
        "text_byte_length": 7,
        "needle_byte_length": 3
      }
    },
    {
      "name": "korean-byte-middle",
      "text": "가x가",
      "needle": "x",
      "expected": {
        "text": "가x가",
        "needle": "x",
        "cut_before": "가",
        "cut_after": "가",
        "cut_found": true,
        "prefix_after": "가x가",
        "prefix_found": false,
        "suffix_before": "가x가",
        "suffix_found": false,
        "text_byte_length": 7,
        "needle_byte_length": 1
      }
    },
    {
      "name": "emoji-prefix",
      "text": "🙂🙂x",
      "needle": "🙂",
      "expected": {
        "text": "🙂🙂x",
        "needle": "🙂",
        "cut_before": "",
        "cut_after": "🙂x",
        "cut_found": true,
        "prefix_after": "🙂x",
        "prefix_found": true,
        "suffix_before": "🙂🙂x",
        "suffix_found": false,
        "text_byte_length": 9,
        "needle_byte_length": 4
      }
    },
    {
      "name": "emoji-suffix",
      "text": "x🙂",
      "needle": "🙂",
      "expected": {
        "text": "x🙂",
        "needle": "🙂",
        "cut_before": "x",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "x🙂",
        "prefix_found": false,
        "suffix_before": "x",
        "suffix_found": true,
        "text_byte_length": 5,
        "needle_byte_length": 4
      }
    },
    {
      "name": "no-normalize-decomposed-source",
      "text": "é",
      "needle": "é",
      "expected": {
        "text": "é",
        "needle": "é",
        "cut_before": "é",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "é",
        "prefix_found": false,
        "suffix_before": "é",
        "suffix_found": false,
        "text_byte_length": 3,
        "needle_byte_length": 2
      }
    },
    {
      "name": "no-normalize-composed-source",
      "text": "é",
      "needle": "é",
      "expected": {
        "text": "é",
        "needle": "é",
        "cut_before": "é",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "é",
        "prefix_found": false,
        "suffix_before": "é",
        "suffix_found": false,
        "text_byte_length": 2,
        "needle_byte_length": 3
      }
    },
    {
      "name": "combining-sequence-repeated",
      "text": "éé",
      "needle": "é",
      "expected": {
        "text": "éé",
        "needle": "é",
        "cut_before": "",
        "cut_after": "é",
        "cut_found": true,
        "prefix_after": "é",
        "prefix_found": true,
        "suffix_before": "é",
        "suffix_found": true,
        "text_byte_length": 6,
        "needle_byte_length": 3
      }
    },
    {
      "name": "bom-not-trimmed",
      "text": "` + "\ufeff" + `abc",
      "needle": "` + "\ufeff" + `",
      "expected": {
        "text": "` + "\ufeff" + `abc",
        "needle": "` + "\ufeff" + `",
        "cut_before": "",
        "cut_after": "abc",
        "cut_found": true,
        "prefix_after": "abc",
        "prefix_found": true,
        "suffix_before": "` + "\ufeff" + `abc",
        "suffix_found": false,
        "text_byte_length": 6,
        "needle_byte_length": 3
      }
    },
    {
      "name": "last-byte-1024",
      "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaZ",
      "needle": "Z",
      "expected": {
        "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaZ",
        "needle": "Z",
        "cut_before": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaZ",
        "prefix_found": false,
        "suffix_before": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "suffix_found": true,
        "text_byte_length": 1024,
        "needle_byte_length": 1
      }
    },
    {
      "name": "whole-needle-1024",
      "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "needle": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "expected": {
        "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "needle": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "cut_before": "",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "",
        "prefix_found": true,
        "suffix_before": "",
        "suffix_found": true,
        "text_byte_length": 1024,
        "needle_byte_length": 1024
      }
    },
    {
      "name": "needle-1024-source-1023",
      "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "needle": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "expected": {
        "text": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "needle": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "cut_before": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "cut_after": "",
        "cut_found": false,
        "prefix_after": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "prefix_found": false,
        "suffix_before": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "suffix_found": false,
        "text_byte_length": 1023,
        "needle_byte_length": 1024
      }
    },
    {
      "name": "multibyte-last-byte-1024",
      "text": "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂ABCD",
      "needle": "D",
      "expected": {
        "text": "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂ABCD",
        "needle": "D",
        "cut_before": "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂ABC",
        "cut_after": "",
        "cut_found": true,
        "prefix_after": "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂ABCD",
        "prefix_found": false,
        "suffix_before": "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂ABC",
        "suffix_found": true,
        "text_byte_length": 1024,
        "needle_byte_length": 1
      }
    },
    {
      "name": "multibyte-whole-1024",
      "text": "éééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé",
      "needle": "é",
      "expected": {
        "text": "éééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé",
        "needle": "é",
        "cut_before": "",
        "cut_after": "ééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé",
        "cut_found": true,
        "prefix_after": "ééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé",
        "prefix_found": true,
        "suffix_before": "ééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééééé",
        "suffix_found": true,
        "text_byte_length": 1024,
        "needle_byte_length": 2
      }
    }
  ],
  "domain": "Each argument explicitly present decoded valid UTF-8 Text <=1024 bytes. Go tuple results mapped to required fields; arbitrary invalid UTF-8 and longer strings unsupported. No normalization or trimming."
}
`
const literalCutFrozenManualSHA256 = "4a1de6ea5e6756c03e3e0dceb39adf3fe2382e4a8265f7db867185ab26b1ee58"

type literalCutManualExpected struct {
	Text         string `json:"text"`
	Needle       string `json:"needle"`
	CutBefore    string `json:"cut_before"`
	CutAfter     string `json:"cut_after"`
	CutFound     bool   `json:"cut_found"`
	PrefixAfter  string `json:"prefix_after"`
	PrefixFound  bool   `json:"prefix_found"`
	SuffixBefore string `json:"suffix_before"`
	SuffixFound  bool   `json:"suffix_found"`
	TextBytes    int    `json:"text_byte_length"`
	NeedleBytes  int    `json:"needle_byte_length"`
}

func (a literalCutManualExpected) fields() map[string]any {
	return map[string]any{"text": a.Text, "needle": a.Needle, "cut_before": a.CutBefore, "cut_after": a.CutAfter, "cut_found": a.CutFound, "prefix_after": a.PrefixAfter, "prefix_found": a.PrefixFound, "suffix_before": a.SuffixBefore, "suffix_found": a.SuffixFound, "text_byte_length": a.TextBytes, "needle_byte_length": a.NeedleBytes}
}

type literalCutManualAnchor struct {
	Name     string                   `json:"name"`
	Text     string                   `json:"text"`
	Needle   string                   `json:"needle"`
	Expected literalCutManualExpected `json:"expected"`
}

func literalCutManualAnchors(t *testing.T) []literalCutManualAnchor {
	t.Helper()
	raw := []byte(literalCutFrozenManualContract)
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != literalCutFrozenManualSHA256 {
		t.Fatal("original manual32 byte seal changed")
	}
	var contract struct {
		Schema string                   `json:"schema"`
		Status string                   `json:"status"`
		Cases  []literalCutManualAnchor `json:"cases"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil || contract.Schema != "gooo/literal-cut-manual-contract/v1" || contract.Status != "FROZEN_BEFORE_ORACLE_OR_NATIVE" || len(contract.Cases) != 32 {
		t.Fatal("frozen manual anchor contract changed", err)
	}
	var fields struct {
		Cases []struct {
			Expected map[string]json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for i, row := range fields.Cases {
		if len(row.Expected) != 11 {
			t.Fatal("manual result must contain exactly eleven fields", i)
		}
		for key := range contract.Cases[i].Expected.fields() {
			value, ok := row.Expected[key]
			if !ok || string(value) == "null" {
				t.Fatal("manual result field is missing or null", i, key)
			}
		}
	}
	return contract.Cases
}

func TestLiteralCutFrozen32ManualAnchorsAndAllOriginalGoFields(t *testing.T) {
	anchors := literalCutManualAnchors(t)
	result, err := literalCutCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Suites) != 4 || result.CutReferenceCases != 32 || result.CutPrefixReferenceCases != 32 || result.CutSuffixReferenceCases != 32 || result.ReportReferenceCases != 32 || len(result.Pairs) != 32 {
		t.Fatal("the four unchanged32 reference rosters changed")
	}
	seen, names := map[literalTextPair]bool{}, map[string]bool{}
	for i, anchor := range anchors {
		pair, err := readLiteralTextPair([]any{anchor.Text, anchor.Needle})
		if err != nil || seen[pair] || names[anchor.Name] || pair != result.Pairs[i] {
			t.Fatal("original named input pair changed or duplicated", i, err)
		}
		seen[pair], names[anchor.Name] = true, true
		a := anchor.Expected
		before, after, found := strings.Cut(anchor.Text, anchor.Needle)
		prefix, prefixFound := strings.CutPrefix(anchor.Text, anchor.Needle)
		suffix, suffixFound := strings.CutSuffix(anchor.Text, anchor.Needle)
		if before != a.CutBefore || after != a.CutAfter || found != a.CutFound || prefix != a.PrefixAfter || prefixFound != a.PrefixFound || suffix != a.SuffixBefore || suffixFound != a.SuffixFound || a.Text != anchor.Text || a.Needle != anchor.Needle || a.TextBytes != len(anchor.Text) || a.NeedleBytes != len(anchor.Needle) {
			t.Fatal("independent manual tuple/11-field semantics disagree with original Go", anchor.Name)
		}
		want := []map[string]any{{"before": a.CutBefore, "after": a.CutAfter, "found": a.CutFound}, {"text": a.PrefixAfter, "found": a.PrefixFound}, {"text": a.SuffixBefore, "found": a.SuffixFound}, a.fields()}
		for kind, s := range result.Suites {
			if !isLiteralCutSuite(s) || len(s.Cases) != 32 || !reflect.DeepEqual(s.Cases[i].Inputs, []any{anchor.Text, anchor.Needle}) || !reflect.DeepEqual(s.Cases[i].Expected, want[kind]) {
				t.Fatal("input or complete original output record changed", anchor.Name, s.Activity)
			}
			got := s.Cases[i].Expected.(map[string]any)
			if len(got) != []int{3, 2, 2, 11}[kind] {
				t.Fatal("missing or additional result fields", s.Activity)
			}
		}
	}
	wantNames := []string{"Cut", "CutPrefix", "CutSuffix", "Report"}
	for i, s := range result.Suites {
		wantPackage := literalCutPackage
		if i == 3 {
			wantPackage = literalCutConsumerPackage
		}
		if s.Package != wantPackage || s.Activity != wantNames[i] {
			t.Fatal("ordered four-suite roster changed")
		}
	}
}

func TestLiteralCutRejectsEveryBadRosterBeforeAnyOracle(t *testing.T) {
	valid := []any{"é🙂", "🙂"}
	bad := []any{nil, false, true, 0, int64(1), []any{}, map[string]any{}, []byte("text"), string([]byte{0xff}), string([]byte{0xc0, 0x80}), strings.Repeat("a", 1025), strings.Repeat("🙂", 256) + "a"}
	for _, spec := range []struct{ pkg, activity string }{{literalCutPackage, "Cut"}, {literalCutPackage, "CutPrefix"}, {literalCutPackage, "CutSuffix"}, {literalCutConsumerPackage, "Report"}} {
		for position := 0; position < 2; position++ {
			for _, value := range bad {
				row := append([]any(nil), valid...)
				row[position] = value
				calls := 0
				_, err := literalCutReferenceSuite(spec.pkg, spec.activity, [][]any{valid, row}, func(string, string) any { calls++; return nil })
				if err == nil || calls != 0 {
					t.Fatal("invalid tail pair reached original Go oracle", spec.activity, position, calls)
				}
			}
		}
		for _, rows := range [][][]any{nil, {nil}, {{"one"}}, {{"a", "b", "extra"}}, {valid, valid}} {
			calls := 0
			_, err := literalCutReferenceSuite(spec.pkg, spec.activity, rows, func(string, string) any { calls++; return nil })
			if err == nil || calls != 0 {
				t.Fatal("empty/arity/duplicate roster reached original Go oracle", spec.activity, calls)
			}
		}
	}
	calls := 0
	if _, err := literalCutReferenceSuite(literalCutPackage, "Index", [][]any{valid}, func(string, string) any { calls++; return nil }); err == nil || calls != 0 {
		t.Fatal("unrelated native suite admitted as literal Cut")
	}
	if _, err := literalCutReferenceSuite(literalCutPackage, "Cut", [][]any{valid}, nil); err == nil {
		t.Fatal("missing independent oracle accepted")
	}
}

func TestLiteralCutSerializedBatchesKeepLargestPrefixAndAllOriginalFields(t *testing.T) {
	result, err := literalCutCorpus()
	if err != nil {
		t.Fatal(err)
	}
	limited, cases := false, 0
	for _, s := range result.Suites {
		batches := 0
		for start := 0; start < len(s.Cases); {
			limit := nativeBatchSize(s)
			end, err := nativeBatchEnd(s, start, limit)
			if err != nil || end <= start || end > min(start+limit, len(s.Cases)) {
				t.Fatal("invalid actual native batch boundary", s.Activity, start, end, err)
			}
			batch := s
			batch.Cases = s.Cases[start:end]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > stringByteMaxBatchBytes {
				t.Fatal("serialized Cut budget, including final LF", s.Activity, len(raw)+1, err)
			}
			if end < min(start+limit, len(s.Cases)) {
				limited = true
				larger := s
				larger.Cases = s.Cases[start : end+1]
				next, err := json.MarshalIndent(caseDocument(larger), "", "  ")
				if err != nil || len(next)+1 <= stringByteMaxBatchBytes {
					t.Fatal("native batch was not the largest fitting serialized prefix", s.Activity, start, end, err)
				}
			}
			var document struct {
				Cases []struct{ Inputs, Expected map[string]json.RawMessage }
			}
			if err := json.Unmarshal(raw, &document); err != nil || len(document.Cases) != end-start {
				t.Fatal("batch case roster changed", err)
			}
			key := s.Package + ":" + s.Activity
			for i, row := range document.Cases {
				if len(row.Inputs) != 2 || len(row.Expected) != 1 {
					t.Fatal("serialized two-input/one-output arity changed")
				}
				for arg := 0; arg < 2; arg++ {
					if !equalValue(row.Inputs[fmt.Sprintf("%s.input%d", key, arg)], batch.Cases[i].Inputs[arg]) {
						t.Fatal("wire serialization changed original UTF-8 Text", s.Activity, start+i, arg)
					}
				}
				if !equalValue(row.Expected[key], batch.Cases[i].Expected) {
					t.Fatal("wire serialization changed full original result", s.Activity, start+i)
				}
			}
			cases += end - start
			batches++
			start = end
		}
		t.Logf("%s:%s original_cases=%d observed_batches=%d", s.Package, s.Activity, len(s.Cases), batches)
	}
	if !limited || cases != 128 {
		t.Fatal("all128 activity observations or byte-boundary coverage missing", cases, limited)
	}
}

func TestLiteralCutInputsAndResultsDoNotAliasAcrossRowsSuitesOrCalls(t *testing.T) {
	result, err := literalCutCorpus()
	if err != nil {
		t.Fatal(err)
	}
	result.Suites[0].Cases[0].Inputs[0] = "changed input"
	result.Suites[0].Cases[0].Expected.(map[string]any)["before"] = "changed Cut result"
	result.Suites[1].Cases[0].Expected.(map[string]any)["text"] = "changed prefix result"
	result.Suites[3].Cases[0].Expected.(map[string]any)["cut_before"] = "changed report"
	for _, kind := range []int{1, 2, 3} {
		if result.Suites[kind].Cases[0].Inputs[0] != "" {
			t.Fatal("input slices alias across suites", kind)
		}
	}
	if result.Suites[0].Cases[1].Expected.(map[string]any)["before"] != "" || result.Suites[2].Cases[0].Expected.(map[string]any)["text"] != "" || result.Suites[3].Cases[1].Expected.(map[string]any)["cut_before"] != "" {
		t.Fatal("flat result maps alias across rows or activities")
	}
	result.Pairs[0].Text = "changed exposed pair"
	fresh, err := literalCutCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Pairs[0] != (literalTextPair{"", ""}) || fresh.Suites[0].Cases[0].Inputs[0] != "" || fresh.Suites[0].Cases[0].Expected.(map[string]any)["before"] != "" || fresh.Suites[3].Cases[0].Expected.(map[string]any)["cut_before"] != "" {
		t.Fatal("corpus calls share mutable input/result storage")
	}
}
