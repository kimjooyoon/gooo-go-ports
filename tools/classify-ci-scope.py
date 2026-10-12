#!/usr/bin/env python3
"""Classify the checked PR merge tree; never infer native execution from scope."""

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path


DOC_PATHS = frozenset((b"README.md", b"CONTRIBUTING.md"))
SHA = re.compile(r"[0-9a-f]{40}\Z")
RAW_HEADER = re.compile(
    rb":([0-7]{6}) ([0-7]{6}) ([0-9a-f]{40}) ([0-9a-f]{40}) ([A-Z])\Z"
)


class ScopeError(ValueError):
    pass


def git(repo, *args):
    try:
        result = subprocess.run(
            ["git", "-C", str(repo), *args], capture_output=True, check=False
        )
    except OSError as error:
        raise ScopeError("cannot execute Git") from error
    if result.returncode:
        raise ScopeError("Git inspection failed: " + args[0])
    return result.stdout


def commit_sha(value, label):
    if not isinstance(value, str) or not SHA.fullmatch(value):
        raise ScopeError(label + " must be a full lowercase commit SHA")
    return value


def parse_raw_diff(raw):
    """Read --raw --no-abbrev -z --no-renames output without line splitting."""
    if not raw:
        return []
    fields = raw.split(b"\0")
    if fields.pop() != b"" or len(fields) % 2:
        raise ScopeError("malformed NUL-delimited Git diff")
    changes = []
    seen = set()
    for index in range(0, len(fields), 2):
        match = RAW_HEADER.fullmatch(fields[index])
        path = fields[index + 1]
        if not match or not path or path in seen:
            raise ScopeError("malformed or duplicate Git diff entry")
        seen.add(path)
        old_mode, new_mode, old_oid, new_oid, status = match.groups()
        changes.append({
            "path": path.decode("utf-8", errors="backslashreplace"),
            "old_mode": old_mode.decode("ascii"),
            "new_mode": new_mode.decode("ascii"),
            "old_oid": old_oid.decode("ascii"),
            "new_oid": new_oid.decode("ascii"),
            "status": status.decode("ascii"),
            "allowed_path": path in DOC_PATHS,
        })
    return changes


def classify(repo, event_name, event, expected_head):
    expected_head = commit_sha(expected_head, "expected checkout HEAD")
    actual_head = git(repo, "rev-parse", "--verify", "HEAD^{commit}").decode().strip()
    if actual_head != expected_head:
        raise ScopeError("actual checkout HEAD differs from the workflow SHA")
    if git(repo, "status", "--porcelain=v1", "-z", "--untracked-files=all"):
        raise ScopeError("checkout has tracked or untracked changes before classification")
    result = {
        "schema": "gooo/ci-change-scope/v1",
        "event_name": event_name,
        "tested_head": actual_head,
        "tested_tree": git(repo, "rev-parse", actual_head + "^{tree}").decode().strip(),
        "docs_only": False,
        "status": "FULL_REQUIRED",
        "reason": "non_pull_request_event",
        "changes": [],
        "scope": "Change classification only; no native execution or receipt verification.",
    }
    if event_name != "pull_request":
        return result
    try:
        pr = event["pull_request"]
        base = commit_sha(pr["base"]["sha"], "PR base")
        pr_head = commit_sha(pr["head"]["sha"], "PR head")
    except (KeyError, TypeError) as error:
        raise ScopeError("missing PR base/head identities") from error
    for label, revision in (("base", base), ("head", pr_head)):
        resolved = git(repo, "rev-parse", "--verify", revision + "^{commit}").decode().strip()
        if resolved != revision:
            raise ScopeError("PR " + label + " is not the supplied commit")
    result.update(base=base, pr_head=pr_head)
    parents = git(repo, "rev-list", "--parents", "-n", "1", actual_head).decode().split()
    if not parents or parents[0] != actual_head:
        raise ScopeError("cannot inspect checkout parents")
    result["checkout_parents"] = parents[1:]
    if parents[1:] != [base, pr_head]:
        result["reason"] = "checkout_is_not_the_exact_pr_merge"
        return result
    result["base_tree"] = git(repo, "rev-parse", base + "^{tree}").decode().strip()
    # Comparing two committed trees cannot omit untracked names or depend on
    # rename heuristics, line endings, textconv, or a mutable working directory.
    changes = parse_raw_diff(git(
        repo, "diff", "--raw", "--no-abbrev", "-z", "--no-renames",
        "--no-ext-diff", "--no-textconv", base, actual_head, "--"
    ))
    result["changes"] = changes
    if not changes:
        result["reason"] = "empty_tree_delta"
        return result
    for change in changes:
        if not change["allowed_path"]:
            result["reason"] = "path_outside_root_document_allowlist"
            return result
        if (change["status"] != "M" or change["old_mode"] != "100644"
                or change["new_mode"] != "100644"
                or change["old_oid"] == change["new_oid"]
                or "0" * 40 in (change["old_oid"], change["new_oid"])):
            result["reason"] = "not_existing_regular_document_content_only"
            return result
    result.update(docs_only=True, status="DOCS_ONLY_CONTENT_CHANGE",
                  reason="exact_root_document_content_delta")
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path("."))
    parser.add_argument("--event-name", required=True)
    parser.add_argument("--event-path", type=Path, required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--summary", type=Path, required=True)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args(argv)
    exit_code = 0
    try:
        # Push and tag events always require the full path, without a PR payload.
        event = json.loads(args.event_path.read_text()) if args.event_name == "pull_request" else {}
        result = classify(args.repo, args.event_name, event, args.expected_head)
    except (ScopeError, OSError, ValueError, UnicodeError) as error:
        result = {"schema": "gooo/ci-change-scope/v1", "docs_only": False,
                  "status": "CLASSIFICATION_FAILED", "error": str(error),
                  "scope": "No successful scope classification; native verification is not established."}
        exit_code = 1
    raw = json.dumps(result, indent=2, sort_keys=True) + "\n"
    args.summary.parent.mkdir(parents=True, exist_ok=True)
    args.summary.write_text(raw)
    if args.github_output:
        with args.github_output.open("a") as stream:
            stream.write("docs_only=" + ("true" if result["docs_only"] else "false") + "\n")
    sys.stdout.write(raw)
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
