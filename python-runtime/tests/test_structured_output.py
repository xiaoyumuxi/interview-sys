import unittest

from app.structured_output import parse_json_object


class StructuredOutputTest(unittest.TestCase):
    def test_parse_json_object_plain(self):
        self.assertEqual(parse_json_object('{"score": 1}'), {"score": 1})

    def test_parse_json_object_code_fence(self):
        self.assertEqual(parse_json_object('```json\n{"ok": true}\n```'), {"ok": True})

    def test_parse_json_object_with_prefix(self):
        self.assertEqual(parse_json_object('result: {"ok": true}'), {"ok": True})
