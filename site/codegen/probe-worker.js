"use strict";

const REQUEST_SCHEMA = "gooo/browser-body-codegen-request/v1";
const RESPONSE_SCHEMA = "gooo/browser-body-codegen-response/v1";
const METADATA_SCHEMA = "gooo/browser-body-codegen-metadata/v1";
const BUILD_SCHEMA = "gooo/browser-codegen-build/v1";
const staticFetch = self.fetch.bind(self);
const encoder = new TextEncoder();
let buildManifest;
let bridgeMetadata;
let ready = false;
let busy = false;
let stopped = false;

// This worker has no networking API for compilation. Only these two static
// reads and the local Go runtime import occur during initialization.
self.fetch = () => Promise.reject(new Error("Network access is disabled in the compiler worker."));

function messageText(error) {
  return error instanceof Error ? error.message : String(error);
}

function fatal(error) {
  if (stopped) return;
  stopped = true;
  ready = false;
  self.postMessage({ type: "fatal", message: messageText(error) });
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

async function sha256(bytes) {
  const digest = await self.crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, "0")).join("");
}

async function readStaticAsset(name) {
  const url = new URL(name, self.location.href);
  if (url.origin !== self.location.origin) throw new Error("Static asset must share the worker origin.");
  const response = await staticFetch(url.href, { cache: "no-store", credentials: "omit", redirect: "error" });
  if (!response.ok) throw new Error(`Could not load ${name}: HTTP ${response.status}.`);
  return response;
}

function verifyManifest(manifest) {
  if (!manifest || manifest.schema !== BUILD_SCHEMA ||
      !/^[a-f0-9]{40}$/.test(manifest.compiler_pin) ||
      !/^[a-f0-9]{64}$/.test(manifest.bridge_source_sha256) ||
      !Number.isSafeInteger(manifest.wasm?.bytes) || manifest.wasm.bytes <= 0 ||
      !/^[a-f0-9]{64}$/.test(manifest.wasm?.sha256)) {
    throw new Error("Build manifest is missing the expected provenance or WebAssembly digest.");
  }
}

function verifyMetadata(metadata) {
  if (!metadata || metadata.schema !== METADATA_SCHEMA ||
      metadata.compiler_pin !== buildManifest.compiler_pin ||
      metadata.bridge_source_sha256 !== buildManifest.bridge_source_sha256 ||
      metadata.native_execution !== false || metadata.model_calls !== false) {
    throw new Error("Compiler metadata does not match the separately verified build manifest.");
  }
  const limits = metadata.resource_limits;
  for (const key of ["max_source_bytes", "max_request_bytes", "max_filename_bytes", "max_id_bytes", "max_activity_bytes"]) {
    if (!Number.isSafeInteger(limits?.[key]) || limits[key] <= 0) {
      throw new Error("Compiler metadata is missing a valid resource limit.");
    }
  }
  if (limits.max_source_bytes > 64 * 1024) throw new Error("Compiler source limit exceeds this probe's 64 KiB boundary.");
}

