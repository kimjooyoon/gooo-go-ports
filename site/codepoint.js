// Decode one entered UTF-16 sequence for the numeric input. No whitespace
// classification occurs here: only the compiled Gooo function supplies results.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooCodePoint = api;
})(globalThis, () => {
  'use strict';
  function fromText(text) {
    if (typeof text !== 'string' || text.length === 0 || text.length > 2 || [...text].length !== 1) {
      throw new Error('Enter exactly one code point. Combining sequences, joined emoji and CRLF contain more than one.');
    }
    const point = text.codePointAt(0);
    return Object.freeze({ decimal: String(point), notation: 'U+' + point.toString(16).toUpperCase().padStart(4, '0'),
      scalar: point < 0xD800 || point > 0xDFFF, carriageReturn: point === 13 });
  }
  return Object.freeze({ fromText });
});
