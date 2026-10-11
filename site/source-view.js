(() => {
  'use strict';

  const $ = id => document.getElementById(id);
  const caseIDs = ['clamp', 'changed', 'double', 'invalid-body'];
  const buttons = Array.from(document.querySelectorAll('[data-source-case]'));
  const hashPattern = /^[a-f0-9]{64}$/;
  let book;
  let controller;
  let timeout;

  function safeURL(value) {
    if (typeof value !== 'string' || !value) throw new Error('저장 자료의 링크를 읽을 수 없습니다.');
    const url = new URL(value, window.location.href);
    if (url.protocol !== 'https:' && !(url.origin === window.location.origin && url.protocol === 'http:')) {
      throw new Error('저장 자료의 링크 형식을 확인할 수 없습니다.');
    }
    return url.href;
  }

  function validateBook(value) {
    const invalid = () => { throw new Error('저장 자료의 형식이 맞지 않습니다. 소스와 출력은 표시하지 않았습니다.'); };
    if (!value || value.schema !== 'gooo/source-view-book/v1' || !value.metadata || !Array.isArray(value.cases)) invalid();
    const metadata = value.metadata;
    for (const key of ['compiler_revision', 'compiler_version', 'go_version']) {
      if (typeof metadata[key] !== 'string' || !metadata[key]) invalid();
    }
    safeURL(metadata.evidence_url);
    if (value.cases.length !== caseIDs.length) invalid();
    const found = new Set();
    for (const item of value.cases) {
      if (!item || !caseIDs.includes(item.id) || found.has(item.id)) invalid();
      found.add(item.id);
      for (const key of ['title', 'description', 'activity', 'source', 'stdout', 'stderr']) {
        if (typeof item[key] !== 'string') invalid();
      }
      if (!item.title || !item.activity || !item.source) invalid();
      for (const key of ['source_sha256', 'stdout_sha256', 'stderr_sha256']) {
        if (typeof item[key] !== 'string' || !hashPattern.test(item[key])) invalid();
      }
      if (!Number.isInteger(item.exit_code) || item.exit_code < 0) invalid();
      safeURL(item.source_url);
    }
    return value;
  }

  function clearCase() {
    // Clear both output streams before selecting another saved observation.
    for (const id of ['source-input', 'source-stdout', 'source-stderr', 'source-activity', 'source-case-description', 'source-input-hash', 'source-stdout-hash', 'source-stderr-hash']) {
      $(id).textContent = '';
    }
    $('source-exit').textContent = '';
    $('source-exit').hidden = true;
    $('source-exit').className = 'source-exit';
    $('source-activity-wrap').hidden = true;
    $('source-no-stdout').hidden = true;
    $('source-stdout-pre').hidden = false;
    $('source-stderr-pre').hidden = true;
    $('source-no-stderr').hidden = false;
    $('source-no-stderr').textContent = '';
    $('source-frozen-link').hidden = true;
    $('source-frozen-link').removeAttribute('href');
    buttons.forEach(button => button.setAttribute('aria-pressed', 'false'));
  }

  function showCase(id) {
    clearCase();
    const item = book?.cases.find(saved => saved.id === id);
    if (!item) throw new Error('선택한 저장 예제를 읽을 수 없습니다.');
    const index = caseIDs.indexOf(id) + 1;
    $('source-case-index').textContent = `SAVED CASE ${String(index).padStart(2, '0')} / 04`;
    $('source-case-title').textContent = item.title;
    $('source-case-description').textContent = item.description;
    $('source-activity').textContent = item.activity;
    $('source-activity-wrap').hidden = false;
    $('source-input').textContent = item.source;
    $('source-stdout').textContent = item.stdout;
    $('source-stderr').textContent = item.stderr;
    $('source-stdout-pre').hidden = item.stdout.length === 0;
    $('source-no-stdout').hidden = item.stdout.length !== 0;
    $('source-stderr-pre').hidden = item.stderr.length === 0;
    $('source-no-stderr').hidden = item.stderr.length !== 0;
    $('source-no-stderr').textContent = item.stderr.length === 0 ? '이 저장 예제의 stderr은 비어 있습니다.' : '';
    $('source-exit').textContent = `CLI exit code ${item.exit_code}`;
    $('source-exit').hidden = false;
    $('source-exit').classList.toggle('rejected', item.exit_code !== 0);
    $('source-input-hash').textContent = item.source_sha256;
    $('source-stdout-hash').textContent = item.stdout_sha256;
    $('source-stderr-hash').textContent = item.stderr_sha256;
    $('source-frozen-link').href = safeURL(item.source_url);
    $('source-frozen-link').hidden = false;
    buttons.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.sourceCase === id)));
    $('source-status').textContent = `${index} / 4 · ${item.title} · 저장된 CLI exit code ${item.exit_code}. ${item.exit_code === 0 ? '생성된 Go를 표시했습니다.' : '해당 소스의 거부 기록을 표시했습니다.'}`;
  }

  function fail(error) {
    book = undefined;
    clearCase();
    buttons.forEach(button => { button.disabled = true; });
    $('source-comparison').setAttribute('aria-busy', 'false');
    $('source-case-index').textContent = 'SAVED CASE';
    $('source-case-title').textContent = '저장 예제를 표시할 수 없습니다.';
    $('source-status').className = 'source-status error';
    $('source-status').textContent = '저장 자료를 불러오지 못했습니다. 표시된 코드가 없습니다.';
    $('source-load-error').textContent = error instanceof Error ? error.message : '저장 자료를 읽을 수 없습니다. 다시 불러와 주세요.';
    $('source-load-error').hidden = false;
    $('source-retry').hidden = false;
    $('source-evidence-link').hidden = true;
    $('source-evidence-link').removeAttribute('href');
    $('source-no-stderr').textContent = '저장 자료를 불러온 뒤 CLI 진단을 확인할 수 있습니다.';
    for (const id of ['source-compiler-version', 'source-compiler-revision', 'source-go-version', 'source-input-hash', 'source-stdout-hash', 'source-stderr-hash']) {
      $(id).textContent = '자료 없음';
    }
  }

  async function loadBook() {
    controller?.abort();
    controller = new AbortController();
    const request = controller;
    window.clearTimeout(timeout);
    book = undefined;
    clearCase();
    buttons.forEach(button => { button.disabled = true; });
    $('source-comparison').setAttribute('aria-busy', 'true');
    $('source-case-title').textContent = '저장 예제를 불러오는 중입니다.';
    $('source-status').className = 'source-status';
    $('source-status').textContent = '저장된 소스와 CLI 출력을 불러오는 중…';
    $('source-load-error').hidden = true;
    $('source-load-error').textContent = '';
    $('source-retry').hidden = true;
    timeout = window.setTimeout(() => request.abort(), 30000);
    try {
      const response = await fetch('./source-view.json', { cache: 'no-store', signal: request.signal });
      if (!response.ok) throw new Error(`저장 자료를 불러오지 못했습니다 (HTTP ${response.status}). 다시 불러와 주세요.`);
      const saved = validateBook(await response.json());
      if (request !== controller) return;
      book = saved;
      $('source-compiler-version').textContent = book.metadata.compiler_version;
      $('source-compiler-revision').textContent = book.metadata.compiler_revision;
      $('source-go-version').textContent = book.metadata.go_version;
      $('source-evidence-link').href = safeURL(book.metadata.evidence_url);
      $('source-evidence-link').hidden = false;
      showCase(caseIDs[0]);
      buttons.forEach(button => { button.disabled = false; });
      $('source-comparison').setAttribute('aria-busy', 'false');
    } catch (error) {
      if (request !== controller) return;
      fail(error?.name === 'AbortError' ? new Error('불러오기가 30초 안에 끝나지 않았습니다. 다시 불러와 주세요.') : error);
    } finally {
      if (request === controller) window.clearTimeout(timeout);
    }
  }

  buttons.forEach(button => button.addEventListener('click', () => {
    if (!book || button.disabled) return;
    try { showCase(button.dataset.sourceCase); }
    catch (error) { fail(error); }
  }));
  $('source-retry').addEventListener('click', loadBook);
  loadBook();
})();
