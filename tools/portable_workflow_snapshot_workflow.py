"""Project only the exact portable-snapshot test addition from the reviewed CI."""
import hashlib


PRIOR_SOURCE_COMMIT = 'bcdf10b82d0a55b2f3c919c0d1689a9739312676'
PRIOR_SHA256 = '2c10fbc293b18563220c9e7fb02e77f3a25210b1eb73f21a92d5bdbd2d6a08f0'
STEP = """      - name: Test portable workflow source snapshots
        run: python3 -I -B tools/package-gooo-workflows_test.py -v
"""
BEFORE = """      - name: Check current generation toolchain documentation
        run: |
          python3 -I -B tools/current-toolchain-docs_test.py -v
          python3 -I -B tools/current-toolchain-docs.py --check
"""
AFTER = '      - name: Check frozen reader profiles before builds\n'


def project(text):
    if text.count(STEP) != 1:
        raise ValueError('portable snapshot CI step missing, duplicated or changed')
    if text.count(BEFORE + STEP + AFTER) != 1:
        raise ValueError('portable snapshot CI step moved from the reviewed observer position')
    prior = text.replace(STEP, '', 1)
    if hashlib.sha256(prior.encode()).hexdigest() != PRIOR_SHA256:
        raise ValueError('portable snapshot integration changed unrelated workflow bytes')
    return prior
