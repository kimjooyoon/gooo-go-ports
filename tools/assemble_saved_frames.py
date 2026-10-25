#!/usr/bin/env python3
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
"""Assemble local saved UTF-8 frames; domain validation belongs to the native CLI.

Usage: python3 -I -B assemble_saved_frames.py plan.json > observations.json

The plan uses request_file/response_file in every page instead of
request_json/response_json. Relative paths use the supplied plan path's parent,
not the working directory. Explicit absolute paths and symlinks to regular
files are allowed. Metadata and frame text are not inferred or repaired.
"""

import json
import os
from pathlib import Path
import stat
import sys

MAX_PLAN_BYTES = 1 << 20
MAX_OUTPUT_BYTES = 1 << 20
MAX_FRAME_BYTES = 64 << 10
MAX_CAPTURES = 8
MAX_PAGES = 16


class AssemblyError(Exception):
    pass


class _RawNumber:
    """A number token already accepted by json.loads, never a Python float."""

    __slots__ = ("token",)

    def __init__(self, token):
        self.token = token


def require(condition, code, location):
    if not condition:
        raise AssemblyError(f"{location}: {code}")


def read_regular(path, maximum, location):
    """Bound the read and reject FIFOs/devices after a nonblocking open."""
    fd = None
    try:
        fd = os.open(os.fspath(path), os.O_RDONLY | getattr(os, "O_NONBLOCK", 0))
        info = os.fstat(fd)
        require(stat.S_ISREG(info.st_mode), "REGULAR_FILE_REQUIRED", location)
        require(info.st_size <= maximum, "BYTE_LIMIT", location)
        with os.fdopen(fd, "rb") as source:
            fd = None
            raw = source.read(maximum + 1)
        require(len(raw) <= maximum, "BYTE_LIMIT", location)
        return raw
    except OSError as error:
        raise AssemblyError(f"{location}: FILE_READ_FAILED ({type(error).__name__})") from error
    finally:
        if fd is not None:
            os.close(fd)


def unique_pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "DUPLICATE_PLAN_KEY", "plan")
        result[key] = value
    return result


def reject_constant(_value):
    raise AssemblyError("plan: NONFINITE_JSON_NUMBER")


def load_plan(path):
    raw = read_regular(path, MAX_PLAN_BYTES, "plan")
    try:
        return json.loads(
            raw.decode("utf-8", "strict"),
            object_pairs_hook=unique_pairs,
            parse_constant=reject_constant,
            parse_int=_RawNumber,
            parse_float=_RawNumber,
        )
    except (UnicodeError, ValueError, RecursionError) as error:
        raise AssemblyError("plan: INVALID_UTF8_OR_JSON") from error


def frame_text(base, reference, location):
    require(isinstance(reference, str) and reference != "", "FILE_PATH_STRING_REQUIRED", location)
    require("\x00" not in reference, "NUL_IN_FILE_PATH", location)
    path = Path(reference)
    if not path.is_absolute():
        path = base / path
    raw = read_regular(path, MAX_FRAME_BYTES, location)
    try:
        return raw.decode("utf-8", "strict")
    except UnicodeError as error:
        raise AssemblyError(f"{location}: INVALID_FRAME_UTF8") from error


