import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('cel_generator',ROOT/'generate.py')
generator=importlib.util.module_from_spec(spec);spec.loader.exec_module(generator)

class BodyReceiptComparison(unittest.TestCase):
    def test_only_process_observations_are_normalized(self):
        original=json.loads((ROOT/'receipts/api-body-codegen.json').read_bytes())
        other=copy.deepcopy(original)
        other['report']['route_decision_latency_ms']=0.5
        other['report']['completeness_receipt']['scope']['execution_environment']='linux/amd64'
        self.assertEqual(generator.stable(original),generator.stable(other))
        for field,value in [('output_type','ChangedRecord'),('compiler_source_sha','0'*40),('source_digest','sha256:bad'),('generated_digest','sha256:bad')]:
            modified=copy.deepcopy(other);modified['report'][field]=value
            self.assertNotEqual(generator.stable(original),generator.stable(modified),field)
        modified=copy.deepcopy(other);modified['source']+='\n'
        self.assertNotEqual(generator.stable(original),generator.stable(modified))
    def test_malformed_process_observation_is_rejected(self):
        original=json.loads((ROOT/'receipts/api-body-codegen.json').read_bytes())
        for environment in ['',None,'linux','linux/']:
            other=copy.deepcopy(original);other['report']['completeness_receipt']['scope']['execution_environment']=environment
            with self.assertRaises(AssertionError):generator.stable(other)

if __name__=='__main__':unittest.main()
