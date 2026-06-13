# DOX Framework - Skills Directory Contract

- This directory governs all installed and custom agent skills.
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose and Scope

This folder holds the agent skills available to coding agents in this environment.
Each subdirectory under `.agents/skills/` represents a single skill containing:
- `SKILL.md` (metadata frontmatter and instructions for the skill)
- Optional `scripts/`, `examples/`, `resources/`, and `references/` directories

## Guidelines for Modifying Skills

1. **Adding Skills**:
   - Use `npx skills@latest add <author/repository>` to install external skills. Do not manually clone or copy unless absolutely necessary.
   
2. **Modifying Existing Skills**:
   - Do not make ad-hoc changes to imported skills unless overriding behavior.
   - If overriding or customizing, document the modifications in this file or the skill's own `SKILL.md`.

3. **Writing Custom Skills**:
   - Follow the structure defined by the [write-a-skill](file:///C:/Users/HP/Desktop/termtk/.agents/skills/write-a-skill/SKILL.md) guidelines.
   - Ensure the skill has a clean, linted `SKILL.md` file.

4. **Consistency**:
   - Make sure any additions or updates are tracked correctly in [skills-lock.json](file:///C:/Users/HP/Desktop/termtk/skills-lock.json).