def encode_bounded(value):
    # Re-encode only the outer plan. Decoding this JSON restores the exact frame
    # strings, including CRLF, whitespace, escapes, NUL and non-ASCII scalars.
    # Only json.loads parse_int/parse_float create _RawNumber instances. Its JSON
    # number grammar has already validated their tokens; do not parse or round
    # them again. Strings and object keys still use the standard JSON encoder.
    result = bytearray()

    def append(raw):
        require(len(result) + len(raw) + 1 <= MAX_OUTPUT_BYTES, "BYTE_LIMIT", "output")
        result.extend(raw)

    def encode(item):
        if item is None:
            append(b"null")
        elif type(item) is bool:
            append(b"true" if item else b"false")
        elif type(item) is _RawNumber:
            append(item.token.encode("ascii"))
        elif type(item) is str:
            append(json.dumps(item, ensure_ascii=True).encode("utf-8"))
        elif type(item) is list:
            append(b"[")
            for index, member in enumerate(item):
                if index:
                    append(b",")
                encode(member)
            append(b"]")
        elif type(item) is dict:
            append(b"{")
            for index, (key, member) in enumerate(item.items()):
                require(type(key) is str, "OBJECT_KEY_STRING_REQUIRED", "output")
                if index:
                    append(b",")
                append(json.dumps(key, ensure_ascii=True).encode("utf-8"))
                append(b":")
                encode(member)
            append(b"}")
        else:
            raise AssemblyError("output: UNEXPECTED_INTERNAL_VALUE_TYPE")

    try:
        encode(value)
    except (ValueError, UnicodeError, RecursionError) as error:
        raise AssemblyError("output: JSON_ENCODING_FAILED") from error
    result.extend(b"\n")
    return bytes(result)


def assemble(plan_path):
    path = Path(plan_path).absolute()
    plan = load_plan(path)
    require(isinstance(plan, dict), "OBJECT_REQUIRED", "plan")
    for side in ("before", "after"):
        observation = plan.get(side)
        require(isinstance(observation, dict), "OBJECT_REQUIRED", side)
        captures = observation.get("captures")
        require(isinstance(captures, list), "ARRAY_REQUIRED", f"{side}.captures")
        require(len(captures) <= MAX_CAPTURES, "CAPTURE_LIMIT", f"{side}.captures")
        for ci, capture in enumerate(captures):
            label = f"{side}.captures[{ci}]"
            require(isinstance(capture, dict), "OBJECT_REQUIRED", label)
            pages = capture.get("pages")
            require(isinstance(pages, list), "ARRAY_REQUIRED", f"{label}.pages")
            require(len(pages) <= MAX_PAGES, "PAGE_LIMIT", f"{label}.pages")
            for pi, page in enumerate(pages):
                at = f"{label}.pages[{pi}]"
                require(isinstance(page, dict), "OBJECT_REQUIRED", at)
                require(
                    "request_file" in page
                    and "response_file" in page
                    and "request_json" not in page
                    and "response_json" not in page,
                    "FILE_PAIR_REQUIRED_NO_INLINE_MIXING",
                    at,
                )
                request = frame_text(path.parent, page["request_file"], f"{at}.request_file")
                response = frame_text(path.parent, page["response_file"], f"{at}.response_file")
                replaced = {}
                for key, value in page.items():
                    if key == "request_file":
                        replaced["request_json"] = request
                    elif key == "response_file":
                        replaced["response_json"] = response
                    else:
                        replaced[key] = value
                pages[pi] = replaced
    # No stdout is touched until every required frame has been read and the
    # complete bounded output encoded. Context/Watch fields, enums, aliases,
    # RPC structure and inner duplicate keys are left to the native importer.
    return encode_bounded(plan)


def emit(raw, output):
    offset = 0
    while offset < len(raw):
        written = output.write(raw[offset:])
        require(type(written) is int and 0 < written <= len(raw) - offset, "STDOUT_WRITE_FAILED", "output")
        offset += written
    output.flush()


def run(arguments, output, diagnostics):
    try:
        require(len(arguments) == 1, "ONE_PLAN_PATH_REQUIRED", "usage")
        raw = assemble(arguments[0])
        emit(raw, output)
        return 0
    except (AssemblyError, OSError, ValueError, UnicodeError, RecursionError) as error:
        message = str(error) if isinstance(error, AssemblyError) else f"output: IO_FAILED ({type(error).__name__})"
        try:
            diagnostics.write((message + "\n").encode("utf-8", "backslashreplace"))
            diagnostics.flush()
        except (OSError, ValueError):
            pass
        return 1


class FileDescriptorOutput:
    """Avoid a second buffered stdout flush after an actual broken-pipe error."""

    def write(self, raw):
        return os.write(1, raw)

    def flush(self):
        pass


if __name__ == "__main__":
    raise SystemExit(run(sys.argv[1:], FileDescriptorOutput(), sys.stderr.buffer))
