import unittest

import oci_action_contract as contract


class ActionContractTests(unittest.TestCase):
    expected = 'actions/attest@' + 'a' * 40

    def test_inline_code_and_comments_are_not_action_steps(self):
        text = '''      - name: Check
        run: |
          text = "uses: actions/attest@not-a-step"
          # uses: actions/attest@also-not-a-step
      - name: Attest
        uses: %s # pinned
''' % self.expected
        self.assertEqual(contract.check(text, self.expected, 1), [self.expected])

    def test_wrong_ref_cannot_hide_behind_a_correct_comment(self):
        text = '      - uses: actions/attest@changed # ' + self.expected
        with self.assertRaisesRegex(ValueError, 'CONTRACT_MISMATCH'):
            contract.check(text, self.expected, 1)

    def test_missing_or_extra_real_step_is_refused(self):
        for text in ['', '      - uses: ' + self.expected + '\n      - uses: ' + self.expected]:
            with self.assertRaises(ValueError):
                contract.check(text, self.expected, 1)

    def test_actual_workflows_match_the_reviewed_dependency(self):
        contract.main()


if __name__ == '__main__':
    unittest.main()
