# String Utilities — Solution

## Objectives

1. Read test failure messages to understand expected vs actual behavior.
2. Trace the bug from the failing assertion back to the implementation.
3. Apply minimal, correct fixes without altering unrelated code.
4. Verify all tests pass after each fix.

## Changes

- **Truncate off-by-one**: The length check used `<` instead of `<=`. When the text length exactly equals `max_len`, it was incorrectly truncated. Fix: change `len(text) < max_len` to `len(text) <= max_len`. This is the most common category of boundary bug — the "fence-post" error.
- **Capitalize skips first word**: The range started at `1` instead of `0`, so the first word was never capitalized. Fix: change `range(1, len(words))` to `range(0, len(words))`. The test failure makes this visible immediately — `"hello World"` instead of `"Hello World"`.

## Expected result

All five tests pass:

```
Ran 5 tests in 0.001s

OK
```
