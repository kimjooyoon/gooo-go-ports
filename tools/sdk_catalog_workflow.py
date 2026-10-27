"""An exact additive CI layer; removing it must recover the reviewed workflow."""
import hashlib

PRIOR_SHA256 = '6b0728d9410ddae590cb76d652b5a320234771a1caf79ab8b4288b3dfb5e6855'
COMMON = """          go -C examples/sdk-catalog-refresh vet ./...
          go -C examples/sdk-catalog-refresh test -race -count=1 ./...
          python3 -I -B tools/sdk_catalog_workflow_test.py -v
"""
DECLARATION = '          ./.gooo check ports/tool-origin/sdk_row_visibility.gooo\n'
BUILD_VERIFY = """      - name: Build independent SDK row library compiler
        if: steps.change-scope.outputs.docs_only != 'true'
        timeout-minutes: 8
        run: |
          set -euo pipefail
          python3 -I -B examples/sdk-catalog-refresh/bootstrap.py --go "$(go env GOROOT)/bin/go" --out "$RUNNER_TEMP/sdk-catalog-profile"
          jq -e '.accepted == true and .first_failure == null and .global_pin_changed == false and .clean_checkout_before_and_after == true and .all_direct_wait_EOF_PG_ESRCH == true and .profile.compiler_revision == "1afb98b955fd8dea74650d3b24e693d0dab9eb5d"' "$RUNNER_TEMP/sdk-catalog-profile/summary.json"
      - name: Verify original Gooo SDK row correspondence
        if: steps.change-scope.outputs.docs_only != 'true'
        timeout-minutes: 5
        run: |
          set -euo pipefail
          python3 -I -B examples/sdk-catalog-refresh/verify.py --compiler "$RUNNER_TEMP/sdk-catalog-profile/gooo" --go "$(go env GOROOT)/bin/go" --go-cache "$(go env GOCACHE)" --module-cache "$(go env GOMODCACHE)" --out "$RUNNER_TEMP/sdk-catalog-refresh"
          jq -e '
            .schema == "gooo/sdk-catalog-refresh-verification/v1" and
            .status == "FINITE_SDK_ROW_NATIVE_OWN_REPLAY_AND_LIBRARY_PASS" and
            has("first_failure") and .first_failure == null and
            .compiler_revision == "1afb98b955fd8dea74650d3b24e693d0dab9eb5d" and
            .native_cases == 8 and .replay_cases == 8 and .receipt_negative_controls == 8 and
            .typed_input_refusals == 2 and
            .ordered_inputs_verified == true and .generated_origin_verified == true and
            .generated_assets_exact == true and .source_unchanged == true and
            .initial_bootstrap == false and (.generated_write_records | length) == 0 and
            .global_compiler_pin_changed == false and .native_acceptance == true and
            .all_direct_wait_EOF_PG_ESRCH == true and
            .model_calls == 0 and .network_calls == 0 and .engine_builds == 0 and .external_adoption == false
          ' "$RUNNER_TEMP/sdk-catalog-refresh/summary.json"
"""
UPLOAD = """      - name: Preserve SDK row correspondence evidence
        if: always() && steps.change-scope.outputs.docs_only != 'true'
        uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7
        with:
          name: sdk-catalog-refresh-evidence
          path: |
            @RUNNER@/sdk-catalog-refresh/
            @RUNNER@/sdk-catalog-profile/summary.json
            @RUNNER@/sdk-catalog-profile/first-failure.json
            @RUNNER@/sdk-catalog-profile/*/stdout
            @RUNNER@/sdk-catalog-profile/*/stderr
            @RUNNER@/sdk-catalog-profile/*/process.json
          if-no-files-found: warn
          retention-days: 14
""".replace('@RUNNER@', '$'+'{{ runner.temp }}')
COMMON_ANCHOR = '          go -C examples/catalog-refresh test -race -count=1 ./...\n'
DECLARATION_ANCHOR = '          ./.gooo check ports/image/color.gooo\n'
STEP_ANCHOR = '      - name: Verify Gooo recorded observation diagnostics\n'


def project(text):
    for chunk in [COMMON, DECLARATION, BUILD_VERIFY, UPLOAD]:
        if text.count(chunk) != 1:
            raise ValueError('SDK catalog CI block missing, duplicated or changed')
    if COMMON_ANCHOR+COMMON not in text or DECLARATION+DECLARATION_ANCHOR not in text:
        raise ValueError('SDK catalog common checks moved or made conditional')
    if BUILD_VERIFY+UPLOAD+STEP_ANCHOR not in text:
        raise ValueError('SDK native verification and failure evidence moved')
    prior = text
    for chunk in [COMMON, DECLARATION, BUILD_VERIFY, UPLOAD]:
        prior = prior.replace(chunk, '', 1)
    if hashlib.sha256(prior.encode()).hexdigest() != PRIOR_SHA256:
        raise ValueError('SDK catalog integration changed unrelated workflow bytes')
    return prior
