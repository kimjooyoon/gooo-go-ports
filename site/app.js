(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const operations = {
    'http.StatusText': { scenario: 'http', path: 'go/net/http', source: 'ports/net/http.gooo', label: 'Status code', domain: 'int64', example: '404', description: 'Look up the standard description of an HTTP status code.', help: 'Use a decimal integer. Unknown status codes return an empty string.' },
    'utf8.ValidRune': { scenario: 'unicode', path: 'go/unicode/utf8', source: 'ports/unicode/utf8.gooo', label: 'Code point', domain: 'int32 rune', example: '128512', description: 'Check whether a code point is a valid Unicode scalar value.', help: 'Use a decimal integer. Surrogates and values outside the Unicode scalar range are invalid.' },
    'utf8.RuneLen': { scenario: 'unicode', path: 'go/unicode/utf8', source: 'ports/unicode/utf8.gooo', label: 'Code point', domain: 'int32 rune', example: '128512', description: 'Find the UTF-8 byte length of one Unicode code point.', help: 'Returns 1–4 bytes for a valid scalar value, or −1 for an invalid code point.' },
    'utf8.RuneStart': { scenario: 'bytes', path: 'go/unicode/utf8', source: 'ports/unicode/utf8.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '170', description: 'Check whether a byte is not a UTF-8 continuation byte.', help: 'This tests one byte. A true result does not validate a complete UTF-8 sequence.' },
    'utf16.IsSurrogate': { scenario: 'unicode', path: 'go/unicode/utf16', source: 'ports/unicode/utf16.gooo', label: 'Code point', domain: 'int32 rune', example: '55357', description: 'Check whether a value is in the UTF-16 surrogate range.', help: 'Surrogates occupy decimal values 55296–57343. They are not Unicode scalar values.' },
    'utf16.RuneLen': { scenario: 'unicode', path: 'go/unicode/utf16', source: 'ports/unicode/utf16.gooo', label: 'Code point', domain: 'int32 rune', example: '128512', description: 'Find the number of UTF-16 code units for one code point.', help: 'Returns 1 or 2 code units for a valid scalar value, or −1 for an invalid code point.' },
    'utf16.DecodeRune': { scenario: 'unicode', path: 'go/unicode/utf16', source: 'ports/unicode/utf16.gooo', label: 'High surrogate', domain: 'two int32 runes', example: '55357', second: '56832', description: 'Decode a UTF-16 high and low surrogate pair into one code point.', help: 'Use decimal integers. An invalid pair returns the replacement code point, 65533.' },
    'utf16.EncodeRune': { scenario: 'unicode', path: 'go/unicode/utf16', source: 'ports/unicode/utf16.gooo', label: 'Code point', domain: 'int32 rune', example: '128512', description: 'Encode a supplementary code point as a UTF-16 surrogate pair.', help: 'Try 128512 (U+1F600). Values outside 65536–1114111 return {high: 65533, low: 65533}.' },
    'bits.Len8': { scenario: 'bytes', path: 'go/math/bits', source: 'ports/math/bits.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '170', description: 'Find how many bits are needed to represent an unsigned byte.', help: 'Returns 0 for zero and 1–8 for nonzero byte values.' },
    'bits.LeadingZeros8': { scenario: 'bytes', path: 'go/math/bits', source: 'ports/math/bits.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '15', description: 'Count the zero bits before the first one in an 8-bit byte.', help: 'All eight positions count: a zero byte has 8 leading zeros.' },
    'bits.TrailingZeros8': { scenario: 'bytes', path: 'go/math/bits', source: 'ports/math/bits.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '128', description: 'Count the zero bits after the last one in an 8-bit byte.', help: 'A zero byte has 8 trailing zeros.' },
    'bits.OnesCount8': { scenario: 'bytes', path: 'go/math/bits', source: 'ports/math/bits.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '170', description: 'Count the one bits in the binary representation of a byte.', help: 'For example, decimal 170 is binary 10101010 and contains four one bits.' },
    'bits.Reverse8': { scenario: 'bytes', path: 'go/math/bits', source: 'ports/math/bits.gooo', label: 'Byte value', domain: 'byte · 0…255', example: '170', description: 'Reverse the order of all eight bits in a byte.', help: 'Try decimal 170: binary 10101010 becomes 01010101.' }
  };
  const scenarios = { http: 'http.StatusText', unicode: 'utf16.EncodeRune', bytes: 'bits.Reverse8' };
  let ready = false;
  let busy = false;
  let checksRunning = false;
  let metadata = null;

  function setRuntime(message, state) {
    $('runtime-message').textContent = message;
    $('runtime-status').className = `runtime-status ${state || ''}`;
    $('reload-button').hidden = state !== 'error';
  }

  function updateButtons() {
    $('run-button').disabled = !ready || busy || checksRunning;
    $('checks-button').disabled = !ready || busy || checksRunning;
    $('run-button').firstChild.textContent = busy ? 'Running… ' : 'Run function ';
    $('checks-button').firstChild.textContent = checksRunning ? 'Checking… ' : 'Run reference checks ';
  }

  function clearResult() {
    $('result-badge').textContent = 'NOT RUN';
    $('result-badge').className = 'result-badge';
    $('raw-result').hidden = true;
    $('raw-result').open = false;
    $('raw-output').textContent = '';
    $('result-content').replaceChildren();
    const title = document.createElement('p');
    title.className = 'result-placeholder';
    title.textContent = 'Ready for a new input';
    const detail = document.createElement('p');
    detail.className = 'result-detail';
    detail.textContent = 'Run the function to see its actual result.';
    $('result-content').append(title, detail);
  }

  function setSourceLink() {
    const link = $('function-source');
    if (!metadata) { link.hidden = true; return; }
    const source = operations[$('operation').value].source;
    link.href = metadata.source_dirty
      ? `./source/${source}`
      : `https://github.com/kimjooyoon/gooo-go-ports/blob/${metadata.source_revision}/${source}`;
    link.hidden = false;
  }

  function selectOperation(name, resetInput = true) {
    const operation = operations[name];
    $('operation').value = name;
    $('function-description').textContent = operation.description;
    $('first-label').textContent = operation.label;
    $('second-field').hidden = !operation.second;
    $('second-input').disabled = !operation.second;
    $('input-domain').replaceChildren(document.createTextNode('Input domain '));
    const domain = document.createElement('code');
    domain.textContent = operation.domain;
    $('input-domain').append(domain);
    $('input-range').textContent = operation.domain.startsWith('byte')
      ? '0 to 255; values outside this range are rejected.'
      : name === 'http.StatusText'
        ? '-9223372036854775808 to 9223372036854775807'
        : '-2147483648 to 2147483647 for each input';
    $('input-help').textContent = operation.help;
    $('package-path').textContent = operation.path;
    if (resetInput) {
      $('first-input').value = operation.example;
      $('second-input').value = operation.second || '0';
    }
    document.querySelectorAll('[data-scenario]').forEach((button) => {
      const active = button.dataset.scenario === operation.scenario;
      button.classList.toggle('active', active);
      button.setAttribute('aria-pressed', String(active));
    });
    setSourceLink();
    clearResult();
  }

  function validOutput(output) {
    if (!output || typeof output !== 'object') return false;
    if (output.type === 'text') return typeof output.value === 'string';
    if (output.type === 'integer') return typeof output.value === 'string' && /^-?\d+$/.test(output.value);
    if (output.type === 'boolean') return typeof output.value === 'boolean';
    return output.type === 'pair' && output.value && typeof output.value.high === 'string' &&
      typeof output.value.low === 'string' && /^-?\d+$/.test(output.value.high) && /^-?\d+$/.test(output.value.low);
  }

  function sameInputs(actual, expected) {
    return Array.isArray(actual) && actual.length === expected.length && actual.every((value, i) => value === expected[i]);
  }

  function sameOutput(actual, expected) {
    if (!validOutput(actual) || !validOutput(expected) || actual.type !== expected.type) return false;
    return actual.type === 'pair'
      ? actual.value.high === expected.value.high && actual.value.low === expected.value.low
      : actual.value === expected.value;
  }

  function invoke(operation, inputs) {
    if (!ready || typeof window.goooRun !== 'function') throw new Error('The WebAssembly runtime is not ready. Try reloading the page.');
    const raw = window.goooRun(operation, inputs[0], inputs[1] === undefined ? '0' : inputs[1]);
    if (typeof raw !== 'string') throw new Error('The runtime returned an unexpected response.');
    let result;
    try { result = JSON.parse(raw); } catch { throw new Error('The runtime returned invalid JSON.'); }
    if (!result || typeof result.ok !== 'boolean') throw new Error('The runtime response is missing its status.');
    if (result.ok && (result.operation !== operation || !sameInputs(result.inputs, inputs) || !validOutput(result.output))) {
      throw new Error('The runtime result did not match the requested function, inputs, or output format.');
    }
    if (!result.ok && (typeof result.error !== 'string' || !result.error)) throw new Error('The runtime returned an invalid error response.');
    return result;
  }

  function showError(message) {
    $('result-badge').textContent = 'ERROR';
    $('result-badge').className = 'result-badge error';
    $('result-content').replaceChildren();
    const error = document.createElement('p');
    error.className = 'result-error';
    error.textContent = message;
    const detail = document.createElement('p');
    detail.className = 'result-detail';
    detail.textContent = 'No successful result is being shown for these inputs.';
    $('result-content').append(error, detail);
    $('raw-result').hidden = true;
  }

  function showResult(result) {
    $('result-badge').textContent = 'EXECUTED';
    $('result-badge').className = 'result-badge success';
    const output = result.output;
    const area = $('result-content');
    area.replaceChildren();
    const type = document.createElement('p');
    type.className = 'result-type';
    type.textContent = output.type === 'pair' ? 'SURROGATE PAIR' : output.type;
    area.append(type);
    if (output.type === 'pair') {
      const pair = document.createElement('div');
      pair.className = 'pair-result';
      for (const key of ['high', 'low']) {
        const part = document.createElement('div');
        part.className = 'pair-part';
        const label = document.createElement('span');
        label.textContent = key;
        const value = document.createElement('strong');
        value.textContent = output.value[key];
        part.append(label, value);
        pair.append(part);
      }
      area.append(pair);
    } else {
      const value = document.createElement('p');
      value.className = `result-value${output.type === 'text' ? ' text-result' : ''}${output.value === true ? ' boolean-true' : ''}`;
      value.textContent = output.value === '' ? '""' : String(output.value);
      area.append(value);
    }
    const note = document.createElement('p');
    note.className = 'result-detail';
    note.textContent = output.value === '' ? 'Empty string · this status code has no registered description.' : 'Returned by the compiled Gooo function.';
    area.append(note);
    $('raw-output').textContent = JSON.stringify(result, null, 2);
    $('raw-result').hidden = false;
  }

  $('run-form').addEventListener('submit', (event) => {
    event.preventDefault();
    if (!ready || busy || checksRunning) return;
    clearResult();
    const name = $('operation').value;
    const inputs = [$('first-input').value, operations[name].second ? $('second-input').value : '0'];
    if (inputs.some((value) => !/^(?:0|[1-9][0-9]*|-[1-9][0-9]*)$/.test(value))) {
      showError('Enter a decimal integer, such as 0, -1, or 404. Do not add spaces, a plus sign, or leading zeros. Use 0 rather than -0.');
      return;
    }
    busy = true;
    updateButtons();
    try {
      const result = invoke(name, inputs);
      if (!result.ok) showError(result.error);
      else showResult(result);
    } catch (error) { showError(error.message); }
    finally { busy = false; updateButtons(); }
  });

  $('operation').addEventListener('change', () => selectOperation($('operation').value));
  $('first-input').addEventListener('input', clearResult);
  $('second-input').addEventListener('input', clearResult);
  document.querySelectorAll('[data-scenario]').forEach((button) => button.addEventListener('click', () => selectOperation(scenarios[button.dataset.scenario])));
  $('reload-button').addEventListener('click', () => window.location.reload());

  $('checks-button').addEventListener('click', async () => {
    if (!ready || busy || checksRunning) return;
    checksRunning = true;
    updateButtons();
    const status = $('checks-status');
    const progress = $('checks-progress');
    status.className = 'checks-status';
    status.textContent = 'Loading the bundled reference cases…';
    progress.hidden = true;
    let passed = 0;
    let rejected = 0;
    let compared = 0;
    let rejectionChecks = 0;
    const failures = [];
    try {
      const response = await fetch('./fixtures.json');
      if (!response.ok) throw new Error(`Reference cases could not be loaded (HTTP ${response.status}).`);
      const fixtures = await response.json();
      if (!Array.isArray(fixtures.cases) || fixtures.cases.length === 0 || !Array.isArray(fixtures.rejections)) throw new Error('The fixture file has an unexpected format.');
      const total = fixtures.cases.length + fixtures.rejections.length;
      progress.max = total;
      progress.value = 0;
      progress.hidden = false;
      const tick = async () => {
        progress.value = compared + rejectionChecks;
        status.textContent = `${compared} / ${fixtures.cases.length} reference cases checked; ${rejectionChecks} / ${fixtures.rejections.length} invalid-input checks.`;
        await new Promise((resolve) => window.setTimeout(resolve, 0));
      };
      for (const item of fixtures.cases) {
        if (!item || typeof item.operation !== 'string' || !Array.isArray(item.inputs) || item.inputs.length !== 2 || item.inputs.some((input) => typeof input !== 'string') || !validOutput(item.expected)) throw new Error('A reference case has an unexpected format.');
        try {
          const result = invoke(item.operation, item.inputs);
          if (result.ok && sameOutput(result.output, item.expected)) passed++;
          else failures.push(`${item.operation}(${item.inputs.join(', ')}): result did not match the Go reference.`);
        } catch (error) { failures.push(`${item.operation}: ${error.message}`); }
        compared++;
        if (compared % 25 === 0) await tick();
      }
      for (const item of fixtures.rejections) {
        if (!item || typeof item.operation !== 'string' || !Array.isArray(item.inputs) || item.inputs.length !== 2 || item.inputs.some((input) => typeof input !== 'string')) throw new Error('An invalid-input case has an unexpected format.');
        try {
          const result = invoke(item.operation, item.inputs);
          if (!result.ok) rejected++;
          else failures.push(`${item.operation}(${item.inputs.join(', ')}): an invalid input was accepted.`);
        } catch (error) { failures.push(`${item.operation}: ${error.message}`); }
        rejectionChecks++;
        if (rejectionChecks % 25 === 0) await tick();
      }
      progress.value = total;
      status.className = `checks-status ${failures.length ? 'error' : 'success'}`;
      status.textContent = `${passed} / ${compared} reference cases matched. ${rejected} / ${rejectionChecks} invalid-input checks rejected as expected. ${failures.length ? `First failure: ${failures[0]}` : 'Run completed in this browser.'}`;
    } catch (error) {
      status.className = 'checks-status error';
      status.textContent = `${error.message} Completed: ${passed} / ${compared} reference matches; ${rejected} / ${rejectionChecks} expected rejections. The check run is incomplete.`;
    } finally { checksRunning = false; updateButtons(); }
  });

  async function loadMetadata() {
    try {
      const response = await fetch('./metadata.json');
      if (!response.ok) throw new Error('Build metadata could not be loaded.');
      const data = await response.json();
      if (!data || !/^[a-f0-9]{40}$/.test(data.compiler_revision) || !/^[a-f0-9]{40}$/.test(data.source_revision) || typeof data.compiler_version !== 'string' || typeof data.go_version !== 'string') throw new Error('Build metadata is incomplete.');
      metadata = data;
      $('compiler-link').textContent = `${data.compiler_version} · ${data.compiler_revision.slice(0, 8)}`;
      $('compiler-link').href = `https://github.com/kimjooyoon/meta-ontology-go/tree/${data.compiler_revision}`;
      $('revision-link').textContent = data.source_dirty ? `${data.source_revision.slice(0, 8)} + local changes` : data.source_revision.slice(0, 12);
      $('revision-link').href = data.source_dirty ? './metadata.json' : `https://github.com/kimjooyoon/gooo-go-ports/tree/${data.source_revision}`;
      $('go-version').textContent = data.go_version;
      $('build-details').hidden = false;
      $('metadata-status').hidden = true;
      setSourceLink();
    } catch (error) {
      $('metadata-status').textContent = `${error.message} The downloadable evidence links remain below.`;
    }
  }

  async function loadRuntime() {
    try {
      if (typeof WebAssembly === 'undefined') throw new Error('This browser does not support WebAssembly.');
      if (typeof window.Go !== 'function') throw new Error('The Go WebAssembly loader could not be loaded.');
      const go = new window.Go();
      const response = await fetch('./demo.wasm');
      if (!response.ok) throw new Error(`The WebAssembly program could not be loaded (HTTP ${response.status}).`);
      let loaded;
      if (typeof WebAssembly.instantiateStreaming === 'function') {
        try { loaded = await WebAssembly.instantiateStreaming(response.clone(), go.importObject); }
        catch { loaded = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject); }
      } else { loaded = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject); }
      let runtimeError = null;
      Promise.resolve(go.run(loaded.instance)).then(() => {
        runtimeError = new Error('The WebAssembly runtime stopped. Reload to try again.');
        ready = false;
        updateButtons();
        setRuntime(runtimeError.message, 'error');
      }).catch((error) => {
        runtimeError = error;
        ready = false;
        updateButtons();
        setRuntime(`Runtime error: ${error.message}`, 'error');
      });
      const deadline = Date.now() + 20000;
      while (window.goooReady !== true || typeof window.goooRun !== 'function') {
        if (runtimeError) throw runtimeError;
        if (Date.now() > deadline) throw new Error('The WebAssembly runtime did not become ready.');
        await new Promise((resolve) => window.setTimeout(resolve, 25));
      }
      if (runtimeError) throw runtimeError;
      ready = true;
      updateButtons();
      setRuntime('WebAssembly ready', 'ready');
    } catch (error) {
      ready = false;
      updateButtons();
      setRuntime(error.message, 'error');
    }
  }

  loadMetadata();
  loadRuntime();
})();
