"""Adversarial synthetic loader tests; no production expected data or CI runs."""
import copy
import json
from pathlib import Path
import sys
import subprocess
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
import native_verification_contract as C

PREFIX = C.FrozenGuard("a" * 40, 2, "sha256:56c72db9fea353441caa2dadee5a5cf44cf34f09094c192dcda48bfbe4639c64")
SOURCES = C.FrozenGuard("b" * 40, 2, "sha256:83e811c8e6d05c680ab07ec103a03aff3f3549fa195ba16bb355c1fe2a353429")


def fixture():
    return {
        "schema": C.SCHEMA, "profiles": dict(C.PROFILES),
        "prefix": dict(commit=PREFIX.commit, count=PREFIX.count, sha256=PREFIX.sha256),
        "suites": [
            dict(index=0, package="go/a", activity="First", cases=3, batches=2, source_paths=["ports/a.gooo"]),
            dict(index=1, package="go/b", activity="Second", cases=5, batches=3, source_paths=["ports/b.gooo"]),
            dict(index=2, package="app/c", activity="Third", cases=2, batches=1, source_paths=["ports/a.gooo", "ports/b.gooo"])],
        "source_baseline": dict(commit=SOURCES.commit, count=SOURCES.count, sha256=SOURCES.sha256),
        "sources": [
            dict(path="ports/a.gooo", mode="100644", owner="native-ci", roles=["catalog-source"], baseline=True),
            dict(path="ports/b.gooo", mode="100755", owner="native-ci", roles=["catalog-source"], baseline=True),
            dict(path="tools/reader.go", mode="100644", owner="native-ci", roles=["contract-reader", "explicit-observer-input"], baseline=False)]}


def wire(doc):
    return json.dumps(doc, ensure_ascii=False, separators=(",", ":")).encode()


