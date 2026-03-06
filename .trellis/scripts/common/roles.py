#!/usr/bin/env python3
"""
Role management utilities for three-role collaboration pipeline.

Provides:
    parse_role_from_username   - Parse role prefix from username
    load_roles_json            - Load roles.json
    save_roles_json            - Save roles.json
    get_upstream_dirs          - Get upstream directories for a role
    upsert_developer_role      - Add or update developer role mapping
    resolve_role_constraint    - Resolve role and output directory
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

from .paths import (
    DIR_WORKFLOW,
    FILE_ROLES_JSON,
    FILE_TASK_JSON,
    get_repo_root,
    get_current_task,
    get_developer,
)


# =============================================================================
# Constants
# =============================================================================

KNOWN_ROLES = {"pm", "designer", "frontend"}


# =============================================================================
# JSON Helpers (per-module convention)
# =============================================================================

def _read_json_file(path: Path) -> dict | None:
    """Read and parse a JSON file."""
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return None


def _write_json_file(path: Path, data: dict) -> bool:
    """Write dict to JSON file."""
    try:
        path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")
        return True
    except (OSError, IOError):
        return False


# =============================================================================
# Role Parsing
# =============================================================================

def parse_role_from_username(username: str) -> tuple[str | None, str]:
    """Parse role from -u parameter prefix.

    Splits username on the first hyphen. If the prefix is a known role,
    returns it alongside the full name; otherwise returns None.

    Args:
        username: Developer username (e.g., "pm-alice", "bob").

    Returns:
        Tuple of (role, full_name). role is None if no known prefix found.
    """
    if not username:
        return None, username or ""
    parts = username.split("-", 1)
    if len(parts) == 2 and parts[0] in KNOWN_ROLES:
        return parts[0], username
    return None, username


# =============================================================================
# roles.json I/O
# =============================================================================

def load_roles_json(repo_root: Path | None = None) -> dict | None:
    """Load .trellis/roles.json.

    Args:
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        Parsed dict or None if file doesn't exist or is invalid.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    roles_file = repo_root / DIR_WORKFLOW / FILE_ROLES_JSON
    return _read_json_file(roles_file)


def save_roles_json(data: dict, repo_root: Path | None = None) -> bool:
    """Save roles.json.

    Args:
        data: Dictionary to write.
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        True on success, False on error.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    roles_file = repo_root / DIR_WORKFLOW / FILE_ROLES_JSON
    return _write_json_file(roles_file, data)


# =============================================================================
# Upstream Directory Resolution
# =============================================================================

def get_upstream_dirs(role: str, roles_json: dict) -> list[str]:
    """Get upstream directories (all dirs NOT owned by current role).

    Args:
        role: Current developer's role.
        roles_json: Parsed roles.json dict.

    Returns:
        List of directory paths owned by other roles.
    """
    directories = roles_json.get("directories", {})
    return [d for d, r in directories.items() if r != role]


# =============================================================================
# Developer Role Upsert
# =============================================================================

def upsert_developer_role(
    name: str,
    role: str,
    output_dir: str,
    repo_root: Path | None = None,
) -> bool:
    """Add or update developer role mapping in roles.json.

    Creates roles.json if it doesn't exist. Appends to existing if it does.
    Rejects if output_dir is already bound to a DIFFERENT role.
    Overwrites if same developer re-inits (with warning).

    Args:
        name: Developer name.
        role: Developer role (e.g., "pm", "designer", "frontend").
        output_dir: Output directory path.
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        True on success, False on conflict.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    data = load_roles_json(repo_root)
    if data is None:
        data = {"developers": {}, "directories": {}}

    developers = data.get("developers", {})
    directories = data.get("directories", {})

    # Check conflict: output_dir already bound to a different role
    if output_dir in directories and directories[output_dir] != role:
        existing_role = directories[output_dir]
        print(
            f"Error: directory '{output_dir}' is already bound to role "
            f"'{existing_role}', cannot assign to '{role}'",
            file=sys.stderr,
        )
        return False

    # Check re-init: same developer name already exists
    if name in developers:
        print(
            f"Warning: developer '{name}' already exists, overwriting",
            file=sys.stderr,
        )

    # Upsert
    developers[name] = {"role": role, "output_dir": output_dir}
    directories[output_dir] = role
    data["developers"] = developers
    data["directories"] = directories

    return save_roles_json(data, repo_root)


# =============================================================================
# Role Constraint Resolution
# =============================================================================

def resolve_role_constraint(
    repo_root: Path | None = None,
) -> tuple[str | None, str | None]:
    """Three-layer fallback to resolve role and output directory.

    L1: .current-task -> task.json.role / output_dir (highest priority)
    L2: .developer name -> roles.json[name] (persistent fallback)
    L3: (None, None) - no constraint

    Args:
        repo_root: Repository root path. Defaults to auto-detected.

    Returns:
        Tuple of (role, output_dir). Both None if no constraint found.
    """
    if repo_root is None:
        repo_root = get_repo_root()

    # L1: current task -> task.json
    current_task = get_current_task(repo_root)
    if current_task:
        task_json = _read_json_file(repo_root / current_task / FILE_TASK_JSON)
        if task_json:
            task_role = task_json.get("role")
            task_output = task_json.get("output_dir")
            if task_role and task_output:
                return task_role, task_output

    # L2: developer name -> roles.json
    developer = get_developer(repo_root)
    if developer:
        roles_data = load_roles_json(repo_root)
        if roles_data:
            dev_entry = roles_data.get("developers", {}).get(developer)
            if dev_entry:
                return dev_entry.get("role"), dev_entry.get("output_dir")

    # L3: no constraint
    return None, None


# =============================================================================
# Main Entry (for testing)
# =============================================================================

if __name__ == "__main__":
    repo = get_repo_root()
    print(f"Repository root: {repo}")
    print(f"Roles JSON: {load_roles_json(repo)}")
    role, output_dir = resolve_role_constraint(repo)
    print(f"Role constraint: role={role}, output_dir={output_dir}")
