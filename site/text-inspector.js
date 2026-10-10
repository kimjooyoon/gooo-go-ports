(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const input = window.GoooTextInput;
  const protocol = window.GoooTextInspectorProtocol;
  const reports = window.GoooTextInspectionReport;
  let worker = null;
  let binding = null;
  let decoded = null;
  let job = null;
  let nextId = 0;
  let loadTimeout = null;
  let completedInspection = null;
  const downloadUrls = new Map();

  function clearLoadTimeout() {
    if (loadTimeout !== null) window.clearTimeout(loadTimeout);
    loadTimeout = null;
  }

  function updateButtons() {
    $('text-run').disabled = !binding || !decoded || !!job;
    $('text-cancel').hidden = !job;
    $('text-run').firstChild.textContent = job ? 'Inspecting… ' : 'Inspect text ';
    const canDownload = !!completedInspection && !job && !!binding && completedInspection.snapshot === decoded;
    $('text-download-controls').hidden = !canDownload;
    $('text-download').hidden = !canDownload;
    $('text-download').disabled = !canDownload;
  }

  function clearResults(message = 'Not inspected.') {
    completedInspection = null;
    $('text-download-controls').hidden = true;
    $('text-download').hidden = true;
    $('text-download').disabled = true;
    $('text-download-status').hidden = true;
    $('text-download-status').textContent = '';
    $('text-download-status').className = 'text-download-status';
    $('text-rows').replaceChildren();
    $('text-table-wrap').hidden = true;
    $('text-summary').textContent = '';
    $('text-summary').hidden = true;
    $('text-result-error').textContent = '';
    $('text-result-error').hidden = true;
    $('text-progress').textContent = message;
    $('text-progress-bar').hidden = true;
    $('text-progress-bar').value = 0;
  }

  function invalidateResults() {
    if (job) {
      job = null;
      // Termination cancels both the running token and any queued callbacks. A
      // fresh worker also prevents a cancelled job from blocking a new input.
      loadWorker();
    }
    clearResults();
  }

  function runtimeFailed(message) {
    clearLoadTimeout();
    if (worker) worker.terminate();
    worker = null;
    binding = null;
    job = null;
    clearResults('Inspection unavailable.');
    $('text-result-error').textContent = message;
    $('text-result-error').hidden = false;
    $('text-runtime').textContent = message;
    $('text-runtime').className = 'text-runtime error';
    $('text-program').textContent = 'Program identity is unavailable.';
    $('text-retry').hidden = false;
    updateButtons();
  }

  function loadWorker() {
    clearLoadTimeout();
    if (worker) worker.terminate();
    worker = null;
    binding = null;
    $('text-runtime').textContent = 'Loading the WebAssembly worker…';
    $('text-runtime').className = 'text-runtime';
    $('text-program').textContent = 'Waiting for verified program identity.';
    $('text-retry').hidden = true;
    updateButtons();
    try {
      const instance = new Worker('./text-inspector-worker.js');
      worker = instance;
      loadTimeout = window.setTimeout(() => {
        if (worker === instance && !binding) runtimeFailed('Loading the WebAssembly worker timed out after 30 seconds. Retry loading.');
      }, 30000);
      instance.onmessage = event => {
        if (worker !== instance) return;
        const message = event.data;
        try {
          if (!message || typeof message !== 'object') throw new Error('The worker returned an invalid message.');
          if (message.type === 'ready') {
            if (binding) throw new Error('The worker sent a duplicate ready message.');
            binding = protocol.validateReady(message);
            clearLoadTimeout();
            $('text-runtime').textContent = 'WebAssembly ready · text stays in this browser';
            $('text-program').textContent = `WebAssembly SHA-256 ${binding.wasmSha256} · source revision ${binding.sourceRevision}`;
            updateButtons();
          } else if (message.type === 'progress' || message.type === 'result') {
            if (!job || message.id !== job.request.id) return;
            if (message.type === 'progress') {
              const completed = protocol.validateProgress(message, { id: job.request.id, total: job.request.points.length });
              if (completed < job.completed) throw new Error('The worker returned progress out of order.');
              job.completed = completed;
              $('text-progress').textContent = `Inspecting ${completed} of ${job.request.points.length} items…`;
              $('text-progress-bar').value = completed;
            } else {
              const rows = protocol.validateResult(message, job.request);
              const snapshot = job.snapshot;
              const program = Object.freeze({ wasmSha256: job.request.wasmSha256, sourceRevision: job.request.sourceRevision });
              job = null;
              showResults(snapshot, rows);
              completedInspection = Object.freeze({ snapshot, rows, binding: program });
              updateButtons();
            }
          } else if (message.type === 'error') {
            if (message.id !== undefined && (!job || message.id !== job.request.id)) return;
            const error = protocol.validateError(message, message.id === undefined ? undefined : job.request.id);
            runtimeFailed(error);
          } else throw new Error('The worker returned an unknown message type.');
        } catch (error) { runtimeFailed(error.message); }
      };
      instance.onerror = event => {
        if (worker !== instance) return;
        event.preventDefault?.();
        runtimeFailed('The inspection worker stopped. Retry loading to inspect the current input.');
      };
      instance.onmessageerror = () => {
        if (worker === instance) runtimeFailed('The inspection worker returned an unreadable message.');
      };
    } catch {
      runtimeFailed('The WebAssembly worker could not start. Retry loading in a browser with worker support.');
    }
  }

  function setInputError(message) {
    decoded = null;
    $('text-preview').textContent = 'Input was not accepted. Nothing was inspected.';
    $('text-capture').setAttribute('aria-invalid', 'true');
    $('text-input-status').className = 'text-input-status error';
    $('text-input-status').textContent = message;
    updateButtons();
  }

  function acceptInput(source, origin) {
    invalidateResults();
    decoded = null;
    $('text-capture').setAttribute('aria-invalid', 'false');
    $('text-input-status').className = 'text-input-status';
    $('text-source').textContent = origin;
    try {
      decoded = input.read(source, $('text-mode').value);
      $('text-preview').textContent = decoded.escaped;
      $('text-input-status').textContent = `${decoded.items.length} decoded items · ${decoded.codeUnits} received UTF-16 code units. ` +
        (decoded.items.length ? 'Ready to inspect the complete input.' : 'Empty text is valid; it needs no Gooo calls.');
    } catch (error) { setInputError(error.message); }
    updateButtons();
  }

  function updateMode() {
    const json = $('text-mode').value === 'json';
    $('text-capture-label').textContent = json ? 'Type or paste one JSON string' : 'Type or paste text';
    $('text-capture').placeholder = json ? '"A\\r\\n\\uD800"' : 'Paste text here, or try an example below';
    $('text-json-limit').hidden = !json;
  }

  function resetInput() {
    invalidateResults();
    decoded = null;
    $('text-capture').value = '';
    $('text-capture').setAttribute('aria-invalid', 'false');
    $('text-preview').textContent = 'No input yet.';
    $('text-input-status').className = 'text-input-status';
    $('text-input-status').textContent = $('text-mode').value === 'json' ? 'Enter one JSON string, including its double quotes.' : 'Enter text or choose an example.';
    $('text-source').textContent = 'This view reads the text delivered by your browser. A source app or clipboard may have transformed it before delivery.';
    updateMode();
    updateButtons();
  }

  function receiveTransfer(event, kind) {
    event.preventDefault();
    const transfer = kind === 'paste' ? event.clipboardData : event.dataTransfer;
    $('text-capture').value = '';
    if (!transfer || typeof transfer.getData !== 'function' || transfer.files?.length ||
        (transfer.types?.length && !Array.from(transfer.types).includes('text/plain'))) {
      invalidateResults();
      $('text-source').textContent = 'No plain text was received.';
      setInputError('Paste or drop plain text. Files and other clipboard formats are not read.');
      return;
    }
    let source;
    try { source = transfer.getData('text/plain'); }
    catch {
      invalidateResults();
      $('text-source').textContent = 'The browser did not provide plain text.';
      setInputError('The clipboard or drop could not be read. Nothing was inspected; try again.');
      return;
    }
    const json = $('text-mode').value === 'json';
    acceptInput(source, json
      ? `JSON string received by ${kind}; its escapes were decoded. The capture box was cleared. Typing starts a new input.`
      : `Exact UTF-16 text received by ${kind}, before insertion into the editor. The capture box was cleared to preserve CR and CRLF. Typing starts a new input. The source app or clipboard may have transformed text before delivery.`);
  }

  function cell(tag, label, value, detail = '', invalid = false) {
    const td = document.createElement(tag);
    td.setAttribute('data-label', label);
    if (tag === 'th') td.setAttribute('scope', 'row');
    const content = document.createElement('span');
    content.className = 'text-cell-value' + (invalid ? ' text-invalid' : '');
    content.textContent = value;
    if (detail) {
      const small = document.createElement('span');
      small.className = 'text-cell-detail';
      small.textContent = detail;
      content.append(small);
    }
    td.append(content);
    return td;
  }

  function showResults(snapshot, rows) {
    const rendered = [];
    let spaces = 0;
    let allScalar = true;
    let bytes = 0;
    let units = 0;
    rows.forEach((row, index) => {
      const item = snapshot.items[index];
      const tr = document.createElement('tr');
      tr.append(cell('th', 'Item / UTF-16 offset', `#${index + 1}`, `offset ${item.offset}`),
        cell('td', 'Received code units', item.units, `${item.width} received unit${item.width === 1 ? '' : 's'}`),
        cell('td', 'Code point', item.notation, `decimal ${item.decimal}`),
        cell('td', 'Valid scalar', row.validScalar ? 'Yes' : 'No', '', !row.validScalar),
        cell('td', 'Go White_Space', row.goSpace ? 'Yes' : 'No'),
        cell('td', 'Go letter (L)', row.goLetter ? 'Yes' : 'No'),
        cell('td', 'Go decimal digit (Nd)', row.goDigit ? 'Yes' : 'No'),
        cell('td', 'UTF-8 bytes', row.utf8Bytes, row.utf8Bytes === '-1' ? 'Cannot encode' : ''),
        cell('td', 'UTF-16 encoded units', row.utf16Units, row.utf16Units === '-1' ? 'Cannot encode' : ''));
      rendered.push(tr);
      if (row.goSpace) spaces++;
      if (!row.validScalar || row.utf8Bytes === '-1' || row.utf16Units === '-1') allScalar = false;
      // These bounded sums aggregate validated Gooo results, never host encoders.
      bytes += Number(row.utf8Bytes);
      units += Number(row.utf16Units);
    });
    $('text-rows').replaceChildren(...rendered);
    $('text-table-wrap').hidden = rows.length === 0;
    $('text-summary').textContent = `${rows.length} decoded items · ${snapshot.codeUnits} received UTF-16 code units · ${spaces} Go whitespace items. ` +
      (allScalar ? `Encoded totals: ${bytes} UTF-8 bytes and ${units} UTF-16 units.` : 'Encoded totals are unavailable: at least one returned scalar or encoding result is invalid. Received code-unit counts remain exact.');
    $('text-summary').hidden = false;
    $('text-progress').textContent = rows.length ? `Complete · ${rows.length} items · ${rows.length * protocol.OPERATIONS.length} Gooo calls.` : 'Complete · empty input · no Gooo calls needed.';
    $('text-progress-bar').hidden = true;
    $('text-result-error').hidden = true;
  }

  function revokeDownload(url) {
    if (!downloadUrls.has(url)) return;
    const timer = downloadUrls.get(url);
    if (timer !== null) window.clearTimeout(timer);
    downloadUrls.delete(url);
    window.URL.revokeObjectURL(url);
  }

  $('text-download').addEventListener('click', () => {
    if ($('text-download').disabled || !completedInspection || job || !binding || completedInspection.snapshot !== decoded) return;
    const completed = completedInspection;
    let url = null;
    let link = null;
    let requested = false;
    $('text-download-status').textContent = '';
    $('text-download-status').hidden = true;
    $('text-download-status').className = 'text-download-status';
    try {
      const report = reports.create(completed.snapshot, completed.rows, completed.binding);
      const blob = new Blob([reports.serialize(report)], { type: 'application/json;charset=utf-8' });
      url = window.URL.createObjectURL(blob);
      downloadUrls.set(url, null);
      link = document.createElement('a');
      link.href = url;
      link.download = 'gooo-text-inspection.json';
      link.hidden = true;
      document.body.append(link);
      link.click();
      requested = true;
      $('text-download-status').textContent = 'Download requested. Check your browser’s downloads for gooo-text-inspection.json.';
    } catch {
      $('text-download-status').className = 'text-download-status error';
      $('text-download-status').textContent = 'The JSON download could not be started. Your inspection results are still shown. Try again.';
    } finally {
      if (link) link.remove();
      if (url !== null) {
        // Leave a successful object URL alive for the browser to start reading
        // it. Failed starts release immediately; pagehide releases any remainder.
        if (requested) downloadUrls.set(url, window.setTimeout(() => revokeDownload(url), 1000));
        else revokeDownload(url);
      }
      $('text-download-status').hidden = false;
    }
  });
  window.addEventListener('pagehide', () => {
    for (const url of [...downloadUrls.keys()]) revokeDownload(url);
  });

  $('text-form').addEventListener('submit', event => {
    event.preventDefault();
    if (!worker || !binding || !decoded || job) return;
    clearResults('Starting inspection…');
    const request = { id: ++nextId, points: decoded.items.map(item => item.decimal), ...binding };
    job = { request, snapshot: decoded, completed: 0 };
    $('text-progress-bar').max = Math.max(1, request.points.length);
    $('text-progress-bar').hidden = request.points.length === 0;
    updateButtons();
    try { worker.postMessage({ type: 'inspect', id: request.id, points: request.points }); }
    catch { runtimeFailed('The input could not be sent to the local worker. Retry loading.'); }
  });
  $('text-capture').addEventListener('input', () => acceptInput($('text-capture').value,
    $('text-mode').value === 'json' ? 'JSON entered in the editor; escape sequences are decoded explicitly.' : 'Text entered in the editor. Line endings follow the browser’s editor behavior; use paste or JSON escapes to supply an exact CR or CRLF sequence.'));
  $('text-capture').addEventListener('paste', event => receiveTransfer(event, 'paste'));
  $('text-capture').addEventListener('drop', event => receiveTransfer(event, 'drop'));
  $('text-capture').addEventListener('dragover', event => event.preventDefault());
  $('text-mode').addEventListener('change', resetInput);
  $('text-clear').addEventListener('click', resetInput);
  $('text-cancel').addEventListener('click', () => {
    if (!job) return;
    invalidateResults();
    $('text-progress').textContent = 'Inspection cancelled. No results retained.';
    updateButtons();
  });
  $('text-retry').addEventListener('click', () => { job = null; clearResults(); loadWorker(); });
  document.querySelectorAll('[data-text-preset]').forEach(button => button.addEventListener('click', () => {
    const preset = input.presets[button.dataset.textPreset];
    if (!preset) return;
    $('text-mode').value = preset.mode;
    $('text-capture').value = '';
    updateMode();
    acceptInput(preset.source, `Built-in example: ${preset.label}. This is an explicit example, not a claim about clipboard delivery.`);
  }));
  updateMode();
  loadWorker();
})();
