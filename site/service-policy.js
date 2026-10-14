(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const protocol = window.GoooServicePolicyProtocol;
  const fieldNames = ['pattern', 'before', 'after', 'req_type', 'req_name'];
  const values = Object.fromEntries(fieldNames.map(name => [name, $('policy-' + name).value]));
  const origins = Object.fromEntries(fieldNames.map(name => [name, 'editor']));
  const fieldLabels = { pattern: 'Service pattern', before: 'Current grant pattern', after: 'Proposed grant pattern', req_type: 'Request type', req_name: 'Request name' };
  let worker = null, program = null, job = null, completed = null, nextId = 0;
  let loadTimer = null, runTimer = null, factsValid = false;
  const downloads = new Map();
  const modeHelp = {
    match: 'Check one supplied request against one pattern. A valid refusal is different from malformed input.',
    impact: 'Compare the same supplied request before and after. This request alone cannot establish the relationship between the grants.',
    refine: 'Compare two single patterns. A widening witness is a concrete request allowed only by the proposal.'
  };
  function facts() {
    const fields = protocol.MODE_FIELDS[$('policy-mode').value];
    if (!fields) throw new Error('Choose a supported check.');
    return Object.fromEntries(fields.map(name => [name, values[name]]));
  }
  function clearTimers() {
    if (loadTimer !== null) window.clearTimeout(loadTimer);
    if (runTimer !== null) window.clearTimeout(runTimer);
    loadTimer = null; runTimer = null;
  }
  function available() {
    return !!completed && !!program && !job && factsValid &&
      completed.mode === $('policy-mode').value && JSON.stringify(completed.facts) === JSON.stringify(facts());
  }
  function buttons() {
    $('policy-run').disabled = !program || !factsValid || !!job;
    $('policy-run').textContent = job ? 'Checking…' : 'Run policy check';
    $('policy-cancel').hidden = !job;
    $('policy-download-controls').hidden = !available();
    $('policy-download').disabled = !available();
  }
  function clearResult(message = 'No check has run for these inputs.') {
    completed = null;
    $('policy-fields').replaceChildren();
    $('policy-result').hidden = true;
    $('policy-summary').textContent = '';
    $('policy-proof-wrap').hidden = true;
    $('policy-proof').textContent = '';
    $('policy-error').hidden = true;
    $('policy-error').textContent = '';
    $('policy-download-status').hidden = true;
    $('policy-download-status').textContent = '';
    $('policy-progress').textContent = message;
    buttons();
  }
  function refreshInputs() {
    const mode = $('policy-mode').value, active = protocol.MODE_FIELDS[mode] || [];
    document.querySelectorAll('[data-policy-field]').forEach(row => { row.hidden = !active.includes(row.dataset.policyField); });
    $('policy-mode-help').textContent = modeHelp[mode] || 'Choose a supported check.';
    $('policy-input-preview').textContent = active.map(name => name + ': ' + JSON.stringify(values[name])).join('\n');
    const captured = active.filter(name => origins[name] !== 'editor');
    $('policy-input-origin').textContent = captured.length
      ? captured.map(name => `${fieldLabels[name]}: ${origins[name] === 'paste' ? 'pasted' : 'dropped'} plain text.`).join(' ') + ' The empty editors keep these captured values until you replace or clear them.'
      : 'Using the current editor values.';
    fieldNames.forEach(name => {
      const isCaptured = origins[name] !== 'editor';
      $('policy-capture-' + name).hidden = !isCaptured;
      $('policy-origin-' + name).textContent = isCaptured
        ? `${origins[name] === 'paste' ? 'Pasted' : 'Dropped'} text is captured below. Typing replaces this field.` : '';
    });
    $('policy-input-error').hidden = true;
    $('policy-input-error').textContent = '';
    factsValid = false;
    fieldNames.forEach(name => $('policy-' + name).setAttribute('aria-invalid', 'false'));
    try { protocol.validateFacts(mode, facts()); factsValid = true; }
    catch (error) { $('policy-input-error').textContent = error.message; $('policy-input-error').hidden = false; }
    buttons();
  }
  function failed(message) {
    clearTimers();
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    clearResult('The policy program is unavailable.');
    $('policy-runtime').textContent = message;
    $('policy-runtime').className = 'literal-runtime error';
    $('policy-error').textContent = message;
    $('policy-error').hidden = false;
    $('policy-program').textContent = 'Program identity is unavailable.';
    $('policy-sources').replaceChildren();
    $('policy-retry').hidden = false;
    buttons();
  }
  function showProgram() {
    $('policy-program').textContent = `Compiler ${program.bindings.compiler_source_sha} · generation ${program.bindings.generation_sha256}`;
    const links = Object.keys(program.files).filter(name => name.startsWith('source/')).map(name => {
      const row = document.createElement('li'), link = document.createElement('a');
      link.href = './service-policy/' + name; link.textContent = name.slice('source/'.length);
      row.append(link); return row;
    });
    $('policy-sources').replaceChildren(...links);
  }
  function showResult(proof, mode) {
    const decision = proof.decision;
    let summary;
    if (decision.domain_status !== 'VALID_INPUT') {
      summary = `Gooo rejected the input in ${decision.invalid_field}. The semantic result is unavailable; inspect the captured input and supported domain.`;
    } else if (mode === 'match') {
      summary = decision.matched ? 'This supplied request matches the pattern.' : 'The input is valid. This supplied request does not match the pattern.';
    } else if (mode === 'impact') {
      summary = {
        UNCHANGED_ALLOW: 'Both patterns admit this request. Other requests may behave differently.',
        UNCHANGED_DENY: 'Both patterns refuse this request. Other requests may behave differently.',
        WIDENED: 'The proposal admits this request; the current pattern does not.',
        REVOKED: 'The current pattern admits this request; the proposal does not.'
      }[decision.change];
    } else if (decision.status === 'WIDENING_WITNESS') {
      summary = `Gooo found added access: ${decision.witness_type}://${decision.witness_name}. Only the proposed pattern admits this request.`;
    } else if (decision.status === 'SUPPORTED_SUBSET') {
      summary = `Gooo recognizes the proposed pattern as a subset using ${decision.rule_id}. This establishes the supported pattern relationship, not complete service authorization.`;
    } else {
      summary = 'Gooo did not establish a subset or find a widening witness. The relationship remains unproven.';
    }
    $('policy-summary').textContent = summary;
    $('policy-summary').className = 'policy-summary' + (decision.domain_status === 'VALID_INPUT' ? '' : ' invalid');
    const rows = Object.entries(decision).map(([name, value]) => {
      const row = document.createElement('div'), label = document.createElement('dt'), result = document.createElement('dd');
      label.textContent = name;
      result.textContent = typeof value === 'string' ? JSON.stringify(value) : String(value);
      row.append(label, result); return row;
    });
    $('policy-fields').replaceChildren(...rows);
    $('policy-result').hidden = false;
    $('policy-proof').textContent = JSON.stringify(proof, null, 2);
    $('policy-proof-wrap').hidden = false;
    $('policy-progress').textContent = 'Complete · one checked Gooo policy call.';
  }
  function loadWorker() {
    clearTimers();
    if (worker) worker.terminate();
    worker = null; program = null; job = null;
    $('policy-runtime').textContent = 'Loading the compiled Gooo policy rules…';
    $('policy-runtime').className = 'literal-runtime';
    $('policy-program').textContent = 'Waiting for the program manifest.';
    $('policy-sources').replaceChildren();
    $('policy-retry').hidden = true;
    buttons();
    try {
      const instance = new Worker('./service-policy-worker.js');
      worker = instance;
      loadTimer = window.setTimeout(() => {
        if (worker === instance && !program) failed('Program loading timed out after 60 seconds. Retry loading.');
      }, 60000);
      instance.onmessage = event => {
        if (worker !== instance) return;
        const message = event.data;
        try {
          if (!message || typeof message !== 'object') throw new Error('The worker returned an invalid message.');
          if (message.type === 'ready') {
            if (program) throw new Error('The worker returned a duplicate ready message.');
            program = protocol.validateReady(message);
            if (loadTimer !== null) window.clearTimeout(loadTimer);
            loadTimer = null;
            $('policy-runtime').textContent = 'Ready · compiled Gooo rules run here; inputs stay in this browser.';
            showProgram(); buttons();
          } else if (message.type === 'result') {
            if (!job || message.id !== job.request.id) return;
            const proof = protocol.validateResult(message, { ...job.request, program: job.program });
            if (runTimer !== null) window.clearTimeout(runTimer);
            runTimer = null;
            const current = job; job = null;
            showResult(proof, current.request.mode);
            completed = Object.freeze({ mode: current.request.mode, facts: current.request.facts, proof, program: current.program });
            buttons();
          } else if (message.type === 'error') {
            if (message.id !== undefined && (!job || message.id !== job.request.id)) return;
            failed(protocol.validateError(message, job ? job.request.id : undefined));
          } else throw new Error('The worker returned an unknown message type.');
        } catch (error) { failed(error.message); }
      };
      instance.onerror = event => {
        if (worker !== instance) return;
        event.preventDefault?.(); failed('The policy worker stopped. Retry loading.');
      };
      instance.onmessageerror = () => { if (worker === instance) failed('The policy worker returned an unreadable message.'); };
    } catch { failed('The policy worker could not start. Retry loading in a browser with Worker support.'); }
  }
  function invalidate(message) {
    if (job) loadWorker();
    clearResult(message); refreshInputs();
  }
  fieldNames.forEach(name => {
    const input = $('policy-' + name);
    input.addEventListener('input', () => { values[name] = input.value; origins[name] = 'editor'; invalidate(); });
    $('policy-clear-' + name).addEventListener('click', () => {
      if (origins[name] === 'editor') return;
      input.value = ''; values[name] = ''; origins[name] = 'editor'; invalidate();
    });
    for (const kind of ['paste', 'drop']) input.addEventListener(kind, event => {
      event.preventDefault();
      input.value = '';
      const data = kind === 'paste' ? event.clipboardData : event.dataTransfer;
      try {
        if (!data || typeof data.getData !== 'function' || data.files?.length ||
            (data.types?.length && !Array.from(data.types).includes('text/plain'))) throw new Error('Paste or drop plain text.');
        values[name] = data.getData('text/plain');
        origins[name] = kind;
        invalidate();
      } catch (error) {
        values[name] = ''; origins[name] = 'editor';
        invalidate(); factsValid = false;
        $('policy-input-error').textContent = error.message; $('policy-input-error').hidden = false; buttons();
      }
    });
    input.addEventListener('dragover', event => event.preventDefault());
  });
  $('policy-mode').addEventListener('change', () => invalidate());
  const presets = {
    match: ['match', { pattern: 'mcp://weather', req_type: 'mcp', req_name: 'weather' }],
    denied: ['match', { pattern: 'mcp://weather', req_type: 'mcp', req_name: 'stocks' }],
    'same-request': ['impact', { before: 'mcp://*.x', after: 'mcp://a.*', req_type: 'mcp', req_name: 'a.x' }],
    witness: ['refine', { before: 'mcp://*.x', after: 'mcp://a.*' }],
    subset: ['refine', { before: 'mcp://*', after: 'mcp://weather' }],
    invalid: ['match', { pattern: 'mcp://a*.', req_type: 'mcp', req_name: 'weather' }]
  };
  document.querySelectorAll('[data-policy-preset]').forEach(button => button.addEventListener('click', () => {
    const preset = presets[button.dataset.policyPreset];
    if (!preset) return;
    $('policy-mode').value = preset[0];
    for (const [name, value] of Object.entries(preset[1])) { values[name] = value; origins[name] = 'editor'; $('policy-' + name).value = value; }
    invalidate();
  }));
  $('policy-form').addEventListener('submit', event => {
    event.preventDefault();
    if (!worker || !program || job || !factsValid) return;
    if (nextId === Number.MAX_SAFE_INTEGER) { failed('The request identifier limit was reached. Reload this page.'); return; }
    const request = { type: 'evaluate', id: ++nextId, mode: $('policy-mode').value, facts: Object.freeze(facts()) };
    try { protocol.validateRequest(request); }
    catch (error) { failed(error.message); return; }
    clearResult('Checking the captured inputs with Gooo…');
    job = { request, program };
    const instance = worker;
    runTimer = window.setTimeout(() => {
      if (worker === instance && job?.request.id === request.id) failed('The check timed out after 15 seconds. The worker stopped; retry loading.');
    }, 15000);
    buttons();
    try { worker.postMessage(request); }
    catch { failed('The inputs could not be sent to the policy worker. Retry loading.'); }
  });
  $('policy-cancel').addEventListener('click', () => { if (job) { loadWorker(); clearResult('Check cancelled. No result was retained.'); } });
  $('policy-retry').addEventListener('click', () => { clearResult(); loadWorker(); });
  function revoke(url) {
    if (!downloads.has(url)) return;
    const timer = downloads.get(url);
    if (timer !== null) window.clearTimeout(timer);
    downloads.delete(url); window.URL.revokeObjectURL(url);
  }
  $('policy-download').addEventListener('click', () => {
    if (!available() || $('policy-download').disabled) return;
    let url = null, link = null, requested = false;
    try {
      const report = protocol.observation(completed.mode, completed.facts, completed.proof, completed.program);
      url = window.URL.createObjectURL(new Blob([JSON.stringify(report, null, 2) + '\n'], { type: 'application/json;charset=utf-8' }));
      downloads.set(url, null);
      link = document.createElement('a'); link.href = url; link.download = 'gooo-service-policy-observation.json'; link.hidden = true;
      document.body.append(link); link.click(); requested = true;
      $('policy-download-status').textContent = 'Download requested. Check your browser downloads for gooo-service-policy-observation.json.';
    } catch { $('policy-download-status').textContent = 'The observation download could not start. Your result is still available.'; }
    finally {
      if (link) link.remove();
      if (url !== null) { if (requested) downloads.set(url, window.setTimeout(() => revoke(url), 1000)); else revoke(url); }
      $('policy-download-status').hidden = false;
    }
  });
  window.addEventListener('pagehide', () => {
    clearTimers(); if (worker) worker.terminate();
    worker = null; program = null; job = null;
    for (const url of [...downloads.keys()]) revoke(url);
    clearResult('Program stopped when this page left the active view.');
  });
  window.addEventListener('pageshow', event => { if (event.persisted && !worker) { clearResult(); loadWorker(); } });
  refreshInputs(); loadWorker();
})();
