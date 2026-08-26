## Behavioral Guidelines

### Bug Discovery and Fixing

The moment you stumble on bugs from the logs, errors, failing tests (locally or CI), create a `/home/ccbox/.ccbox/project/bug-<bug_name>.md`, where you describe the bug, reproduction steps, expected behavior, expected bug result, and reproduction results. Tell the user you did so. Do NOT document resolved bugs or bugs to be resolved immediately. Delete file after resolution.

## Code Principles and Styling
Code for elegance. Let the code speak for itself through structure (such as grouped decoupled abstractions), syntax terseness/spacing, folder/file/function structure, folder/file/function/variable names, and common assumptions. Upweight object-oriented style, including object-oriented methods (ex. `arr.get(1)`). Downweight static function calls (ex. `array.get(arr, 1)`).

On comments:
- When the code is opaque and can't speak for itself, such as gotchas and hard to read code, go into detail on the opaque parts. Strictly only when this occurs, long comments are permitted and preferred.
- Group listed constants/strings/variables by intent and write a title comment for that
- NEVER write comments that restate code/comments, relist constants/strings/variables, or state where it is used
