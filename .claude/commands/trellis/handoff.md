---
description: "Complete current phase and generate handoff document for downstream roles"
---

You are executing the **HANDOFF** workflow for a collaborative project.

## Step 1: Identify Context

1. Read `.trellis/.developer` to get the current developer name
2. Read `.trellis/roles.json` to determine the role and bound output directory
3. If roles.json doesn't exist or current developer has no role mapping, abort with:
   "Error: No role binding found. Run `trellis init -u <role>-<name> -d <dir>` first."

## Step 2: Check Deliverables

1. List all files in the bound output directory
2. If the directory is empty, warn: "Output directory is empty. Are you sure you want to proceed?" and wait for confirmation
3. Run `git diff --name-only` scoped to the output directory to identify recent changes

## Step 3: Generate HANDOFF.md

Create or overwrite `HANDOFF.md` **inside the output directory** with the following structure:

````
# {Feature/Task Name} - Handoff Document

## Task Info
- **Role**: {current role}
- **Developer**: {developer name}
- **Output Directory**: {bound directory}
- **Completed**: {current timestamp}

## Summary
(Summarize what was done in 2-3 paragraphs, based on git diff + conversation context)

## Deliverable Files
(List all files in the output directory with one-line descriptions)

## Key Design Decisions
(Numbered list of important decisions and rationale)

## Notes for Downstream
(Specific items the next role should pay attention to)

## Contact
Questions? Reach out to {developer name}
````

## Step 4: Update CHANGELOG.md

1. If `CHANGELOG.md` doesn't exist in the output directory, create it with the standard header:
   ````
   # CHANGELOG - {directory name}

   > 变更记录表，由 AI 自动维护（/trellis:handoff 时追加）。

   | 日期 | 作者 | 类型 | 摘要 | 关联文件 |
   |------|------|------|------|----------|
   ````
2. Append one row to the changelog table:
   - Date: current datetime (YYYY-MM-DD HH:mm)
   - Author: developer name from .developer
   - Type: Infer from git diff (新增/修改/删除/重构)
   - Summary: One sentence (max 80 chars) summarizing the changes
   - Files: Key files changed

## Step 5: User Review

1. Show the generated HANDOFF.md content to the user
2. Show the new CHANGELOG.md entry
3. Ask: "Please review the handoff document. Reply with changes or confirm to finalize."
4. If the user requests changes, modify and re-show
5. Once confirmed, remind:
   - `git add {output_dir}/` to stage changes
   - `git commit` and `git push`
   - Notify the downstream developer offline

## Constraints

- **NEVER** write files outside the bound output directory
- HANDOFF.md and CHANGELOG.md must be placed at the **root** of the output directory
- If `--message` argument is provided, include it in the HANDOFF.md Notes section
