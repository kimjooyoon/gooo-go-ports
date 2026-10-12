(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooUUIDParseProtocol;
  const names = ['input'];
  let snapshots = [null], origins = [''];
  let worker = null, program = null, job = null, completed = null, nextId = 0;
  let loadTimer = null, runTimer = null;
  const downloads = new Map();
  function clearTimer(name) {
    const value = name === 'load' ? loadTimer : runTimer;
    if (value !== null) window.clearTimeout(value);
    if (name === 'load') loadTimer = null; else runTimer = null;
  }
  function updateButtons() {
    $('uuid-run').disabled = !program || snapshots.some(value => !value) || !!job;
    $('uuid-run').textContent = job ? 'Parsing…' : 'Parse UUID text';
    $('uuid-cancel').hidden = !job;
    const available = !!completed && !!program && !job && completed.snapshots.every((value, index) => value === snapshots[index]);
    $('uuid-download-controls').hidden = !available;
    $('uuid-download').disabled = !available;
  }
  function clearResult(status = 'No parsing has run.') {
    completed = null;
    $('uuid-output').replaceChildren();
    $('uuid-output-wrap').hidden = true;
    $('uuid-result-error').hidden = true;
    $('uuid-result-error').textContent = '';
    $('uuid-result-note').hidden = true;
    $('uuid-result-note').textContent = '';
    $('uuid-progress').textContent = status;
    $('uuid-download-status').hidden = true;
    $('uuid-download-status').textContent = '';
    updateButtons();
  }
  function failed(message) {
    clearTimer('load'); clearTimer('run');
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    clearResult('Parser unavailable.');
    $('uuid-runtime').textContent = message;
    $('uuid-runtime').className = 'literal-runtime error';
    $('uuid-result-error').textContent = message;
    $('uuid-result-error').hidden = false;
    $('uuid-program').textContent = 'Program identity is unavailable.';
    $('uuid-retry').hidden = false;
    updateButtons();
  }
  function loadWorker() {
    clearTimer('load'); clearTimer('run');
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    $('uuid-runtime').textContent = 'Loading the compiled UUID parser…';
    $('uuid-runtime').className = 'literal-runtime';
    $('uuid-program').textContent = 'Waiting for the verified program manifest.';
    $('uuid-retry').hidden = true;
    updateButtons();
    try {
      const instance = new Worker('./uuid-parse-worker.js');
      worker = instance;
      loadTimer = window.setTimeout(() => {
        if (worker === instance && !program) failed('Program loading timed out after 30 seconds. Retry loading.');
      }, 30000);
      instance.onmessage = event => {
        if (worker !== instance) return;
        const message = event.data;
        try {
          if (!message || typeof message !== 'object') throw new Error('The worker returned an invalid message.');
          if (message.type === 'ready') {
            if (program) throw new Error('The worker returned a duplicate ready message.');
            program = protocol.validateReady(message);
            clearTimer('load');
            $('uuid-runtime').textContent = 'WebAssembly ready · parser inputs stay in this browser';
            $('uuid-program').textContent = `WebAssembly SHA-256 ${program.wasm_sha256} · source revision ${program.source_revision}` +
              (program.source_dirty ? ' · build includes source changes' : '') + ` · compiler ${program.compiler_version} · ${program.go_version}`;
            updateButtons();
          } else if (message.type === 'result') {
            if (!job || message.id !== job.request.id) return;
            const output = protocol.validateResult(message, job.request);
            clearTimer('run');
            const current = job;
            job = null;
            showOutput(output);
            completed = Object.freeze({ snapshots: current.snapshots, origins: current.origins, output, program: current.request.program });
            updateButtons();
          } else if (message.type === 'error') {
            if (message.id !== undefined && (!job || message.id !== job.request.id)) return;
            failed(protocol.validateError(message, job ? job.request.id : undefined));
          } else throw new Error('The worker returned an unknown message type.');
        } catch (error) { failed(error.message); }
      };
      instance.onerror = event => {
        if (worker !== instance) return;
        event.preventDefault?.();
        failed('The parser worker stopped. Retry loading to parse the current input.');
      };
      instance.onmessageerror = () => { if (worker === instance) failed('The parser worker returned an unreadable message.'); };
    } catch { failed('The parser worker could not start. Retry loading in a browser with Worker support.'); }
  }
  function invalidate() {
    if (job) loadWorker();
    clearResult();
  }
  function accept(index, source, origin) {
    invalidate();
    const name = names[index];
    snapshots[index] = null; origins[index] = origin;
    $('uuid-' + name + '-source').textContent = origin;
    $('uuid-' + name).setAttribute('aria-invalid', 'false');
    const status = $('uuid-' + name + '-status');
    status.className = 'literal-input-status';
    try {
      const snapshot = protocol.read(source, $('uuid-' + name + '-mode').value);
      snapshots[index] = snapshot;
      $('uuid-' + name + '-preview').textContent = snapshot.originalEscaped;
      $('uuid-' + name + '-bound').textContent = snapshot.boundEscaped;
      $('uuid-' + name + '-bound-wrap').hidden = !snapshot.changed;
      status.textContent = `${snapshot.originalUtf16Units.length} original UTF-16 code units · ${snapshot.utf8Bytes} bound UTF-8 bytes. ` +
        (snapshot.changed ? 'Unpaired UTF-16 surrogates were replaced with U+FFFD; the bound input is shown below.' : 'Original and bound input are identical.');
    } catch (error) {
      $('uuid-' + name).setAttribute('aria-invalid', 'true');
      $('uuid-' + name + '-preview').textContent = 'Input was not accepted. The parser was not called.';
      $('uuid-' + name + '-bound').textContent = '';
      $('uuid-' + name + '-bound-wrap').hidden = true;
      status.className = 'literal-input-status error'; status.textContent = error.message;
    }
    updateButtons();
  }
  function updateMode(index) {
    const name = names[index], json = $('uuid-' + name + '-mode').value === 'json';
    $('uuid-' + name + '-label').textContent = 'UUID text' + (json ? ' — one JSON string' : ' — type or paste');
    $('uuid-' + name).placeholder = json ? '"Use \\r\\n or \\uD800 escapes"' : 'Any decoded Text up to 1,024 UTF-8 bytes';
  }
  function transfer(index, event, kind) {
    event.preventDefault();
    const data = kind === 'paste' ? event.clipboardData : event.dataTransfer;
    const name = names[index];
    $('uuid-' + name).value = '';
    if (!data || typeof data.getData !== 'function' || data.files?.length ||
        (data.types?.length && !Array.from(data.types).includes('text/plain'))) {
      accept(index, '', 'No plain text was received.');
      snapshots[index] = null;
      $('uuid-' + name).setAttribute('aria-invalid', 'true');
      $('uuid-' + name + '-status').className = 'literal-input-status error';
      $('uuid-' + name + '-status').textContent = 'Paste or drop plain text. Files and other clipboard formats are not read.';
      updateButtons(); return;
    }
    let source;
    try { source = data.getData('text/plain'); }
    catch {
      accept(index, '', 'The browser did not provide plain text.'); snapshots[index] = null;
      $('uuid-' + name).setAttribute('aria-invalid', 'true');
      $('uuid-' + name + '-status').className = 'literal-input-status error';
      $('uuid-' + name + '-status').textContent = 'The clipboard or drop could not be read. The parser was not called.';
      updateButtons(); return;
    }
    accept(index, source, `Exact UTF-16 source received by ${kind}, before insertion into the editor. The capture box was cleared to preserve CR and CRLF. Typing starts a new input. The source app or clipboard may have transformed it before delivery.`);
  }
  function showOutput(output) {
    const labels = { parsed: 'parsed · Parser succeeded', canonical_uuid: output.parsed ? 'canonical_uuid · Returned UUID' : 'canonical_uuid · Returned UUID on error',
      error: 'error · Original parser error', invalid_length: 'invalid_length · Length error' };
    const rows = protocol.fields.map(field => {
      const row = document.createElement('div'), label = document.createElement('dt'), value = document.createElement('dd');
      label.textContent = labels[field];
      value.textContent = typeof output[field] === 'boolean' ? (output[field] ? 'true' : 'false') :
        field === 'error' && output[field] === '' ? '"" (empty; no parser error)' : output[field];
      row.append(label, value); return row;
    });
    $('uuid-output').replaceChildren(...rows);
    $('uuid-output-wrap').hidden = false;
    $('uuid-result-note').hidden = false;
    $('uuid-result-note').className = 'uuid-result-note' + (output.parsed ? '' : ' error');
    $('uuid-result-note').textContent = output.parsed ? 'parsed: true — the original parser succeeded.' :
      'parsed: false — the parser returned an error. The returned UUID is preserved below, including any partial value; do not treat its canonical shape as success.';
    $('uuid-progress').textContent = 'Complete · one compiled Gooo ParseCanonical call' + (output.parsed ? '.' : '; parser error preserved.');
  }
  function revoke(url) {
    if (!downloads.has(url)) return;
    const timer = downloads.get(url);
    if (timer !== null) window.clearTimeout(timer);
    downloads.delete(url); window.URL.revokeObjectURL(url);
  }
  $('uuid-download').addEventListener('click', () => {
    if ($('uuid-download').disabled || !completed || job || !program || !completed.snapshots.every((value, index) => value === snapshots[index])) return;
    let url = null, link = null, requested = false;
    const status = $('uuid-download-status');
    status.className = 'literal-download-status';
    try {
      const report = protocol.observation(completed.snapshots, completed.output, completed.program, completed.origins);
      const blob = new Blob([JSON.stringify(report, null, 2) + '\n'], { type: 'application/json;charset=utf-8' });
      url = window.URL.createObjectURL(blob); downloads.set(url, null);
      link = document.createElement('a'); link.href = url; link.download = 'gooo-uuid-parse-observation.json'; link.hidden = true;
      document.body.append(link); link.click(); requested = true;
      status.textContent = 'Download requested. Check your browser’s downloads for gooo-uuid-parse-observation.json.';
    } catch {
      status.className = 'literal-download-status error'; status.textContent = 'The JSON download could not be started. Your results are still shown; try again.';
    } finally {
      if (link) link.remove();
      if (url !== null) {
        if (requested) downloads.set(url, window.setTimeout(() => revoke(url), 1000)); else revoke(url);
      }
      status.hidden = false;
    }
  });
  $('uuid-form').addEventListener('submit', event => {
    event.preventDefault();
    if (!worker || !program || job || snapshots.some(value => !value)) return;
    clearResult('Parsing with the compiled ParseCanonical activity…');
    const request = { id: ++nextId, inputs: snapshots.map(value => value.bound), program };
    const instance = worker;
    job = { request, snapshots: Object.freeze([...snapshots]), origins: Object.freeze([...origins]) };
    runTimer = window.setTimeout(() => {
      if (worker === instance && job && job.request.id === request.id) failed('Parsing timed out after 10 seconds. The worker was stopped; retry loading.');
    }, 10000);
    updateButtons();
    try { worker.postMessage({ type: 'parse', id: request.id, inputs: request.inputs }); }
    catch { failed('The inputs could not be sent to the parser worker. Retry loading.'); }
  });
  names.forEach((name, index) => {
    $('uuid-' + name).addEventListener('input', () => accept(index, $('uuid-' + name).value,
      'Source entered in the editor. Line endings follow browser editor behavior; use paste or JSON escapes to supply exact CR or CRLF.'));
    $('uuid-' + name).addEventListener('paste', event => transfer(index, event, 'paste'));
    $('uuid-' + name).addEventListener('drop', event => transfer(index, event, 'drop'));
    $('uuid-' + name).addEventListener('dragover', event => event.preventDefault());
    $('uuid-' + name + '-mode').addEventListener('change', () => {
      $('uuid-' + name).value = ''; updateMode(index);
      accept(index, '', 'Input format changed; enter a new source.');
    });
  });
  $('uuid-clear').addEventListener('click', () => {
    names.forEach((name, index) => { $('uuid-' + name).value = ''; accept(index, '', 'Cleared input. Empty text is accepted and returns a parser length error.'); });
  });
  $('uuid-cancel').addEventListener('click', () => {
    if (!job) return;
    loadWorker(); clearResult('Parsing cancelled. No result was retained.');
  });
  $('uuid-retry').addEventListener('click', () => { clearResult(); loadWorker(); });
  const presets = {
    canonical: ['text', '00112233-4455-6677-8899-AABBCCDDEEFF'], raw: ['text', '00112233445566778899aabbccddeeff'],
    wrapped: ['text', '[00112233-4455-6677-8899-aabbccddeeff]'],
    'raw-error': ['text', '00112233445566778899aabbccddee0g'],
    'standard-error': ['text', '00112233-4455-6677-8899-aabbccddee0g'],
    zero: ['text', '00000000-0000-0000-0000-000000000000'], surrogates: ['json', '"\\uFEFF\\uD800A\\r\\n"']
  };
  document.querySelectorAll('[data-uuid-preset]').forEach(button => button.addEventListener('click', () => {
    const preset = presets[button.dataset.uuidPreset];
    if (!preset) return;
    names.forEach((name, index) => {
      $('uuid-' + name + '-mode').value = preset[index * 2]; $('uuid-' + name).value = preset[index * 2 + 1]; updateMode(index);
      accept(index, preset[index * 2 + 1], 'Built-in example supplied explicitly; no clipboard delivery is claimed.');
    });
  }));
  window.addEventListener('pagehide', () => {
    clearTimer('load'); clearTimer('run');
    if (worker) worker.terminate();
    worker = null; program = null; job = null; completed = null;
    for (const url of [...downloads.keys()]) revoke(url);
    clearResult('Program stopped when this page left the active view.');
  });
  window.addEventListener('pageshow', event => {
    if (event.persisted && !worker) { clearResult(); loadWorker(); }
  });
  names.forEach((name, index) => { updateMode(index); accept(index, '', 'Initial empty input. Type, paste, drop plain text, or choose an example.'); });
  loadWorker();
})();
