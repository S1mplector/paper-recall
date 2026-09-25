# Scheduling

Paper Recall uses a deterministic SM-2-inspired scheduler with explicit learning and relearning steps. It is not Anki's implementation or FSRS, and it does not estimate a personalized retention probability. All intervals shown on rating buttons come from the same function that commits the rating.

## Learning

New cards start with ease 2.5. Again schedules one minute and resets the learning step. Good at the first step schedules ten minutes; Good at the second step graduates to a one-day review interval. Hard schedules six minutes at the first step or ten minutes at the second, without advancing the step. Easy graduates immediately to four days.

## Review

Let `I` be the previous interval in days, `E` the ease, and `L` the number of days overdue (never negative).

| Rating | Result |
| --- | --- |
| Again | Increment lapse count; decrease ease by 0.2; retain half the old interval (rounded, at least one day); enter relearning due in ten minutes |
| Hard | Decrease ease by 0.15; interval `max(I + 1, I × 1.2)` |
| Good | Keep ease; interval `max(I + 1, (I + L/2) × E)` |
| Easy | Increase ease by 0.15; interval `max(I + 2, (I + L) × updated E × 1.3)` |

Ease is bounded between 1.3 and 3. Review intervals are rounded to whole days and capped at 3,650 days. Due times are measured from the time of the rating; a day is 24 hours, not the next midnight.

## Relearning

Again schedules one minute. Hard schedules fifteen minutes. Good returns to review at the retained interval. Easy increases ease by 0.15 and returns to review at `max(retained interval + 1, retained interval × 1.3)` days.

## Queue and correctness

Due learning/relearning cards come first, then due reviews, then new cards; each group sorts by due time. No early review or daily new-card cap is implemented. The due queue refreshes while on the home screen and while waiting for the next due card.

A rating is acknowledged only after saving. Each request includes the expected review count, so duplicate or stale ratings cannot advance an already changed card. Undo restores the previous schedule and daily count for the last rating while retaining text edits. It survives reopening. Daily counts count ratings, including repeated learning steps, using the device's local date.

If the clock moves more than five minutes behind the last mutation, reviewing pauses until the clock is corrected. Keep the tablet's date and time accurate.
