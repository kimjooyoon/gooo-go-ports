(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooCropPlannerProtocol;
  const states = Object.fromEntries(Object.keys(protocol.modes).map(mode => [mode,
    { status: 'idle', metadata: null, error: '', load: null }]));
  let selected = 'ordered';
  $('crop-mode').value = selected;

  function clearResult(message = 'Your exact result will appear here.') {
    $('crop-result-values').hidden = true;
    $('crop-raw-result').hidden = true;
    $('crop-raw-output').textContent = '';
    $('crop-error').hidden = true;
    $('crop-error').textContent = '';
    $('crop-result-badge').textContent = 'NOT RUN';
    $('crop-result-status').textContent = message;
    protocol.fields.forEach(field => $(`result-${field.replace('_', '-')}`).textContent = '');
    $('result-has-pixels').textContent = '';
  }

  function edited(message = 'Coordinates changed. Plan the crop to see a new result.') {
    clearResult(message);
    protocol.inputIds.forEach(id => $(id).setAttribute('aria-invalid', 'false'));
    $('crop-input-status').className = 'crop-input-status';
    $('crop-input-status').textContent = selected === 'corners'
      ? 'Both corners are sent exactly as entered. Gooo orders only the selection; bounds remain unchanged.'
      : 'Coordinates are kept exactly as entered; neither rectangle is reordered.';
  }

  function showLinks(mode, metadata) {
    const config = protocol.modeFor(mode);
    const sourceBase = !metadata || metadata.source_dirty ? config.assetBase + 'source/'
      : `https://github.com/kimjooyoon/gooo-go-ports/blob/${metadata.source_revision}/`;
    $('crop-source-link').href = sourceBase + config.source;
    $('crop-source-link').textContent = `${config.activity} source`;
    $('crop-port-link').href = sourceBase + config.portSource;
    $('crop-port-link').textContent = mode === 'corners' ? 'RectangleCanon port source' : 'Rectangle port source';
    for (const [id, filename] of Object.entries({ generated: 'generated.go', metadata: 'metadata.json',
      execution: 'execution.json', replay: 'replay.json', fixtures: 'fixtures.json' })) {
      $(`crop-${id}-link`).href = config.assetBase + filename;
    }
    // The local source snapshot is available even when this candidate is unshipped.
    $('crop-native-guide').href = sourceBase + (mode === 'corners' ? 'docs/drag-crop.md' : 'examples/crop-planner/README.md');
  }

  function paintRuntime() {
    const state = states[selected];
    const config = protocol.modeFor(selected);
    $('crop-run').disabled = state.status !== 'ready';
    $('crop-retry').hidden = state.status !== 'failed';
    $('crop-runtime').className = state.status === 'failed' ? 'crop-runtime error' : 'crop-runtime';
    showLinks(selected, state.metadata);
    if (state.status === 'failed') {
      clearResult('Crop planning is unavailable in this mode.');
      $('crop-runtime').textContent = state.error;
      $('crop-error').textContent = state.error;
      $('crop-error').hidden = false;
    } else {
      $('crop-runtime').textContent = state.status === 'ready'
        ? `${config.label} ready · coordinates stay in this browser`
        : `Loading ${config.label.toLowerCase()}…`;
    }
    if (state.status === 'ready') {
      const metadata = state.metadata;
      $('crop-program').textContent = `${config.activity} · Source ${metadata.source_revision}${metadata.source_dirty ? ' + local changes' : ''} · compiler ${metadata.compiler_revision} · ${metadata.go_version}`;
      $('crop-program-hash').textContent = `WebAssembly SHA-256 ${metadata.wasm_sha256}`;
    } else {
      $('crop-program').textContent = state.status === 'failed' ? 'Program identity is unavailable.' : 'Program identity will appear after loading.';
      $('crop-program-hash').textContent = '';
    }
  }

  function activeLoad(mode, token) {
    return states[mode].load === token && !token.cancelled;
  }

  function runtimeFailed(mode, token, message) {
    if (!activeLoad(mode, token)) return;
    token.cancelled = true;
    token.controller.abort();
    window.clearTimeout(token.timeout);
    Object.assign(states[mode], { status: 'failed', error: message });
    if (selected === mode) paintRuntime();
  }

  function cancelUnstarted(mode) {
    const state = states[mode];
    const token = state.load;
    if (state.status !== 'loading' || !token || token.started) return;
    token.cancelled = true;
    token.controller.abort();
    window.clearTimeout(token.timeout);
    Object.assign(state, { status: 'idle', metadata: null, load: null });
  }

  function showMode() {
    const corners = selected === 'corners';
    $('crop-selection-title').textContent = corners ? 'Two-corner selection' : 'Requested crop';
    $('crop-selection-help').textContent = corners ? 'Enter the two opposite corners, in either direction.' : 'The region you want to keep.';
    ['min-x', 'min-y', 'max-x', 'max-y'].forEach((name, index) => {
      $(`crop-label-${name}`).textContent = corners
        ? ['First corner x', 'First corner y', 'Second corner x', 'Second corner y'][index]
        : name.replace('-', '_');
    });
    $('crop-mode-help').textContent = corners
      ? 'Only the selection corners are ordered before clipping: (200,200) to (100,100) selects (100,100)–(200,200). Bounds remain unchanged; reversed or zero-size bounds are still empty.'
      : "Keep the requested rectangle's minimum and maximum as entered. Reversed coordinates describe an empty rectangle. Bounds always remain unchanged.";
    $('crop-semantics').textContent = corners
      ? 'After ordering the selection, minimum edges are included and maximum edges are excluded. Zero-size selections remain empty. Touching an edge alone does not create an overlap.'
      : 'Minimum edges are included; maximum edges are excluded. Reversed or zero-size rectangles are empty. Touching an edge alone does not create an overlap.';
    $('crop-method-copy').textContent = corners
      ? 'The DragCropPlan consumer orders only the selection with the Gooo RectangleCanon port, then calls RectangleIntersect and RectangleEmpty. The browser passes all eight original coordinates to the compiled program.'
      : 'The CropPlan consumer calls the Gooo RectangleIntersect and RectangleEmpty ports. The browser passes all eight original coordinates to the compiled program without reordering either rectangle.';
  }

  $('crop-retry').addEventListener('click', () => window.location.reload());
  $('crop-mode').addEventListener('change', () => {
    const next = $('crop-mode').value;
    if (!Object.hasOwn(protocol.modes, next)) { $('crop-mode').value = selected; return; }
    cancelUnstarted(selected);
    selected = next;
    showMode();
    edited('Selection mode changed. Plan the crop to see a new result.');
    loadRuntime(selected);
    paintRuntime();
  });
  protocol.inputIds.forEach(id => $(id).addEventListener('input', () => {
    $('crop-preset').value = '';
    edited();
  }));
  $('crop-preset').addEventListener('change', () => {
    const values = protocol.presets[$('crop-preset').value];
    if (!values) return;
    protocol.inputIds.forEach((id, index) => $(id).value = values[index]);
    edited();
  });

  $('crop-form').addEventListener('submit', event => {
    event.preventDefault();
    clearResult();
    const mode = selected;
    const config = protocol.modeFor(mode);
    if (states[mode].status !== 'ready' || window[config.readyName] !== true || typeof window[config.runtimeName] !== 'function') {
      $('crop-error').textContent = 'Wait for this mode to finish loading, or retry if loading failed.';
      $('crop-error').hidden = false;
      return;
    }
    try {
      const inputs = protocol.validateInputs(protocol.inputIds.map(id => $(id).value), mode);
      const result = protocol.parseReply(window[config.runtimeName](...inputs), inputs, mode);
      protocol.fields.forEach(field => $(`result-${field.replace('_', '-')}`).textContent = result.output[field]);
      $('result-has-pixels').textContent = String(result.output.has_pixels);
      $('crop-result-status').textContent = result.output.has_pixels
        ? `${config.label}: the crop overlaps your bounds. These are the coordinates to keep.`
        : `${config.label}: the intersection is empty. There is no region to keep.`;
      $('crop-result-badge').textContent = 'EXECUTED';
      $('crop-result-values').hidden = false;
      $('crop-raw-output').textContent = JSON.stringify({ mode, entry_id: config.entryId, ...result }, null, 2);
      $('crop-raw-result').hidden = false;
      $('crop-input-status').className = 'crop-input-status';
      $('crop-input-status').textContent = `Executed ${config.activity} in this browser using all eight original coordinates.`;
    } catch (error) {
      clearResult('No crop result was accepted.');
      $('crop-error').textContent = error.message;
      $('crop-error').hidden = false;
      $('crop-input-status').className = 'crop-input-status error';
      $('crop-input-status').textContent = 'Check the input or reported program error, then try again.';
      if (Number.isInteger(error.inputIndex)) {
        $(protocol.inputIds[error.inputIndex]).setAttribute('aria-invalid', 'true');
        $(protocol.inputIds[error.inputIndex]).focus();
      }
    }
  });

  async function loadRuntime(mode) {
    const state = states[mode];
    if (state.status !== 'idle') return;
    const config = protocol.modeFor(mode);
    const token = { controller: new AbortController(), cancelled: false, started: false };
    state.status = 'loading';
    state.load = token;
    token.timeout = window.setTimeout(() => runtimeFailed(mode, token,
      'Loading timed out after 30 seconds. Retry loading.'), 30000);
    try {
      if (typeof WebAssembly === 'undefined' || typeof window.Go !== 'function') throw new Error('The WebAssembly loader is unavailable. Retry loading in a browser with WebAssembly support.');
      if (!globalThis.crypto?.subtle) throw new Error('Program integrity checks require HTTPS or a localhost server. Open the hosted page or serve this folder locally.');
      const options = { cache: 'no-store', signal: token.controller.signal };
      const metadataResponse = await fetch(config.assetBase + 'metadata.json', options);
      if (!activeLoad(mode, token)) return;
      if (!metadataResponse.ok) throw new Error(`Build metadata could not be loaded (HTTP ${metadataResponse.status}).`);
      const metadata = protocol.validateMetadata(await metadataResponse.json(), mode);
      if (!activeLoad(mode, token)) return;
      const response = await fetch(config.assetBase + 'crop.wasm', options);
      if (!activeLoad(mode, token)) return;
      if (!response.ok) throw new Error(`The WebAssembly program could not be loaded (HTTP ${response.status}).`);
      const bytes = await response.arrayBuffer();
      if (!activeLoad(mode, token)) return;
      if (bytes.byteLength !== metadata.wasm_bytes) throw new Error('The program size does not match its metadata. Retry loading.');
      const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes);
      if (!activeLoad(mode, token)) return;
      const observedHash = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
      if (observedHash !== metadata.wasm_sha256) throw new Error('The program hash does not match its metadata. Retry loading.');
      const go = new window.Go();
      const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
      if (!activeLoad(mode, token)) return;
      window[config.readyName] = false;
      delete window[config.runtimeName];
      token.started = true;
      Promise.resolve(go.run(instance)).then(
        () => runtimeFailed(mode, token, 'The program stopped. Retry loading to plan another crop.'),
        () => runtimeFailed(mode, token, 'The program stopped unexpectedly. Retry loading to plan another crop.')
      );
      while (activeLoad(mode, token) && (window[config.readyName] !== true || typeof window[config.runtimeName] !== 'function')) {
        await new Promise(resolve => window.setTimeout(resolve, 25));
      }
      if (!activeLoad(mode, token)) return;
      Object.assign(state, { status: 'ready', metadata });
      if (selected === mode) paintRuntime();
    } catch (error) {
      runtimeFailed(mode, token, error.message || 'The program could not be loaded. Retry loading.');
    } finally { window.clearTimeout(token.timeout); }
  }

  showMode();
  loadRuntime(selected);
  paintRuntime();
})();
