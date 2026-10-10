// Decode and display the UTF-16 sequence delivered by the browser. Classification
// and encoding lengths belong to the compiled Gooo functions in the worker.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooTextInput = api;
})(globalThis, () => {
  'use strict';
  const MAX_ITEMS = 256;
  const MAX_UNITS = 512;
  const MAX_JSON_SOURCE = 4096;
  const unitEscape = unit => '\\u' + unit.toString(16).toUpperCase().padStart(4, '0');

  function decode(text) {
    if (typeof text !== 'string') throw new Error('The input must be text.');
    if (text.length > MAX_UNITS) throw new Error('Input exceeds 512 UTF-16 code units. Nothing was inspected; shorten the whole input and try again.');
    const items = [];
    const allUnits = [];
    for (let offset = 0; offset < text.length;) {
      if (items.length === MAX_ITEMS) throw new Error('Input exceeds 256 decoded items. Nothing was inspected; shorten the whole input and try again.');
      const point = text.codePointAt(offset);
      const width = point > 0xFFFF ? 2 : 1;
      const units = [];
      for (let index = 0; index < width; index++) units.push(unitEscape(text.charCodeAt(offset + index)));
      allUnits.push(...units);
      items.push(Object.freeze({ decimal: String(point), notation: 'U+' + point.toString(16).toUpperCase().padStart(4, '0'),
        offset, width, units: units.join(' ') }));
      offset += width;
    }
    return Object.freeze({ items: Object.freeze(items), codeUnits: text.length, escaped: '"' + allUnits.join('') + '"' });
  }

  function read(source, mode) {
    if (typeof source !== 'string') throw new Error('The input must be text.');
    if (mode === 'text') return decode(source);
    if (mode !== 'json') throw new Error('Choose Text or JSON string input.');
    if (source.length > MAX_JSON_SOURCE) throw new Error('JSON notation exceeds 4,096 characters. Nothing was inspected.');
    let text;
    try { text = JSON.parse(source); }
    catch { throw new Error('Enter one valid JSON string, including its double quotes. Use escapes such as \\r\\n or \\uD800.'); }
    if (typeof text !== 'string') throw new Error('JSON input must be a string, not an array, object, number or other value.');
    return decode(text);
  }

  const presets = Object.freeze({
    letters: Object.freeze({ mode: 'text', source: 'A가𐐀1١𝟎²¼Ⅷe\u0301_', label: 'Letters, decimal digits, other numbers and a combining accent' }),
    mixed: Object.freeze({ mode: 'text', source: 'A\u00A0B\r\n\u200B😀', label: 'NBSP, CRLF, zero-width space and emoji' }),
    boundaries: Object.freeze({ mode: 'text', source: '\u0085\uFEFF', label: 'Next line and byte order mark' }),
    combined: Object.freeze({ mode: 'text', source: 'e\u0301👩‍💻', label: 'Combining accent and joined emoji' }),
    surrogates: Object.freeze({ mode: 'json', source: '"\\uD800A\\uDFFF\\uFFFD"', label: 'Lone surrogates and a replacement character, from JSON escapes' })
  });
  return Object.freeze({ read, decode, presets, MAX_ITEMS, MAX_UNITS, MAX_JSON_SOURCE });
});
