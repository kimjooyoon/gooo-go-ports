(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooLiteralSearchProtocol;
  const names = ['text', 'needle'];
  let snapshots = [null, null], origins = ['', ''];
  let worker = null, program = null, job = null, completed = null, nextId = 0;
  let loadTimer = null, runTimer = null;
  const downloads = new Map();
  function clearTimer(name) {
    const value = name === 'load' ? loadTimer : runTimer;
    if (value !== null) window.clearTimeout(value);
    if (name === 'load') loadTimer = null; else runTimer = null;
  }
  function updateButtons() {
    $('literal-run').disabled = !program || snapshots.some(value => !value) || !!job;
    $('literal-run').textContent = job ? 'Searching…' : 'Search text';
    $('literal-cancel').hidden = !job;
    const available = !!completed && !!program && !job && completed.snapshots.every((value, index) => value === snapshots[index]);
    $('literal-download-controls').hidden = !available;
    $('literal-download').disabled = !available;
  }
  function clearResult(status = 'No search has run.') {
    completed = null;
    $('literal-output').replaceChildren();
    $('literal-output-wrap').hidden = true;
    $('literal-result-error').hidden = true;
    $('literal-result-error').textContent = '';
    $('literal-progress').textContent = status;
    $('literal-download-status').hidden = true;
    $('literal-download-status').textContent = '';
    updateButtons();
  }
  function failed(message) {
    clearTimer('load'); clearTimer('run');
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    clearResult('Search unavailable.');
    $('literal-runtime').textContent = message;
    $('literal-runtime').className = 'literal-runtime error';
    $('literal-result-error').textContent = message;
    $('literal-result-error').hidden = false;
    $('literal-program').textContent = 'Program identity is unavailable.';
    $('literal-retry').hidden = false;
    updateButtons();
  }
  function loadWorker() {
    clearTimer('load'); clearTimer('run');
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    $('literal-runtime').textContent = 'Loading the compiled search program…';
    $('literal-runtime').className = 'literal-runtime';
    $('literal-program').textContent = 'Waiting for the verified program manifest.';
    $('literal-retry').hidden = true;
    updateButtons();
    try {
      const instance = new Worker('./literal-search-worker.js');
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
            $('literal-runtime').textContent = 'WebAssembly ready · search inputs stay in this browser';
            $('literal-program').textContent = `WebAssembly SHA-256 ${program.wasm_sha256} · source revision ${program.source_revision}` +
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
        failed('The search worker stopped. Retry loading to search the current inputs.');
      };
      instance.onmessageerror = () => { if (worker === instance) failed('The search worker returned an unreadable message.'); };
    } catch { failed('The search worker could not start. Retry loading in a browser with Worker support.'); }
  }
  function invalidate() {
    if (job) loadWorker();
    clearResult();
  }
  function accept(index, source, origin) {
    invalidate();
    const name = names[index];
    snapshots[index] = null; origins[index] = origin;
    $('literal-' + name + '-source').textContent = origin;
    $('literal-' + name).setAttribute('aria-invalid', 'false');
    const status = $('literal-' + name + '-status');
    status.className = 'literal-input-status';
    try {
      const snapshot = protocol.read(source, $('literal-' + name + '-mode').value);
      snapshots[index] = snapshot;
      $('literal-' + name + '-preview').textContent = snapshot.originalEscaped;
      $('literal-' + name + '-bound').textContent = snapshot.boundEscaped;
      $('literal-' + name + '-bound-wrap').hidden = !snapshot.changed;
      status.textContent = `${snapshot.originalUtf16Units.length} original UTF-16 code units · ${snapshot.utf8Bytes} bound UTF-8 bytes. ` +
        (snapshot.changed ? 'Unpaired UTF-16 surrogates were replaced with U+FFFD; the bound input is shown below.' : 'Original and bound input are identical.');
    } catch (error) {
      $('literal-' + name).setAttribute('aria-invalid', 'true');
      $('literal-' + name + '-preview').textContent = 'Input was not accepted. Nothing was searched.';
      $('literal-' + name + '-bound').textContent = '';
      $('literal-' + name + '-bound-wrap').hidden = true;
      status.className = 'literal-input-status error'; status.textContent = error.message;
    }
    updateButtons();
  }
  function updateMode(index) {
    const name = names[index], json = $('literal-' + name + '-mode').value === 'json';
    $('literal-' + name + '-label').textContent = (index ? 'Needle' : 'Text') + (json ? ' — one JSON string' : ' — type or paste');
    $('literal-' + name).placeholder = json ? '"Use \\r\\n or \\uD800 escapes"' : (index ? 'Literal text to find; empty is valid' : 'Text to search');
  }
  function transfer(index, event, kind) {
    event.preventDefault();
    const data = kind === 'paste' ? event.clipboardData : event.dataTransfer;
    const name = names[index];
    $('literal-' + name).value = '';
    if (!data || typeof data.getData !== 'function' || data.files?.length ||
        (data.types?.length && !Array.from(data.types).includes('text/plain'))) {
      accept(index, '', 'No plain text was received.');
      snapshots[index] = null;
      $('literal-' + name).setAttribute('aria-invalid', 'true');
      $('literal-' + name + '-status').className = 'literal-input-status error';
      $('literal-' + name + '-status').textContent = 'Paste or drop plain text. Files and other clipboard formats are not read.';
      updateButtons(); return;
    }
    let source;
    try { source = data.getData('text/plain'); }
    catch {
      accept(index, '', 'The browser did not provide plain text.'); snapshots[index] = null;
      $('literal-' + name).setAttribute('aria-invalid', 'true');
      $('literal-' + name + '-status').className = 'literal-input-status error';
      $('literal-' + name + '-status').textContent = 'The clipboard or drop could not be read. Nothing was searched.';
      updateButtons(); return;
    }
    accept(index, source, `Exact UTF-16 source received by ${kind}, before insertion into the editor. The capture box was cleared to preserve CR and CRLF. Typing starts a new input. The source app or clipboard may have transformed it before delivery.`);
  }
  function showOutput(output) {
    const labels = { text: 'Bound text', needle: 'Bound needle', first_byte_offset: 'First UTF-8 byte offset', found: 'Found',
      non_overlapping_count: 'Non-overlapping count', text_byte_length: 'Text UTF-8 bytes', needle_byte_length: 'Needle UTF-8 bytes' };
    const rows = protocol.fields.map(field => {
      const row = document.createElement('div'), label = document.createElement('dt'), value = document.createElement('dd');
      label.textContent = labels[field];
      value.textContent = field === 'text' || field === 'needle' ? protocol.read(output[field], 'text').originalEscaped :
        field === 'found' ? (output[field] ? 'Yes' : 'No') : output[field];
      row.append(label, value); return row;
    });
    $('literal-output').replaceChildren(...rows);
    $('literal-output-wrap').hidden = false;
    $('literal-progress').textContent = 'Complete · one compiled Gooo Report call.';
  }
  function revoke(url) {
    if (!downloads.has(url)) return;
    const timer = downloads.get(url);
    if (timer !== null) window.clearTimeout(timer);
    downloads.delete(url); window.URL.revokeObjectURL(url);
  }
  $('literal-download').addEventListener('click', () => {
    if ($('literal-download').disabled || !completed || job || !program || !completed.snapshots.every((value, index) => value === snapshots[index])) return;
    let url = null, link = null, requested = false;
    const status = $('literal-download-status');
    status.className = 'literal-download-status';
    try {
      const report = protocol.observation(completed.snapshots, completed.output, completed.program, completed.origins);
      const blob = new Blob([JSON.stringify(report, null, 2) + '\n'], { type: 'application/json;charset=utf-8' });
      url = window.URL.createObjectURL(blob); downloads.set(url, null);
      link = document.createElement('a'); link.href = url; link.download = 'gooo-literal-search-observation.json'; link.hidden = true;
      document.body.append(link); link.click(); requested = true;
      status.textContent = 'Download requested. Check your browser’s downloads for gooo-literal-search-observation.json.';
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
  $('literal-form').addEventListener('submit', event => {
    event.preventDefault();
    if (!worker || !program || job || snapshots.some(value => !value)) return;
    clearResult('Searching with the compiled Report activity…');
    const request = { id: ++nextId, inputs: snapshots.map(value => value.bound), program };
    const instance = worker;
    job = { request, snapshots: Object.freeze([...snapshots]), origins: Object.freeze([...origins]) };
    runTimer = window.setTimeout(() => {
      if (worker === instance && job && job.request.id === request.id) failed('Search timed out after 10 seconds. The worker was stopped; retry loading.');
    }, 10000);
    updateButtons();
    try { worker.postMessage({ type: 'search', id: request.id, inputs: request.inputs }); }
    catch { failed('The inputs could not be sent to the search worker. Retry loading.'); }
  });
  names.forEach((name, index) => {
    $('literal-' + name).addEventListener('input', () => accept(index, $('literal-' + name).value,
      'Source entered in the editor. Line endings follow browser editor behavior; use paste or JSON escapes to supply exact CR or CRLF.'));
    $('literal-' + name).addEventListener('paste', event => transfer(index, event, 'paste'));
    $('literal-' + name).addEventListener('drop', event => transfer(index, event, 'drop'));
    $('literal-' + name).addEventListener('dragover', event => event.preventDefault());
    $('literal-' + name + '-mode').addEventListener('change', () => {
      $('literal-' + name).value = ''; updateMode(index);
      accept(index, '', 'Input format changed; enter a new source.');
    });
  });
  $('literal-clear').addEventListener('click', () => {
    names.forEach((name, index) => { $('literal-' + name).value = ''; accept(index, '', 'Cleared input. Empty text is valid in Text mode.'); });
  });
  $('literal-cancel').addEventListener('click', () => {
    if (!job) return;
    loadWorker(); clearResult('Search cancelled. No result was retained.');
  });
  $('literal-retry').addEventListener('click', () => { clearResult(); loadWorker(); });
  const presets = {
    bytes: ['text', '가x가', 'text', 'x'], overlap: ['text', 'aaaa', 'text', 'aa'],
    empty: ['text', 'e\u0301😀', 'text', ''], surrogates: ['json', '"\\uFEFF\\uD800A\\r\\n"', 'json', '"\\uFFFD"']
  };
  document.querySelectorAll('[data-literal-preset]').forEach(button => button.addEventListener('click', () => {
    const preset = presets[button.dataset.literalPreset];
    if (!preset) return;
    names.forEach((name, index) => {
      $('literal-' + name + '-mode').value = preset[index * 2]; $('literal-' + name).value = ''; updateMode(index);
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
