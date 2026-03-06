#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
PreToolUse Hook: Enforce output directory constraint.

Intercepts Edit/Write/MultiEdit tool calls and verifies the target
file path is within the developer's allowed output directory.

Non-collaboration projects (no roles.json) are unaffected.
Fail-open: any infrastructure error results in allow.
"""

# IMPORTANT: Suppress all warnings FIRST
import warnings
warnings.filterwarnings("ignore")

import json
import os
import sys
from pathlib import Path

# IMPORTANT: Force stdout to use UTF-8 on Windows
if sys.platform == "win32":
    import io as _io
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")  # type: ignore[union-attr]
    elif hasattr(sys.stdout, "detach"):
        sys.stdout = _io.TextIOWrapper(sys.stdout.detach(), encoding="utf-8", errors="replace")  # type: ignore[union-attr]


def _allow():
    """Output allow decision and exit."""
    print(json.dumps({"decision": "allow"}))


def _block(reason: str):
    """Output block decision with reason and exit."""
    print(json.dumps({"decision": "block", "reason": reason}))


def main():
    # Read hook event from stdin
    try:
        event = json.loads(sys.stdin.read())
    except (json.JSONDecodeError, EOFError, OSError):
        _allow()
        return

    tool_name = event.get("tool_name", "")
    tool_input = event.get("tool_input", {})

    # Only intercept file-writing tools
    if tool_name not in ("Edit", "Write", "MultiEdit"):
        _allow()
        return

    # Get file path from tool input
    file_path = tool_input.get("file_path", "")
    if not file_path:
        _allow()
        return

    # Resolve role constraint via .trellis/scripts/common/roles.py
    try:
        project_dir = Path(os.environ.get("CLAUDE_PROJECT_DIR", ".")).resolve()
        scripts_parent = project_dir / ".trellis"
        if scripts_parent.is_dir():
            sys.path.insert(0, str(scripts_parent))

        from scripts.common.roles import resolve_role_constraint
        role, output_dir = resolve_role_constraint(project_dir)
    except Exception:
        # Fail-open: can't resolve constraint -> allow
        _allow()
        return

    # No constraint = non-collaboration project, allow everything
    if not role or not output_dir:
        _allow()
        return

    # Normalize paths for comparison
    try:
        resolved_file = Path(file_path).resolve()
        resolved_output = (project_dir / output_dir).resolve()
        resolved_trellis = (project_dir / ".trellis").resolve()
    except Exception:
        _allow()
        return

    # Allow writes to .trellis/ (workflow files are always writable)
    try:
        resolved_file.relative_to(resolved_trellis)
        _allow()
        return
    except ValueError:
        pass

    # Check if file is within the allowed output directory
    try:
        resolved_file.relative_to(resolved_output)
        _allow()
        return
    except ValueError:
        pass

    # Block: file is outside allowed directory
    _block(
        f"Role '{role}' can only write to '{output_dir}' and '.trellis/'. "
        f"Target file '{file_path}' is outside the allowed directory."
    )


if __name__ == "__main__":
    main()
