# Prompt for generating a deck

Copy the following, replace the bracketed parts, and save the JSON output as `my-deck.recall`. Review generated answers for accuracy before studying.

> Create a Paper Recall flashcard deck about [TOPIC] for [LEVEL], containing [NUMBER] cards. Return only a single valid JSON object, without Markdown fences or commentary. Use this structure:
>
> {"format":"paper-recall","version":1,"deck":{"id":"my-topic","name":"My topic","folder":"Studies/My topic"},"cards":[{"id":"concept-001","front":"A focused question","back":"A concise, accurate answer"}]}
>
> Replace the example with the requested content. Use stable, meaningful deck and card IDs of 1–80 ASCII characters: letters, digits, dots, underscores, and hyphens, beginning with a letter or digit. Each card ID must be unique. Ask one clear question per card. Use plain text, not HTML or Markdown formatting; encode line breaks as JSON \n. Include no extra fields, schedule data, comments, or trailing commas. Each front and back must be nonempty and below 16,000 UTF-8 bytes; the deck name must be below 200 bytes. The folder is optional; use at most six slash-separated segments of at most 80 bytes each. Keep the file below 8 MiB and at most 10,000 cards. When updating a supplied deck, preserve its deck ID and existing card IDs so progress survives import.

Upload with `python3 scripts/send-deck.py my-deck.recall`, then tap **Import** on the tablet.
