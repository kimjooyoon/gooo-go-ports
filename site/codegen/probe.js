"use strict";

(() => {
  const byID = id => document.getElementById(id);
  const sourceInput = byID("probe-source");
  const activityInput = byID("probe-activity");
  const casesInput = byID("probe-cases");
  const generateButton = byID("probe-generate");
  const retryButton = byID("probe-retry");
  const output = byID("probe-output");
  const errorOutput = byID("probe-error");
  const reportOutput = byID("probe-report");
  const statusOutput = byID("probe-status");
  const encoder = new TextEncoder();
  let worker = null;
  let metadata = null;
  let fixtures = [];
  let selectedFixture = null;
  let fixturesReady = false;
  let fixturesError = null;
  let revision = 0;
  let preparing = null;
  let activeRequest = null;
  let initializationTimer = null;
  let generationTimer = null;

  function status(message) { statusOutput.textContent = message; }

  function clearOutputs() {
    output.textContent = "";
    errorOutput.textContent = "";
    reportOutput.textContent = "";
    byID("probe-result-binding").textContent = "";
    byID("probe-engine-provenance").textContent = "No current report.";
  }

  function refreshControls() {
    generateButton.disabled = !metadata || !fixturesReady || preparing || !!activeRequest;
    casesInput.disabled = !fixturesReady;
  }

  function clearProvenance() {
    for (const id of ["probe-compiler-pin", "probe-bridge-sha", "probe-wasm-sha", "probe-go-version", "probe-bridge-limits"]) {
      byID(id).textContent = "Not verified for an active worker.";
    }
    byID("probe-source-limit").textContent = "Source limit will be read from verified bridge metadata.";
  }

  function hasUnpairedSurrogate(value) {
    for (let index = 0; index < value.length; index += 1) {
      const code = value.charCodeAt(index);
      if (code >= 0xd800 && code <= 0xdbff) {
        const next = value.charCodeAt(index + 1);
        if (!(next >= 0xdc00 && next <= 0xdfff)) return true;
        index += 1;
      } else if (code >= 0xdc00 && code <= 0xdfff) {
        return true;
      }
    }
    return false;
  }

  async function sourceDigest(source) {
    const digest = await crypto.subtle.digest("SHA-256", encoder.encode(source));
    return "sha256:" + Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, "0")).join("");
  }

  function inputEdited() {
    revision += 1;
    clearOutputs();
    if (metadata && fixturesReady) status(activeRequest || preparing ? "Inputs changed. The previous request's output will be ignored." : "Ready. Inputs changed; generate again for the current source.");
  }

  function showFailure(message) {
    clearOutputs();
    errorOutput.textContent = message;
  }

  function stopWorker(message) {
    clearTimeout(initializationTimer);
    clearTimeout(generationTimer);
    initializationTimer = null;
    generationTimer = null;
    if (worker) worker.terminate();
    worker = null;
    metadata = null;
    activeRequest = null;
    preparing = null;
    clearProvenance();
    showFailure(message);
    status("Compiler unavailable. Retry compiler load to continue.");
    retryButton.hidden = false;
    refreshControls();
  }

  function showProvenance(metadataValue, manifest) {
    byID("probe-compiler-pin").textContent = metadataValue.compiler_pin;
    byID("probe-bridge-sha").textContent = metadataValue.bridge_source_sha256;
    byID("probe-wasm-sha").textContent = `${manifest.wasm.bytes.toLocaleString()} bytes / ${manifest.wasm.sha256} (verified)`;
    byID("probe-go-version").textContent = metadataValue.go_version;
    const limits = metadataValue.resource_limits;
    byID("probe-bridge-limits").textContent = `${limits.max_source_bytes.toLocaleString()} source bytes; ${limits.max_request_bytes.toLocaleString()} request bytes; ${limits.max_activity_bytes} activity bytes (UTF-8). Native execution: false. Model calls: false.`;
    byID("probe-source-limit").textContent = `Verified source limit: ${limits.max_source_bytes.toLocaleString()} UTF-8 bytes (${limits.max_source_bytes / 1024} KiB).`;
  }

  function currentInputs() {
    const request = activeRequest;
    return request && request.revision === revision &&
      sourceInput.value === request.source && activityInput.value === request.activity;
  }

  function finishRequest(message) {
    const request = activeRequest;
    if (!request || message.id !== request.id) return;
    const isCurrent = currentInputs();
    clearTimeout(generationTimer);
    generationTimer = null;
    activeRequest = null;
    refreshControls();
    if (!isCurrent) {
      status("Ready. Previous output discarded because its inputs are no longer current.");
      return;
    }
    if (message.sourceSha !== request.sourceSha || message.activity !== request.activity) {
      stopWorker("Worker response did not match the current source digest and activity. No output was displayed.");
      return;
    }
    clearOutputs();
    if (message.type === "generation-error") {
      errorOutput.textContent = message.message;
      status("Generation failed. No Go output returned.");
      return;
    }
    const response = message.response;
    if (!response || response.schema !== "gooo/browser-body-codegen-response/v1" ||
        response.id !== request.id || response.source_sha256 !== request.sourceSha ||
        response.activity !== request.activity || response.compiler_pin !== metadata.compiler_pin ||
        response.bridge_source_sha256 !== metadata.bridge_source_sha256 ||
        response.native_execution !== false || response.model_calls !== false || typeof response.ok !== "boolean" ||
        typeof message.responseJSON !== "string") {
      stopWorker("Compiler response failed request or provenance binding checks. No output was displayed.");
      return;
    }
    if (!response.ok) {
      errorOutput.textContent = response.error || "The compiler rejected this request.";
      if (response.diagnostic) errorOutput.textContent += "\n\n" + JSON.stringify(response.diagnostic, null, 2);
      status(`Generation failed for ${request.activity}. No Go output returned.`);
      return;
    }
    output.textContent = response.source;
    // Preserve numeric tokens and the engine's provenance without a JS round trip.
    reportOutput.textContent = message.responseJSON;
    byID("probe-result-binding").textContent = `Activity: ${request.activity} · Source: ${request.sourceSha} · Request: ${request.id}`;
    byID("probe-engine-provenance").textContent = typeof response.report?.compiler_source_sha === "string" ? response.report.compiler_source_sha : "See the unmodified compiler report.";
    status(`Generated Go for ${request.activity}. Native execution: false. Model calls: false.`);
  }

  function startWorker() {
    clearOutputs();
    revision += 1;
    metadata = null;
    clearProvenance();
    retryButton.hidden = true;
    refreshControls();
    status("Loading and verifying the WebAssembly compiler…");
    if (worker) worker.terminate();
    try {
      const candidate = new Worker("./probe-worker.js");
      worker = candidate;
      initializationTimer = setTimeout(() => {
        if (worker === candidate && !metadata) stopWorker("Compiler initialization exceeded 60 seconds. The worker was terminated; retry to load it again.");
      }, 60000);
      candidate.onmessage = event => {
        if (worker !== candidate) return;
        const message = event.data;
        if (message?.type === "status" && !metadata) {
          status(message.message);
        } else if (message?.type === "fatal") {
          stopWorker(message.message);
        } else if (message?.type === "ready") {
          clearTimeout(initializationTimer);
          initializationTimer = null;
          const value = message.metadata;
          const manifest = message.manifest;
          if (!value || value.schema !== "gooo/browser-body-codegen-metadata/v1" ||
              value.compiler_pin !== manifest?.compiler_pin || value.bridge_source_sha256 !== manifest?.bridge_source_sha256 ||
              value.native_execution !== false || value.model_calls !== false ||
              !Number.isSafeInteger(value.resource_limits?.max_source_bytes) || value.resource_limits.max_source_bytes <= 0 || value.resource_limits.max_source_bytes > 65536) {
            stopWorker("Worker metadata failed provenance or source-limit validation.");
            return;
          }
          metadata = value;
          showProvenance(value, manifest);
          refreshControls();
          status(fixturesError ? "Saved samples could not be loaded. Reload this page to retry." : fixturesReady ? "Ready. Select a sample or edit the source, then generate Go." : "Compiler verified. Loading saved samples…");
        } else if (message?.type === "result" || message?.type === "generation-error") {
          finishRequest(message);
        }
      };
      candidate.onerror = event => {
        if (worker !== candidate) return;
        event.preventDefault();
        stopWorker(event.message || "Compiler worker failed. Retry to load it again.");
      };
      candidate.onmessageerror = () => {
        if (worker === candidate) stopWorker("Compiler worker returned an unreadable message. Retry to load it again.");
      };
    } catch (error) {
      stopWorker(error instanceof Error ? error.message : String(error));
    }
  }

  function chooseFixture() {
    selectedFixture = fixtures.find(fixture => fixture.id === casesInput.value) || null;
    if (!selectedFixture) return;
    sourceInput.value = selectedFixture.request.source;
    activityInput.value = selectedFixture.request.activity;
    inputEdited();
    if (metadata && !activeRequest && !preparing) status(`Ready. Saved sample “${selectedFixture.id}” loaded; source is editable.`);
  }

  async function loadFixtures() {
    try {
      const response = await fetch("./fixtures.json", { cache: "no-store", credentials: "omit", redirect: "error" });
      if (!response.ok) throw new Error(`Could not load saved samples: HTTP ${response.status}.`);
      const data = await response.json();
      if (data.schema !== "gooo/browser-codegen-probe-fixtures/v1" || !Array.isArray(data.cases) || !data.cases.length) throw new Error("Saved samples have an unexpected schema.");
      const seen = new Set();
      fixtures = data.cases.map(fixture => {
        if (typeof fixture.id !== "string" || !fixture.id || seen.has(fixture.id) ||
            fixture.request?.schema !== "gooo/browser-body-codegen-request/v1" ||
            typeof fixture.request.source !== "string" || typeof fixture.request.activity !== "string" ||
            typeof fixture.request.filename !== "string" || /[\/\\\u0000]/.test(fixture.request.filename)) throw new Error("A saved sample has invalid or duplicate input fields.");
        seen.add(fixture.id);
        return fixture;
      });
      casesInput.textContent = "";
      for (const fixture of fixtures) {
        const option = document.createElement("option");
        option.value = fixture.id;
        option.textContent = fixture.id;
        casesInput.appendChild(option);
      }
      fixturesReady = true;
      fixturesError = null;
      casesInput.value = fixtures[0].id;
      chooseFixture();
      refreshControls();
    } catch (error) {
      fixturesReady = false;
      fixturesError = error instanceof Error ? error.message : String(error);
      showFailure(fixturesError);
      status("Saved samples could not be loaded. Reload this page to retry.");
      refreshControls();
    }
  }

  async function generate() {
    if (!metadata || !worker || preparing || activeRequest || !fixturesReady) return;
    clearOutputs();
    const source = sourceInput.value;
    const activity = activityInput.value;
    const expectedWorker = worker;
    const requestRevision = revision;
    const limits = metadata.resource_limits;
    if (hasUnpairedSurrogate(source)) {
      showFailure("Source contains an unpaired UTF-16 surrogate. Enter valid Unicode text; it will not be silently replaced.");
      status("Source validation failed. No request was sent.");
      return;
    }
    if (!source.length || encoder.encode(source).byteLength > limits.max_source_bytes) {
      showFailure(`Source must contain 1–${limits.max_source_bytes.toLocaleString()} UTF-8 bytes.`);
      status("Source validation failed. No request was sent.");
      return;
    }
    if (!activity.length || hasUnpairedSurrogate(activity) || encoder.encode(activity).byteLength > limits.max_activity_bytes) {
      showFailure(`Activity must contain 1–${limits.max_activity_bytes} UTF-8 bytes of valid Unicode text.`);
      status("Activity validation failed. No request was sent.");
      return;
    }
    const preparation = {};
    preparing = preparation;
    refreshControls();
    status("Binding the current source to this request…");
    try {
      const sourceSha = await sourceDigest(source);
      if (preparing !== preparation || !metadata || worker !== expectedWorker) return;
      if (requestRevision !== revision || sourceInput.value !== source || activityInput.value !== activity) {
        preparing = null;
        status("Inputs changed before generation. Generate again for the current source.");
        refreshControls();
        return;
      }
      const id = `browser-${crypto.randomUUID()}`;
      const filename = selectedFixture && selectedFixture.request.source === source && selectedFixture.request.activity === activity ? selectedFixture.request.filename : "browser.gooo";
      activeRequest = { id, sourceSha, source, activity, revision: requestRevision };
      refreshControls();
      status(`Generating Go for ${activity}…`);
      const currentWorker = worker;
      generationTimer = setTimeout(() => {
        if (worker === currentWorker && activeRequest?.id === id) stopWorker("Generation exceeded 15 seconds. The worker was terminated and no output retained. Retry compiler load to continue.");
      }, 15000);
      worker.postMessage({ type: "generate", sourceSha, request: { schema: "gooo/browser-body-codegen-request/v1", id, filename, source, activity } });
      preparing = null;
      refreshControls();
    } catch (error) {
      if (preparing !== preparation || worker !== expectedWorker) return;
      preparing = null;
      if (requestRevision !== revision) {
        status("Inputs changed before generation. Generate again for the current source.");
        refreshControls();
        return;
      }
      stopWorker(error instanceof Error ? error.message : String(error));
    }
  }

  sourceInput.addEventListener("input", inputEdited);
  activityInput.addEventListener("input", inputEdited);
  casesInput.addEventListener("change", chooseFixture);
  generateButton.addEventListener("click", generate);
  retryButton.addEventListener("click", startWorker);
  window.addEventListener("pagehide", () => {
    clearTimeout(initializationTimer);
    clearTimeout(generationTimer);
    if (worker) worker.terminate();
    worker = null;
    metadata = null;
    activeRequest = null;
    preparing = null;
    refreshControls();
  });
  window.addEventListener("pageshow", event => { if (event.persisted) startWorker(); });
  loadFixtures();
  startWorker();
})();