async function initialize() {
  try {
    self.postMessage({ type: "status", message: "Loading and verifying the WebAssembly compiler…" });
    const [manifestResponse, wasmResponse] = await Promise.all([
      readStaticAsset("./build-manifest.json"), readStaticAsset("./codegen.wasm")
    ]);
    buildManifest = await manifestResponse.json();
    verifyManifest(buildManifest);
    const wasmBytes = await wasmResponse.arrayBuffer();
    if (wasmBytes.byteLength !== buildManifest.wasm.bytes) throw new Error("WebAssembly byte size does not match the build manifest.");
    const wasmSHA256 = await sha256(wasmBytes);
    if (wasmSHA256 !== buildManifest.wasm.sha256) throw new Error("WebAssembly SHA-256 does not match the build manifest.");
    self.postMessage({ type: "status", message: "WebAssembly verified. Starting the compiler worker…" });
    importScripts("./wasm_exec.js");
    if (typeof self.Go !== "function") throw new Error("The local Go WebAssembly runtime did not load.");
    const go = new self.Go();
    const instantiated = await WebAssembly.instantiate(wasmBytes, go.importObject);
    Promise.resolve(go.run(instantiated.instance)).then(
      () => fatal(new Error("The compiler worker exited. Reload it to retry.")),
      error => fatal(error)
    );
    while (self.goooBrowserReady !== true) {
      if (stopped) return;
      await new Promise(resolve => setTimeout(resolve, 20));
    }
    if (typeof self.goooBrowserMetadata !== "function" || typeof self.goooBrowserGenerate !== "function") {
      throw new Error("The expected compiler bridge was not registered.");
    }
    bridgeMetadata = JSON.parse(self.goooBrowserMetadata());
    verifyMetadata(bridgeMetadata);
    ready = true;
    self.postMessage({ type: "ready", metadata: bridgeMetadata, manifest: buildManifest });
  } catch (error) {
    fatal(error);
  }
}

function validateRequest(request) {
  if (!request || request.schema !== REQUEST_SCHEMA) throw new Error("Unexpected request schema.");
  const limits = bridgeMetadata.resource_limits;
  for (const [key, maximum] of [["id", limits.max_id_bytes], ["filename", limits.max_filename_bytes], ["activity", limits.max_activity_bytes], ["source", limits.max_source_bytes]]) {
    if (typeof request[key] !== "string" || !request[key].length || hasUnpairedSurrogate(request[key])) {
      throw new Error(`${key} must be nonempty valid Unicode text.`);
    }
    if (encoder.encode(request[key]).byteLength > maximum) throw new Error(`${key} exceeds the compiler bridge's UTF-8 byte limit.`);
  }
  if (/[\/\\\u0000]/.test(request.filename) || request.filename === "." || request.filename === "..") throw new Error("Filename must be a diagnostic basename.");
  if (encoder.encode(JSON.stringify(request)).byteLength > limits.max_request_bytes) throw new Error("Request exceeds the compiler bridge's UTF-8 byte limit.");
}

self.onmessage = async event => {
  const message = event.data;
  if (message?.type !== "generate" || stopped) return;
  const request = message.request;
  if (!ready || busy) {
    self.postMessage({ type: "generation-error", id: request?.id, sourceSha: message.sourceSha, activity: request?.activity, message: "Compiler worker is not ready for another request." });
    return;
  }
  busy = true;
  let sourceSha = message.sourceSha;
  try {
    validateRequest(request);
    sourceSha = `sha256:${await sha256(encoder.encode(request.source))}`;
    if (sourceSha !== message.sourceSha) throw new Error("Request source digest does not match the editor digest.");
    // Keep the original response text: JSON.parse rounds int64 values in reports.
    // Parsed fields below are used only for string/boolean request bindings.
    const responseJSON = self.goooBrowserGenerate(JSON.stringify(request));
    const response = JSON.parse(responseJSON);
    if (response.schema !== RESPONSE_SCHEMA || response.id !== request.id ||
        response.source_sha256 !== sourceSha || response.activity !== request.activity ||
        response.compiler_pin !== bridgeMetadata.compiler_pin ||
        response.bridge_source_sha256 !== bridgeMetadata.bridge_source_sha256 ||
        response.native_execution !== false || response.model_calls !== false ||
        typeof response.ok !== "boolean" || typeof response.source !== "string" ||
        (response.ok && (!response.source || !response.report || typeof response.report !== "object"))) {
      throw new Error("Compiler response failed request or provenance binding checks.");
    }
    self.postMessage({ type: "result", id: request.id, sourceSha, activity: request.activity, response, responseJSON });
  } catch (error) {
    self.postMessage({ type: "generation-error", id: request?.id, sourceSha, activity: request?.activity, message: messageText(error) });
  } finally {
    busy = false;
  }
};

initialize();
