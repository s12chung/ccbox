## Overview

This project is basically trying to create a docker devbox wrapper for claude code.

## Workflow Orchestration

### 1. Plan Mode Default
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, STOP and re-plan immediately - don't keep pushing
- Use plan mode for verification steps, not just building
- Write detailed specs upfront to reduce ambiguity
- Most importantly, Ask for clarifying questions early and often

### 2. Subagent Strategy
- Use subagents liberally to keep main context window clean
- Offload research, exploration, and parallel analysis to subagents
- For complex problems, throw more compute at it via subagents
- One task per subagent for focused execution

### 3. Self-Improvement Loop
- After ANY correction from the user: update `tasks/lessons.md` with the pattern
- Write rules for yourself that prevent the same mistake
- Ruthlessly iterate on these lessons until mistake rate drops
- Review `tasks/lessons.md` at session start for relevant project

### 4. Verification Before Done
- Never mark a task complete without proving it works
- Diff behavior between main and your changes when relevant
- Ask yourself: "Would a staff engineer approve this?"
- Run tests, check logs, demonstrate correctness

### 5. Demand Elegance (Balanced)
- For non-trivial changes: pause and ask "is there a more elegant way?"
- If a fix feels hacky: "Knowing everything I know now, implement the elegant solution"
- Skip this for simple, obvious fixes - don't over-engineer
- Push for minimal impact changes that only touch what's necessary. Avoid introducing bugs.
- Challenge your own work before presenting it

### 6. Autonomous Bug Fixing
- When given a bug report: just fix it. Don't ask for hand-holding
- When stumbling on bugs from the logs, errors, failing tests (locally or CI), immediately, with zero context switching required from the user, attempt to reproduce it 3 times. Create a `tasks/bug-<bug_name>.md`, where you describe the bug, reproduction steps, expected behavior, expected bug result, and reproduction results. Tell the user you did so.

## Task Management
1. **Plan First**: Write plan to `tasks/todo.md` with checkable items
2. **Verify Plan**: Check in before starting implementation
3. **Track Progress**: Mark items complete as you go
4. **Explain Changes**: High-level summary at each step
5. **Document Results**: Add review section to `tasks/todo.md`
6. **Capture Lessons**: Update `tasks/lessons.md` after corrections

## Code Principles and Styling
Code for elegance and avoid comments. Let the code speak for itself through structure (such as grouped decoupled abstractions), syntax terseness/spacing, folder/file/function structure, folder/file/function/variable names, and common assumptions. Add, with preferably 1 line (via. extreme optimization of word count), extra comments about title comments, intent, and organization to assist with this. When the code is opaque and can't speak for itself, such as gotchas and hard to read code, go into detail on the opaque parts.

Code with grouped decoupled abstractions, for example:
- Spend thinking finding "wood grain" or root cause of the code, if you're working around or repeating something, you're likely modelling the code with the wrong shape or "cutting against the wood grain"
- When making suggestions/changes, think about whether the current solution does the same with less code
- Upweight object-oriented style, including object-oriented methods (ex. `arr.get(1)`) over the static function calls (ex. `array.get(arr, 1)`)
- Group code with the same intent together
- Define all the "switches and knobs" of the code together, such as pulling out custom literals into global constants
- Break down functions into separate functions for each step, even small functions comprising mostly a loop

Regarding this section, in order for you to learn and emphasize these patterns, I will often ask you to update `tasks/lessons.md` with examples.