(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooCropPlannerProtocol;
  let ready = false;
  let failed = false;

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

  function edited() {
    clearResult('Coordinates changed. Plan the crop to see a new result.');
    protocol.inputIds.forEach(id => $(id).setAttribute('aria-invalid', 'false'));
    $('crop-input-status').className = 'crop-input-status';
    $('crop-input-status').textContent = 'Coordinates are kept exactly as entered; they are not reordered.';
  }

  function runtimeFailed(message) {
    failed = true;
    ready = false;
    $('crop-run').disabled = true;
    clearResult('Crop planning is unavailable.');
    $('crop-runtime').className = 'crop-runtime error';
    $('crop-runtime').textContent = message;
    $('crop-error').textContent = message;
    $('crop-error').hidden = false;
    $('crop-program').textContent = 'Program identity is unavailable.';
    $('crop-program-hash').textContent = '';
    $('crop-retry').hidden = false;
  }

  $('crop-retry').addEventListener('click', () => window.location.reload());
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
    if (!ready || failed || window.goooCropReady !== true || typeof window.goooCrop !== 'function') {
      $('crop-error').textContent = 'Wait for the program to finish loading, or retry if loading failed.';
      $('crop-error').hidden = false;
      return;
    }
    try {
      const inputs = protocol.validateInputs(protocol.inputIds.map(id => $(id).value));
      const result = protocol.parseReply(window.goooCrop(...inputs), inputs);
      protocol.fields.forEach(field => $(`result-${field.replace('_', '-')}`).textContent = result.output[field]);
      $('result-has-pixels').textContent = String(result.output.has_pixels);
      $('crop-result-status').textContent = result.output.has_pixels
        ? 'The crop overlaps your bounds. These are the coordinates to keep.'
        : 'The intersection is empty. There is no region to keep.';
      $('crop-result-badge').textContent = 'EXECUTED';
      $('crop-result-values').hidden = false;
      $('crop-raw-output').textContent = JSON.stringify(result, null, 2);
      $('crop-raw-result').hidden = false;
      $('crop-input-status').className = 'crop-input-status';
      $('crop-input-status').textContent = 'Executed in this browser using all eight original coordinates.';
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

  function showIdentity(metadata) {
    $('crop-program').textContent = `Source ${metadata.source_revision}${metadata.source_dirty ? ' + local changes' : ''} · compiler ${metadata.compiler_revision} · ${metadata.go_version}`;
    $('crop-program-hash').textContent = `WebAssembly SHA-256 ${metadata.wasm_sha256}`;
    const sourceBase = metadata.source_dirty ? './crop/source/'
      : `https://github.com/kimjooyoon/gooo-go-ports/blob/${metadata.source_revision}/`;
    $('crop-source-link').href = sourceBase + 'examples/crop-planner/crop_planner.gooo';
    $('crop-port-link').href = sourceBase + 'ports/image/geometry.gooo';
    if (!metadata.source_dirty) $('crop-native-guide').href = `https://github.com/kimjooyoon/gooo-go-ports/tree/${metadata.source_revision}/examples/crop-planner`;
  }

  async function loadRuntime() {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      controller.abort();
      runtimeFailed('Loading timed out after 30 seconds. Retry loading.');
    }, 30000);
    try {
      if (typeof WebAssembly === 'undefined' || typeof window.Go !== 'function') throw new Error('The WebAssembly loader is unavailable. Retry loading in a browser with WebAssembly support.');
      if (!globalThis.crypto?.subtle) throw new Error('Program integrity checks require HTTPS or a localhost server. Open the hosted page or serve this folder locally.');
      const options = { cache: 'no-store', signal: controller.signal };
      const metadataResponse = await fetch('./crop/metadata.json', options);
      if (!metadataResponse.ok) throw new Error(`Build metadata could not be loaded (HTTP ${metadataResponse.status}).`);
      const metadata = protocol.validateMetadata(await metadataResponse.json());
      const response = await fetch('./crop/crop.wasm', options);
      if (!response.ok) throw new Error(`The WebAssembly program could not be loaded (HTTP ${response.status}).`);
      const bytes = await response.arrayBuffer();
      if (bytes.byteLength !== metadata.wasm_bytes) throw new Error('The program size does not match its metadata. Retry loading.');
      const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes);
      const observedHash = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
      if (observedHash !== metadata.wasm_sha256) throw new Error('The program hash does not match its metadata. Retry loading.');
      if (failed) return;
      const go = new window.Go();
      const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
      if (failed) return;
      Promise.resolve(go.run(instance)).then(
        () => runtimeFailed('The program stopped. Retry loading to plan another crop.'),
        () => runtimeFailed('The program stopped unexpectedly. Retry loading to plan another crop.')
      );
      while (!failed && (window.goooCropReady !== true || typeof window.goooCrop !== 'function')) {
        await new Promise(resolve => window.setTimeout(resolve, 25));
      }
      if (failed) return;
      ready = true;
      showIdentity(metadata);
      $('crop-run').disabled = false;
      $('crop-runtime').textContent = 'WebAssembly ready · coordinates stay in this browser';
    } catch (error) {
      if (!failed) runtimeFailed(error.message || 'The program could not be loaded. Retry loading.');
    } finally { window.clearTimeout(timeout); }
  }

  loadRuntime();
})();
