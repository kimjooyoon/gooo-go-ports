// Input-only links. No URL field can supply an execution result.
((root, factory) => {
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.GoooShare = api;
})(globalThis, () => {
  'use strict';
  const prefix = '#gooo-share?';
  const domains = Object.freeze({
    'http.StatusText': 'int64',
    'utf8.ValidRune': 'rune', 'utf8.RuneLen': 'rune', 'utf8.RuneStart': 'byte',
    'utf16.IsSurrogate': 'rune', 'utf16.RuneLen': 'rune',
    'utf16.EncodeRune': 'rune', 'utf16.DecodeRune': 'pair',
    'bits.Len8': 'byte', 'bits.LeadingZeros8': 'byte', 'bits.TrailingZeros8': 'byte',
    'bits.OnesCount8': 'byte', 'bits.Reverse8': 'byte'
  });
  const required = ['v', 'op', 'a', 'b', 'wasm', 'rev'];
  const decimal = /^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/;
  const hash = /^[a-f0-9]{64}$/;
  const revision = /^[a-f0-9]{40}$/;

  function validateInputs(operation, inputs) {
    if (typeof operation !== 'string' || !Object.hasOwn(domains, operation)) throw new Error('This function is not supported.');
    if (!Array.isArray(inputs) || inputs.length !== 2 || inputs.some(value => typeof value !== 'string')) {
      throw new Error('The link must contain exactly two input strings.');
    }
    const domain = domains[operation];
    for (const value of inputs) {
      if (value.length > 20 || !decimal.test(value)) throw new Error('Use decimal integers without spaces, a plus sign, or leading zeros.');
      const integer = BigInt(value);
      const minimum = domain === 'int64' ? -9223372036854775808n : domain === 'byte' ? 0n : -2147483648n;
      const maximum = domain === 'int64' ? 9223372036854775807n : domain === 'byte' ? 255n : 2147483647n;
      if (integer < minimum || integer > maximum) throw new Error('An input is outside this function’s accepted range.');
    }
    if (domain !== 'pair' && inputs[1] !== '0') throw new Error('This function accepts one input; its unused second value must be 0.');
  }

  function validateState(state) {
    if (!state || typeof state !== 'object' || Array.isArray(state) ||
        Object.keys(state).sort().join(',') !== 'inputs,operation,revision,wasm') throw new Error('The shared input has unexpected fields.');
    validateInputs(state.operation, state.inputs);
    if (typeof state.wasm !== 'string' || !hash.test(state.wasm) ||
        typeof state.revision !== 'string' || !revision.test(state.revision)) throw new Error('The shared program details are invalid.');
  }

  function parse(fragment) {
    if (typeof fragment !== 'string') throw new Error('The shared link is invalid.');
    if (!fragment.startsWith('#gooo-share')) return null; // Ordinary page anchors still work.
    if (fragment.length > 1024) throw new Error('The shared link is too long.');
    if (!fragment.startsWith(prefix)) throw new Error('The shared-link format is not supported.');
    const values = new URLSearchParams(fragment.slice(prefix.length));
    const seen = new Set();
    for (const [key] of values) {
      if (!required.includes(key) || seen.has(key)) throw new Error('The shared link has unknown or repeated fields.');
      seen.add(key);
    }
    if (seen.size !== required.length) throw new Error('The shared link is missing required fields.');
    if (values.get('v') !== '1') throw new Error('This shared-link version is not supported.');
    const state = { operation: values.get('op'), inputs: [values.get('a'), values.get('b')],
      wasm: values.get('wasm'), revision: values.get('rev') };
    validateState(state);
    return state;
  }

  function format(state) {
    validateState(state);
    return prefix + new URLSearchParams({ v: '1', op: state.operation, a: state.inputs[0],
      b: state.inputs[1], wasm: state.wasm, rev: state.revision }).toString();
  }

  return Object.freeze({ parse, format, validateInputs });
});
