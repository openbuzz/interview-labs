// Truncate text to maxLen characters, appending "..." if truncated.
function truncate(text, maxLen) {
  if (maxLen < 3) {
    return text.slice(0, maxLen);
  }
  if (text.length < maxLen) {
    return text;
  }
  return text.slice(0, maxLen - 3) + "...";
}

// Capitalize the first letter of each word.
function capitalize(text) {
  const words = text.split(/\s+/).filter(Boolean);
  if (words.length === 0) {
    return text;
  }
  for (let i = 1; i < words.length; i++) {
    words[i] = words[i][0].toUpperCase() + words[i].slice(1);
  }
  return words.join(" ");
}

// Count the number of whitespace-separated words.
function wordCount(text) {
  return text.split(/\s+/).filter(Boolean).length;
}

// Check whether text reads the same forwards and backwards (case-insensitive).
function isPalindrome(text) {
  const cleaned = text.toLowerCase().replaceAll(" ", "");
  return cleaned === cleaned.split("").reverse().join("");
}

// Convert a snake_case string to Title Case.
function snakeToTitle(text) {
  return text
    .split("_")
    .map((word) => (word.length > 0 ? word[0].toUpperCase() + word.slice(1) : word))
    .join(" ");
}

module.exports = { truncate, capitalize, wordCount, isPalindrome, snakeToTitle };
