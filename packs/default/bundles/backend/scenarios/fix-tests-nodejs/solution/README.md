# String Utilities — Solution

## Objectives

1. Read test failure messages to understand expected vs actual behavior.
2. Trace the bug from the failing assertion back to the implementation.
3. Apply minimal, correct fixes without altering unrelated code.
4. Verify all tests pass after each fix.

## Changes

- **Truncate off-by-one**: The length check used `<` instead of `<=`. When the text length exactly equals `maxLen`, it was incorrectly truncated. Fix: change `text.length < maxLen` to `text.length <= maxLen`. This is the most common category of boundary bug — the "fence-post" error.
- **Capitalize skips first word**: The loop started at index `1` instead of `0`, so the first word was never capitalized. Fix: change `let i = 1` to `let i = 0`. The test failure makes this visible immediately — `"hello World"` instead of `"Hello World"`.

## Expected result

All five tests pass:

```
# tests 5
# pass 5
# fail 0
```
