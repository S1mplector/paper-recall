#!/usr/bin/env python3
"""Convert a plain-text Anki export into a private Paper Recall JSON bundle."""
import argparse
import csv
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path

class PlainText(HTMLParser):
    def __init__(self): super().__init__(); self.parts = []
    def handle_starttag(self, tag, attrs):
        if tag in ("br", "p", "div", "li"): self.parts.append("\n")
        if tag == "img": self.parts.append("[image omitted]")
    def handle_data(self, data): self.parts.append(data)

def plain(value):
    p = PlainText(); p.feed(value)
    return "".join(p.parts).strip()

def convert(path, deck):
    cards = []; seen = set()
    with path.open(encoding="utf-8-sig", newline="") as stream:
        lines = stream.readlines()
    html = any(line.strip().lower() == "#html:true" for line in lines)
    rows = csv.reader((line for line in lines if not line.startswith("#")), delimiter="\t")
    for row in rows:
        if not row or all(not field.strip() for field in row): continue
        if len(row) < 2: raise ValueError("Expected at least two tab-separated fields per card")
        front, back = (plain(x) for x in row[:2]) if html else (x.strip() for x in row[:2])
        if not front or not back: raise ValueError("Cards must have both a question and an answer")
        identity = hashlib.sha256((deck + "\0" + front + "\0" + back).encode()).hexdigest()[:24]
        if identity in seen: continue
        seen.add(identity)
        cards.append(dict(id="import-" + identity, deck=deck, front=front, back=back))
    if not cards: raise ValueError("No cards found")
    return cards

if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("input", type=Path)
    p.add_argument("--deck", required=True)
    p.add_argument("--output", type=Path, default=Path("private/cards.recall"))
    a = p.parse_args()
    try: cards = convert(a.input, a.deck.strip())
    except (ValueError, OSError) as e: p.error(str(e))
    import re
    deck_id = re.sub(r"[^a-z0-9._-]+", "-", a.deck.lower()).strip("-._")[:60] or "anki-deck"
    document = {"format":"paper-recall", "version":1, "deck":{"id":deck_id,"name":a.deck.strip()},
        "cards":[{"id":c["id"],"front":c["front"],"back":c["back"]} for c in cards]}
    a.output.parent.mkdir(parents=True, exist_ok=True)
    a.output.write_text(json.dumps(document, ensure_ascii=False, indent=2) + "\n")
    print(f"Wrote {len(cards)} cards to {a.output}")
