import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("importer", Path(__file__).resolve().parents[1] / "scripts/import_anki.py")
mod = importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)

class ImportTests(unittest.TestCase):
    def convert(self, text):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "cards.txt"; p.write_text(text)
            return mod.convert(p, "Test")
    def test_plain_and_duplicate(self):
        cards = self.convert("Question\tAnswer\nQuestion\tAnswer\n")
        self.assertEqual(len(cards), 1)
        self.assertEqual(cards, self.convert("Question\tAnswer\n"))
    def test_html(self):
        c = self.convert('#html:true\n<b>Bonjour</b>\tHello<br>World\n')[0]
        self.assertEqual(c['front'], 'Bonjour'); self.assertEqual(c['back'], 'Hello\nWorld')
    def test_multiline(self):
        self.assertEqual(self.convert('Q\t"First\nSecond"\n')[0]['back'], 'First\nSecond')
    def test_invalid(self):
        with self.assertRaises(ValueError): self.convert('Just a question\n')
        with self.assertRaises(ValueError): self.convert('Q\t\n')

if __name__ == '__main__': unittest.main()
