'use strict';

const points = [];
const unicodePropertyPoints = [];
let count = 0;
for (let cp = 0; cp <= 0x10ffff; cp++) {
  if (cp >= 0xd800 && cp <= 0xdfff) continue;
  count++;
  const value = String.fromCodePoint(cp);
  const empty = value.trim() === '';
  if (empty !== /^\s$/u.test(value)) throw new Error(`trim/RegExp disagreement: ${cp}`);
  if (empty) points.push(cp);
  if (/^\p{White_Space}$/u.test(value)) unicodePropertyPoints.push(cp);
}
const samples = [0x85, 0xfeff, 0x200b, 0xa0].map(cp => ({
  code_point: `U+${cp.toString(16).toUpperCase().padStart(4, '0')}`,
  trim_empty: String.fromCodePoint(cp).trim() === '',
  regexp_space: /^\s$/u.test(String.fromCodePoint(cp)),
  unicode_white_space: /^\p{White_Space}$/u.test(String.fromCodePoint(cp)),
}));
process.stdout.write(JSON.stringify({
  runtime: process.version, v8: process.versions.v8, unicode_version: process.versions.unicode,
  domain: 'Unicode scalar values U+0000..U+10FFFF excluding U+D800..U+DFFF',
  examined: count, whitespace_code_points: points, whitespace_count: points.length,
  unicode_property_code_points: unicodePropertyPoints,
  trim_regexp_disagreements: 0, samples,
}, null, 2) + '\n');
