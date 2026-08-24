## Behavioral Guidelines

**Tradeoff:** Bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- If something is unclear, stop. Name what's confusing. Ask.
- State your assumptions explicitly. If uncertain, ask.
- Find parts of the design that you're least confident about and core integration points
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.

Mid-task: the moment something goes sideways, STOP and re-plan - don't keep pushing a failing path.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No "flexibility" or "configurability" that wasn't requested.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

Always match existing style, even if you'd do it differently (ex. before writing a new test, check if similar tests exist, add to existing tests or refactor in test cases).

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't re-taxonomize working code.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

Verify by the stated checks: run tests, read logs, and diff behavior against `main` when a change could alter runtime behavior.

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Subagent Strategy
Offload independent work (especially research, exploration, and analysis), one task per subagent, so the main thread stays focused.

## Persistence Between Tasks

### Autonomous Bug Fixing

The moment you stumble on bugs from the logs, errors, failing tests (locally or CI), create a `/home/ccbox/.ccbox/project/bug-<bug_name>.md`, where you describe the bug, reproduction steps, expected behavior, expected bug result, and reproduction results. Tell the user you did so. Do NOT document resolved bugs or bugs to be resolved immediately.

## Code Principles and Styling
Code for elegance:
- Let the code speak for itself through structure (such as grouped decoupled abstractions), syntax terseness/spacing, folder/file/function structure, folder/file/function/variable names, and common assumptions.

On comments:
- When the code is opaque and can't speak for itself, such as gotchas and hard to read code, go into detail on the opaque parts. Strictly only when this occurs, long comments are permitted and preferred.
- Group listed constants/strings/variables by intent and write a title comment for that
- Never write comments that restate code/comments, relist constants/strings/variables, or state where it is used

Code with grouped decoupled abstractions, for example:
- Spend thinking finding "wood grain" or root cause of the code, if you're working around or repeating something, you're likely "cutting against the wood grain"
- Upweight object-oriented style, including object-oriented methods (ex. `arr.get(1)`). Downweight static function calls (ex. `array.get(arr, 1)`).
- Group code with the same intent together
- Break down functions into separate functions for each step, even small functions comprising mostly a loop

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
