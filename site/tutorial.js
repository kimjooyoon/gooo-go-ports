(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooTutorialProtocol;
  let worker, metadata, pending, archiveURL;
  let ready = false, failed = false, nextId = 0, loadTimer, runTimer;
  let sourceFiles = {};
  const loading = new AbortController();

  function clearResult(message = 'Your result will appear here.') {
    $('tutorial-values').hidden = true;
    $('tutorial-raw').hidden = true;
    $('tutorial-raw-output').textContent = '';
    $('tutorial-result-badge').textContent = 'NOT RUN';
    $('tutorial-result-status').textContent = message;
    $('tutorial-error').hidden = true;
    $('tutorial-error').textContent = '';
    protocol.fields.forEach(field => $(`result-${field.replace('_', '-')}`).textContent = '');
  }
  function inputError(error) {
    $('tutorial-input-status').textContent = error.message;
    $('tutorial-input-status').className = 'tutorial-help error';
    $((error.inputIndex === 0) ? 'tutorial-score' : 'tutorial-label').setAttribute('aria-invalid', 'true');
  }
  function currentInputs() {
    let label;
    try { label = protocol.labelFromEditor($('tutorial-label').value, $('tutorial-label-mode').value); }
    catch (error) { error.inputIndex = 1; throw error; }
    return protocol.validateInputs([$('tutorial-score').value, label]);
  }
  function updateInputs() {
    $('tutorial-score').setAttribute('aria-invalid', 'false');
    $('tutorial-label').setAttribute('aria-invalid', 'false');
    $('tutorial-label-warning').hidden = true;
    $('tutorial-label-warning').textContent = '';
    $('tutorial-label-preview').textContent = '';
    $('tutorial-label-bytes').textContent = '';
    $('tutorial-input-status').className = 'tutorial-help';
    let valid = false;
    try {
      const inputs = currentInputs();
      const bound = protocol.boundText(inputs[1]);
      $('tutorial-label-preview').textContent = JSON.stringify(bound);
      $('tutorial-label-bytes').textContent = `${new TextEncoder().encode(bound).length} / 1,024 UTF-8 bytes passed to Gooo.`;
      if (inputs[1] !== bound) {
        $('tutorial-label-warning').hidden = false;
        $('tutorial-label-warning').textContent = 'This label contains an unpaired UTF-16 surrogate. Native JSON decoding replaces it with U+FFFD (�). The label preview shows exactly what Gooo will receive.';
      }
      $('tutorial-input-status').textContent = 'Ready for your next run. Inputs stay in this browser.';
      valid = true;
    } catch (error) { inputError(error); }
    $('tutorial-run').disabled = !ready || failed || !valid || Boolean(pending);
    $('tutorial-download-inputs').disabled = !ready || failed || !valid;
    return valid;
  }
  function edited() {
    pending = null;
    window.clearTimeout(runTimer);
    clearResult('Inputs changed. Run again to see their result.');
    $('tutorial-download-status').textContent = '';
    updateInputs();
  }
  function fatal(message) {
    if (failed) return;
    failed = true; ready = false; pending = null;
    loading.abort();
    window.clearTimeout(loadTimer); window.clearTimeout(runTimer);
    worker?.terminate();
    clearResult('The tutorial program is unavailable.');
    $('tutorial-runtime').className = 'tutorial-runtime error';
    $('tutorial-runtime').textContent = message;
    $('tutorial-error').textContent = message;
    $('tutorial-error').hidden = false;
    $('tutorial-retry').hidden = false;
    $('tutorial-run').disabled = true;
    $('tutorial-download-inputs').disabled = true;
    $('tutorial-download').hidden = true;
    $('tutorial-program').textContent = 'Build identity is unavailable.';
    $('tutorial-program-hash').textContent = '';
    $('tutorial-source').textContent = 'Source could not be verified for this build. Use the native instructions or retry loading.';
    $('tutorial-source-status').textContent = '';
    sourceFiles = {};
    if (archiveURL) { URL.revokeObjectURL(archiveURL); archiveURL = null; }
  }
  async function checkedFile(path, expected, expectedSize) {
    const response = await fetch(path, { cache: 'no-store', signal: loading.signal });
    if (!response.ok) throw new Error(`A tutorial file could not be loaded (HTTP ${response.status}). Retry loading.`);
    const bytes = await response.arrayBuffer();
    if (bytes.byteLength > 1024 * 1024 || (expectedSize !== undefined && bytes.byteLength !== expectedSize)) throw new Error('A tutorial file has an unexpected size.');
    const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes);
    const hash = Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, '0')).join('');
    if (hash !== expected) throw new Error('A tutorial source or download does not match this build. Retry loading.');
    return bytes;
  }
  function showSource() {
    const name = $('tutorial-source-file').value;
    $('tutorial-source').textContent = sourceFiles[name] || 'Choose a verified source file.';
  }
  async function receiveReady(message) {
    if (metadata || !protocol.exact(message, ['type', 'metadata'])) throw new Error('The program sent invalid build information.');
    metadata = protocol.validateMetadata(message.metadata);
    const files = await Promise.all(protocol.files.map(async name => {
      const bytes = await checkedFile(`./tutorial/source/${name}`, metadata.sample_sha256[name]);
      return [name, new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(bytes)];
    }));
    const zip = await checkedFile('./tutorial/source.zip', metadata.source_zip_sha256, metadata.source_zip_bytes);
    if (failed) return;
    sourceFiles = Object.fromEntries(files);
    archiveURL = URL.createObjectURL(new Blob([zip], { type: 'application/zip' }));
    $('tutorial-download').href = archiveURL;
    $('tutorial-download').hidden = false;
    showSource();
    $('tutorial-source-status').textContent = 'All six source files and the project download match this build’s hashes.';
    $('tutorial-program').textContent = `Source ${metadata.source_revision}${metadata.source_dirty ? ' + local changes' : ''} · compiler ${metadata.compiler_revision} · ${metadata.go_version}`;
    $('tutorial-program-hash').textContent = `WebAssembly SHA-256 ${metadata.wasm_sha256}`;
    ready = true;
    window.clearTimeout(loadTimer);
    $('tutorial-runtime').textContent = 'Ready · the compiled Gooo program runs in this browser';
    updateInputs();
  }
  function receiveResult(message) {
    if (!pending || message.id !== pending.id) return; // Edits supersede earlier work.
    const result = protocol.validateResult(message, pending, metadata);
    pending = null; window.clearTimeout(runTimer);
    protocol.fields.forEach(field => {
      $(`result-${field.replace('_', '-')}`).textContent = field === 'label' ? JSON.stringify(result.output[field]) : String(result.output[field]);
    });
    $('tutorial-values').hidden = false;
    $('tutorial-result-badge').textContent = 'EXECUTED';
    $('tutorial-result-status').textContent = 'Returned by the Gooo tutorial program for your submitted inputs.';
    $('tutorial-raw-output').textContent = JSON.stringify(result, null, 2);
    $('tutorial-raw').hidden = false;
    updateInputs();
    $('tutorial-input-status').textContent = 'Executed using the exact score and the displayed bound label.';
  }
  function receiveError(message) {
    const keys = message.fatal === true ? ['type', 'fatal', 'message'] : ['type', 'fatal', 'message', 'id'];
    if (!protocol.exact(message, keys) || typeof message.message !== 'string' || !message.message || message.message.length > 500 || typeof message.fatal !== 'boolean') {
      throw new Error('The program sent an unreadable error.');
    }
    if (message.fatal) { fatal(message.message); return; }
    if (!pending || message.id !== pending.id) return;
    pending = null; window.clearTimeout(runTimer);
    clearResult('No result was accepted.');
    $('tutorial-error').textContent = message.message;
    $('tutorial-error').hidden = false;
    updateInputs();
  }

  $('tutorial-retry').addEventListener('click', () => window.location.reload());
  ['tutorial-score', 'tutorial-label'].forEach(id => $(id).addEventListener('input', () => { $('tutorial-preset').value = ''; edited(); }));
  $('tutorial-label-mode').addEventListener('change', () => { $('tutorial-preset').value = ''; edited(); });
  $('tutorial-preset').addEventListener('change', () => {
    if (!['-5', '42', '75', '105'].includes($('tutorial-preset').value)) return;
    $('tutorial-score').value = $('tutorial-preset').value;
    $('tutorial-label').value = 'sensor'; $('tutorial-label-mode').value = 'text';
    edited();
  });
  $('tutorial-source-file').addEventListener('change', showSource);
  $('tutorial-form').addEventListener('submit', event => {
    event.preventDefault();
    if (pending) return;
    clearResult();
    if (!updateInputs() || !ready || failed) return;
    pending = { type: 'run', id: ++nextId, inputs: currentInputs() };
    $('tutorial-run').disabled = true;
    $('tutorial-result-badge').textContent = 'RUNNING';
    $('tutorial-result-status').textContent = 'Running the compiled program…';
    runTimer = window.setTimeout(() => fatal('Execution timed out. Retry loading before running again.'), 10000);
    try { worker.postMessage(pending); } catch { fatal('The program could not receive your input. Retry loading.'); }
  });
  $('tutorial-download-inputs').addEventListener('click', () => {
    if (!ready || failed || !updateInputs()) return;
    let url, anchor;
    try {
      url = URL.createObjectURL(new Blob([protocol.inputDocument(currentInputs())], { type: 'application/json;charset=utf-8' }));
      anchor = document.createElement('a'); anchor.href = url; anchor.download = 'inputs.json'; anchor.hidden = true;
      document.body.append(anchor); anchor.click();
      $('tutorial-download-status').textContent = 'Download requested. Use this inputs.json with the original project, including its unchanged sources.';
    } catch { $('tutorial-download-status').textContent = 'The input download could not start. Check browser download permissions and try again.'; }
    finally { anchor?.remove(); if (url) window.setTimeout(() => URL.revokeObjectURL(url), 1000); }
  });
  window.addEventListener('pagehide', event => {
    // A cached page resumes this runtime, its timers and its verified download.
    if (event.persisted) return;
    worker?.terminate(); loading.abort(); if (archiveURL) URL.revokeObjectURL(archiveURL);
  });
  updateInputs();
  try {
    if (!globalThis.crypto?.subtle || typeof Worker !== 'function') throw new Error('Use HTTPS or localhost in a browser with Web Workers and WebAssembly support.');
    worker = new Worker('./tutorial-worker.js');
    loadTimer = window.setTimeout(() => fatal('Loading timed out after 30 seconds. Retry loading.'), 30000);
    worker.onmessage = event => {
      if (failed) return;
      const message = event.data;
      try {
        if (message?.type === 'ready') receiveReady(message).catch(error => fatal(error.message));
        else if (message?.type === 'result') receiveResult(message);
        else if (message?.type === 'error') receiveError(message);
        else throw new Error('The program sent an unsupported message.');
      } catch (error) { fatal(error.message); }
    };
    worker.onerror = () => fatal('The program stopped unexpectedly. Retry loading.');
    worker.onmessageerror = () => fatal('The program response could not be read. Retry loading.');
  } catch (error) { fatal(error.message); }
})();
