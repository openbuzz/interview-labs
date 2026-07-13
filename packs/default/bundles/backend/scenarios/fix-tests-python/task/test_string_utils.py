"""Tests for string_utils module."""

import unittest

from string_utils import capitalize, is_palindrome, snake_to_title, truncate, word_count


class TestTruncate(unittest.TestCase):
    def test_truncate(self):
        # Should not truncate when text fits exactly.
        self.assertEqual(truncate("hello", 5), "hello")
        # Should truncate long text with "..." suffix.
        self.assertEqual(truncate("hello world", 8), "hello...")


class TestCapitalize(unittest.TestCase):
    def test_capitalize(self):
        self.assertEqual(capitalize("hello world"), "Hello World")


class TestWordCount(unittest.TestCase):
    def test_word_count(self):
        self.assertEqual(word_count("the quick brown fox"), 4)


class TestIsPalindrome(unittest.TestCase):
    def test_is_palindrome(self):
        self.assertTrue(is_palindrome("Race Car"))
        self.assertFalse(is_palindrome("hello"))


class TestSnakeToTitle(unittest.TestCase):
    def test_snake_to_title(self):
        self.assertEqual(snake_to_title("hello_world"), "Hello World")