class ContractTests(unittest.TestCase):
    rejection_checks = 0

    def reject(self, raw, message="", prefix=PREFIX, sources=SOURCES):
        type(self).rejection_checks += 1
        with self.assertRaisesRegex(C.ContractError, message):
            C.load_contract(raw, prefix, sources)

    def mutation(self, mutate, message=""):
        doc = fixture()
        mutate(doc)
        self.reject(wire(doc), message)

    @classmethod
    def tearDownClass(cls):
        print("synthetic rejection checks:", cls.rejection_checks)

    def test_synthetic_result_and_canonical_vectors(self):
        doc = fixture()
        result = C.load_contract(wire(doc), PREFIX, SOURCES)
        self.assertEqual((result.suite_count, result.total_cases, result.total_batches,
                          result.timing_events, result.source_count, result.baseline_count), (3, 10, 6, 14, 3, 2))
        self.assertEqual(C._digest(doc["suites"][:2]), PREFIX.sha256)
        self.assertEqual(C._digest([dict(path="ports/a.gooo", mode="100644"),
                                    dict(path="ports/b.gooo", mode="100755")]), SOURCES.sha256)
        # Input object ordering is irrelevant; canonical suite field order is fixed.
        C.load_contract(json.dumps(doc, sort_keys=True).encode(), PREFIX, SOURCES)
        doc["sources"][2]["mode"] = "100755"
        C.load_contract(wire(doc), PREFIX, SOURCES)

    def test_html_canonical_vector(self):
        value = [dict(index=0, package="go/<&>", activity="Check", cases=1, batches=1,
                      source_paths=["ports/<&>.gooo"])]
        self.assertEqual(C._digest(value), "sha256:eeb901b3c57b40ee095a555f76e72ff041ebca76afbe5b21108683e546d9b8d5")

    def test_duplicate_decoded_keys_and_json_envelope(self):
        raw = wire(fixture())
        inputs = {
            "duplicate-root": raw.replace(b'"schema":', b'"schema":"x","schema":', 1),
            "escaped-duplicate": raw.replace(b'"schema":', b'"sche\\u006da":"x","schema":', 1),
            "nested-duplicate": raw.replace(b'"case":', b'"case":"x","case":', 1),
            "row-duplicate": raw.replace(b'"index":0', b'"index":0,"index":0', 1),
            "trailing-object": raw + b" {}", "trailing-junk": raw + b"!",
            "invalid-utf8": raw.replace(b"go/a", b"go/\xff", 1),
            "empty": b"", "null": b"null", "array": b"[]",
            "deep-invalid": b"[" * 2000 + b"]" * 2000,
            "nonfinite": raw.replace(b'"cases":3', b'"cases":NaN', 1)}
        for name, value in inputs.items():
            with self.subTest(name=name): self.reject(value)
        padded = raw + b" " * (C.MAX_BYTES - len(raw))
        C.load_contract(padded, PREFIX, SOURCES)
        self.reject(padded + b" ", "1MiB")

    def test_unknown_fields_at_every_level(self):
        for name in ("root", "profiles", "prefix", "source_baseline", "suite", "source", "case-sensitive"):
            doc = fixture()
            target = doc if name in ("root", "case-sensitive") else (
                doc["suites"][0] if name == "suite" else doc["sources"][0] if name == "source" else doc[name])
            target["Schema" if name == "case-sensitive" else "unknown"] = True
            with self.subTest(name=name): self.reject(wire(doc), "fields")

    def test_exact_integer_counts_and_checked_totals(self):
        for key in ("index", "cases", "batches"):
            for value in (True, False, 1.0, -1, 1 << 63, "1", None):
                doc = fixture(); doc["suites"][2][key] = value
                with self.subTest(key=key, value=value): self.reject(wire(doc))
        raw = wire(fixture())
        for token in (b"1e0", b"1E+0", b"1.0"):
            with self.subTest(token=token): self.reject(raw.replace(b'"cases":3', b'"cases":' + token, 1))
        self.reject(raw.replace(b'"index":0', b'"index":-0', 1))
        for key in ("cases", "batches"):
            self.mutation(lambda d, k=key: d["suites"][2].update({k: 0}))
        self.mutation(lambda d: d["suites"][2].update(cases=C.MAX_INT64), "overflow")
        self.mutation(lambda d: d["suites"][2].update(cases=C.MAX_INT64-8, batches=C.MAX_INT64-8), "overflow")
        self.mutation(lambda d: d["suites"][2].update(batches=3), "batches")
        for guard in ("prefix", "source_baseline"):
            for value in (True, 2.0):
                self.mutation(lambda d, g=guard, v=value: d[g].update(count=v))

    def test_external_guards_are_authority(self):
        for guard in ("prefix", "source_baseline"):
            for key, value in (("commit", "c" * 40), ("count", 1), ("sha256", "sha256:" + "0" * 64)):
                with self.subTest(guard=guard, key=key):
                    self.mutation(lambda d, g=guard, k=key, v=value: d[g].update({k: v}), "external guard")
        for bad in (C.FrozenGuard("bad", 2, PREFIX.sha256), C.FrozenGuard(PREFIX.commit, 0, PREFIX.sha256),
                    C.FrozenGuard(PREFIX.commit, True, PREFIX.sha256), C.FrozenGuard(PREFIX.commit, 2, "0" * 64)):
            self.reject(wire(fixture()), prefix=bad)

    def test_same_totals_prefix_and_source_order_are_frozen(self):
        doc=fixture(); doc["suites"][0]["cases"]=4; doc["suites"][1]["cases"]=4
        self.reject(wire(doc), "prefix digest")
        doc=fixture(); doc["suites"][0],doc["suites"][1]=doc["suites"][1],doc["suites"][0]
        self.reject(wire(doc), "index")
        doc["suites"][0]["index"]=0; doc["suites"][1]["index"]=1
        self.reject(wire(doc), "prefix digest")
        self.mutation(lambda d: d["suites"][0].update(source_paths=["ports/b.gooo"]), "prefix digest")

    def test_source_baseline_modes_and_membership(self):
        mutations = {
            "missing-baseline": lambda d: d["sources"].pop(1),
            "swap-baseline-flags": lambda d: (d["sources"][1].update(baseline=False),d["sources"][2].update(baseline=True)),
            "same-count-path-substitute": lambda d: d["sources"][1].update(path="ports/c.gooo"),
            "executable-bit-flip": lambda d: d["sources"][1].update(mode="100644"),
            "forged-mode": lambda d: d["sources"][0].update(mode="100755"),
            "symlink-mode": lambda d: d["sources"][2].update(mode="120000"),
            "directory-mode": lambda d: d["sources"][2].update(mode="040000"),
            "duplicate-path": lambda d: d["sources"][1].update(path="ports/a.gooo"),
            "unknown-role": lambda d: d["sources"][2].update(roles=["unknown"]),
            "duplicate-role": lambda d: d["sources"][2].update(roles=["contract-reader"]*2),
            "unsorted-role": lambda d: d["sources"][2].update(roles=["explicit-observer-input","contract-reader"]),
            "empty-role": lambda d: d["sources"][2].update(roles=[]),
            "lost-catalog-qualification": lambda d: d["sources"][0].update(roles=["contract-reader"]),
            "owner": lambda d: d["sources"][2].update(owner="other"),
            "not-boolean": lambda d: d["sources"][2].update(baseline=0),
            "missing-suite-source": lambda d: d["suites"][2].update(source_paths=["ports/missing.gooo"]),
            "unqualified-suite-source": lambda d: d["suites"][2].update(source_paths=["tools/reader.go"]),
            "duplicate-suite-source": lambda d: d["suites"][2].update(source_paths=["ports/a.gooo"]*2),
            "empty-suite-source": lambda d: d["suites"][2].update(source_paths=[]),
            "duplicate-suite-key": lambda d: d["suites"][2].update(package="go/a",activity="First"),
            "activity": lambda d: d["suites"][2].update(activity="é"),
            "profile": lambda d: d["profiles"].update(case="legacy")}
        for name, mutation in mutations.items():
            with self.subTest(name=name): self.mutation(mutation)
        doc=fixture();doc["sources"][0],doc["sources"][1]=doc["sources"][1],doc["sources"][0]
        self.reject(wire(doc), "sorted")

    def test_unsafe_paths_rejected_without_filesystem_access(self):
        for path in ("", "/abs", "a/../b", "a/./b", "../b", "a//b", "a/", "a\\b", "a:b", "a\0b", "a\x7fb", "é", "a\nb", "a\u2028b"):
            for kind in ("source", "package"):
                doc=fixture()
                (doc["sources"][2] if kind=="source" else doc["suites"][2])["path" if kind=="source" else "package"]=path
                with self.subTest(kind=kind,path=path): self.reject(wire(doc))


