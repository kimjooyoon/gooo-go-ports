// Local browser observations only. This module preserves and validates received
// values; it neither runs a reference oracle nor creates a compiler receipt.
((root, factory) => {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory(require('./text-input.js'), require('./text-inspector-protocol.js'));
  } else {
    root.GoooTextInspectionReport = factory(root.GoooTextInput, root.GoooTextInspectorProtocol);
  }
})(globalThis, (textInput, protocol) => {
  'use strict';
  const schema = 'gooo/text-inspection-report/v1';
  const kind = 'browser-observation';
  const scope = 'Unsigned local browser observation of four compiled Gooo operations. Not a native compiler receipt or an independent reference-test result.';
  const rowKeys = ['codePoint', 'offset', 'width', 'validScalar', 'goSpace', 'utf8Bytes', 'utf16Units'];
  const totalKeys = ['codePoints', 'receivedUtf16Units', 'goWhitespaceItems', 'utf8Bytes', 'utf16Units'];

  function fields(value, names, label) {
    if (!value || typeof value !== 'object' || Array.isArray(value) ||
        Object.keys(value).sort().join(',') !== [...names].sort().join(',')) {
      throw new Error(`Invalid ${label} fields.`);
    }
  }
  function freeze(value) {
    if (value && typeof value === 'object') {
      Object.values(value).forEach(freeze);
      Object.freeze(value);
    }
    return value;
  }
  function checkedBinding(binding) {
    fields(binding, ['wasmSha256', 'sourceRevision'], 'program identity');
    return protocol.validateReady({ type: 'ready', ...binding });
  }
  function checkedSnapshot(snapshot) {
    fields(snapshot, ['items', 'codeUnits', 'escaped'], 'input snapshot');
    if (typeof snapshot.escaped !== 'string' || snapshot.escaped.length > textInput.MAX_UNITS * 6 + 2) {
      throw new Error('Invalid escaped input snapshot.');
    }
    const decoded = textInput.read(snapshot.escaped, 'json');
    if (snapshot.escaped !== decoded.escaped || !Object.is(snapshot.codeUnits, decoded.codeUnits) ||
        !Array.isArray(snapshot.items) || snapshot.items.length !== decoded.items.length) {
      throw new Error('Input snapshot does not match its original UTF-16 sequence.');
    }
    const names = ['decimal', 'notation', 'offset', 'width', 'units'];
    decoded.items.forEach((item, index) => {
      fields(snapshot.items[index], names, 'input item');
      if (names.some(name => !Object.is(snapshot.items[index][name], item[name]))) {
        throw new Error('Input item or position does not match its original UTF-16 sequence.');
      }
    });
    return decoded;
  }
  function originalUnits(decoded) {
    const raw = JSON.parse(decoded.escaped);
    return Array.from({ length: raw.length }, (_, offset) => raw.charCodeAt(offset));
  }

  function create(snapshot, rows, binding) {
    const decoded = checkedSnapshot(snapshot);
    const program = checkedBinding(binding);
    const points = decoded.items.map(item => item.decimal);
    // The UI has already bound the completed Worker request. Reuse its strict
    // record validation to check the exact input roster without classifying it.
    const checked = protocol.validateResult({ type: 'result', id: 1, rows, ...program }, { id: 1, points, ...program });
    const resultRows = checked.map((row, index) => ({
      codePoint: Number(points[index]), offset: decoded.items[index].offset, width: decoded.items[index].width,
      validScalar: row.validScalar, goSpace: row.goSpace, utf8Bytes: row.utf8Bytes, utf16Units: row.utf16Units
    }));
    const encodable = checked.every(row => row.validScalar && row.utf8Bytes !== '-1' && row.utf16Units !== '-1');
    const totals = {
      codePoints: checked.length,
      receivedUtf16Units: decoded.codeUnits,
      goWhitespaceItems: checked.filter(row => row.goSpace).length,
      utf8Bytes: encodable ? checked.reduce((sum, row) => sum + Number(row.utf8Bytes), 0) : null,
      utf16Units: encodable ? checked.reduce((sum, row) => sum + Number(row.utf16Units), 0) : null
    };
    return freeze({ schema, kind, scope, program,
      input: { utf16Units: originalUnits(decoded), escapedJson: decoded.escaped }, rows: resultRows, totals });
  }

  function validate(report) {
    fields(report, ['schema', 'kind', 'scope', 'program', 'input', 'rows', 'totals'], 'report');
    if (report.schema !== schema || report.kind !== kind || report.scope !== scope) throw new Error('Unsupported observation report.');
    const program = checkedBinding(report.program);
    fields(report.input, ['utf16Units', 'escapedJson'], 'original input');
    const units = report.input.utf16Units;
    if (!Array.isArray(units) || units.length > textInput.MAX_UNITS) throw new Error('Invalid original UTF-16 units.');
    for (const unit of units) {
      if (!Number.isInteger(unit) || unit < 0 || unit > 65535 || Object.is(unit, -0)) throw new Error('Invalid original UTF-16 unit.');
    }
    const decoded = textInput.decode(String.fromCharCode(...units));
    if (report.input.escapedJson !== decoded.escaped) throw new Error('Escaped input differs from the original UTF-16 units.');
    if (!Array.isArray(report.rows) || report.rows.length !== decoded.items.length) throw new Error('Report row roster differs from its input.');
    const rows = decoded.items.map((item, index) => {
      const row = report.rows[index];
      fields(row, rowKeys, 'report row');
      if (!Object.is(row.codePoint, Number(item.decimal)) || !Object.is(row.offset, item.offset) || !Object.is(row.width, item.width)) {
        throw new Error('Report code point or position differs from its input.');
      }
      return { input: item.decimal, validScalar: row.validScalar, goSpace: row.goSpace,
        utf8Bytes: row.utf8Bytes, utf16Units: row.utf16Units };
    });
    const canonical = create(decoded, rows, program);
    fields(report.totals, totalKeys, 'report totals');
    if (totalKeys.some(name => !Object.is(report.totals[name], canonical.totals[name]))) {
      throw new Error('Report totals differ from its observed rows.');
    }
    return canonical;
  }
  function serialize(report) {
    return JSON.stringify(validate(report), null, 2) + '\n';
  }
  return Object.freeze({ create, serialize });
});
