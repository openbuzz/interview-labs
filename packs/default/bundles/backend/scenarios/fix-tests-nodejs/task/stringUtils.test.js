const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const {
  truncate,
  capitalize,
  wordCount,
  isPalindrome,
  snakeToTitle,
} = require("./stringUtils");

describe("truncate", () => {
  it("should truncate only when text exceeds maxLen", () => {
    assert.strictEqual(truncate("hello", 5), "hello");
    assert.strictEqual(truncate("hello world", 8), "hello...");
  });
});

describe("capitalize", () => {
  it("should capitalize the first letter of each word", () => {
    assert.strictEqual(capitalize("hello world"), "Hello World");
  });
});

describe("wordCount", () => {
  it("should count words separated by whitespace", () => {
    assert.strictEqual(wordCount("the quick brown fox"), 4);
  });
});

describe("isPalindrome", () => {
  it("should detect palindromes ignoring case and spaces", () => {
    assert.strictEqual(isPalindrome("Race Car"), true);
    assert.strictEqual(isPalindrome("hello"), false);
  });
});

describe("snakeToTitle", () => {
  it("should convert snake_case to Title Case", () => {
    assert.strictEqual(snakeToTitle("hello_world"), "Hello World");
  });
});