class ReviewedProductionWrapperTests(unittest.TestCase):
    ROOT=Path(__file__).resolve().parents[1]

    def test_reviewed_projection_is_definition_only_and_preserves_all_rows(self):
        result=C.load_reviewed_contract(self.ROOT)
        projected=C.expected_projection(result)
        self.assertEqual(set(projected),{'schema','profiles','totals','prefix','suites','source_paths'})
        self.assertEqual(projected['schema'],'gooo/native-verification-expected/v1')
        self.assertEqual(projected['totals'],dict(suites=82,cases=175649,batches=2305,timing_records=2471,source_paths=288))
        self.assertEqual(projected['prefix'],dict(count=80,cases=173866,batches=2252))
        self.assertEqual(projected['suites'],result.document['suites'])
        self.assertEqual(projected['suites'][73]['source_paths'],[
            'ports/net/httpguts/headers.gooo','examples/header-field-facts/main.gooo',
            'ports/strconv/decimal.gooo','examples/payload-admission/main.gooo',
            'examples/diagnostic-header-addition/main.gooo'])
        self.assertEqual(projected['source_paths'],sorted(projected['source_paths']))
        self.assertTrue(set(C.CONTRACT_FILES).issubset(projected['source_paths']))

    def test_reviewed_raw_guard_rejects_valid_generic_rollback_and_lost_readers(self):
        original=C.load_reviewed_contract(self.ROOT)
        mutations={
            'valid-old80':lambda d:d.update(suites=d['suites'][:80]),
            'remove-new-reader-paths':lambda d:d.update(sources=[r for r in d['sources'] if r['baseline']]),
            'suffix-same-total-rebalance':lambda d:(d['suites'][80].update(cases=1693),d['suites'][81].update(cases=90)),
        }
        for name,mutate in mutations.items():
            doc=copy.deepcopy(original.document);mutate(doc)
            # These are intentionally valid under the generic historical guards;
            # the production full-document guard must still refuse all of them.
            C.load_contract(wire(doc),C.REVIEWED_PREFIX,C.REVIEWED_SOURCES)
            with tempfile.TemporaryDirectory() as directory:
                root=Path(directory);path=root/C.REVIEWED_PATH
                path.parent.mkdir(parents=True);path.write_bytes(wire(doc))
                with self.subTest(mutation=name),self.assertRaisesRegex(C.ContractError,'full contract raw digest'):
                    C.load_reviewed_contract(root)

    def test_reviewed_wrapper_retains_the_exact_read_input(self):
        calls=[]
        def read(path):calls.append(path);return path.read_bytes()
        C.load_reviewed_contract(self.ROOT,read_bytes=read)
        self.assertEqual(calls,[self.ROOT/C.REVIEWED_PATH])

    def test_cli_isolated_export_and_fail_closed_empty_stdout(self):
        command=[sys.executable,'-I','-B',str(Path(C.__file__).resolve()),'--root']
        actual=subprocess.run(command+[str(self.ROOT)],capture_output=True,check=False)
        self.assertEqual(actual.returncode,0,actual.stderr)
        self.assertEqual(actual.stderr,b'')
        self.assertEqual(json.loads(actual.stdout),C.expected_projection(C.load_reviewed_contract(self.ROOT)))
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);path=root/C.REVIEWED_PATH
            for name in ('missing-data','changed-data','valid-old80'):
                if name!='missing-data':
                    path.parent.mkdir(parents=True,exist_ok=True)
                    doc=copy.deepcopy(C.load_reviewed_contract(self.ROOT).document)
                    if name=='valid-old80':doc['suites']=doc['suites'][:80]
                    else:doc['suites'][81]['activity']='Substituted'
                    path.write_bytes(wire(doc))
                failed=subprocess.run(command+[str(root)],capture_output=True,check=False)
                with self.subTest(failure=name):
                    self.assertEqual(failed.returncode,1)
                    self.assertEqual(failed.stdout,b'')
                    self.assertTrue(failed.stderr)
            failed=subprocess.run(command+[str(root/'missing-root')],capture_output=True,check=False)
            self.assertEqual((failed.returncode,failed.stdout),(1,b''))


if __name__ == "__main__":
    unittest.main()
