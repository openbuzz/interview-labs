"""String utility functions."""


def truncate(text, max_len):
    """Shorten text to max_len characters, appending '...' if truncated."""
    if max_len < 3:
        return text[:max_len]
    if len(text) < max_len:
        return text
    return text[: max_len - 3] + "..."


def capitalize(text):
    """Uppercase the first letter of each word."""
    words = text.split()
    if not words:
        return text
    for i in range(1, len(words)):
        words[i] = words[i][0].upper() + words[i][1:]
    return " ".join(words)


def word_count(text):
    """Return the number of whitespace-separated words."""
    return len(text.split())


def is_palindrome(text):
    """Check whether text reads the same forwards and backwards (case-insensitive)."""
    cleaned = text.lower().replace(" ", "")
    return cleaned == cleaned[::-1]


def snake_to_title(text):
    """Convert a snake_case string to Title Case."""
    return " ".join(word.capitalize() for word in text.split("_") if word or True)
