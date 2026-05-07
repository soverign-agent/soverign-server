# Eval Lexicons

These JSON files seed the lexicon-based detectors in `shared/eval/`.

## Sourcing

Both lexicons were authored manually for milestone M18. Entries were chosen
to be **obviously and uncontroversially** flagged so the first-pass scoring
produces high-precision signals. We deliberately did not import larger
public corpora (e.g. HateBase, Hatecheck) to avoid:

- importing entries with unclear licensing,
- shipping ambiguous or contested terms in the default policy,
- inflating the lexicon to a size where false-positive rate dominates.

## Limitations

- **English only.** No tokenizer support for CJK, Arabic, etc.
- **Surface-form matching.** No stemming, lemmatization, or fuzzy matching.
  "killing yourself" does not match the phrase "kill yourself".
- **No context model.** Sarcasm, quotation, and reported speech ("She said
  'kill yourself'") are flagged the same as a direct utterance. Negation
  handling is limited to a five-token window before the match.
- **Cultural and dialect coverage is shallow.** Only widely recognized
  English-language stereotypes are seeded.
- **Threat regexes are narrow.** They catch common templates ("I will kill
  you") but not paraphrases ("you'd better watch your back").
- **Profanity ≠ toxicity.** The profanity bucket is intentionally small;
  bare profanity in product feedback is not the same as targeted abuse.

These detectors are a safety net, not a moderation system. The roadmap
calls for an LLM-as-Judge or hosted classifier in a follow-up milestone;
this implementation is the baseline that those upgrades replace.

## Tuning

Score normalization saturates quickly so a small number of obvious hits
already triggers thresholds:

| Detector   | Per hit | Saturation point |
|------------|---------|------------------|
| Bias       | 0.20    | 5 hits → 1.0     |
| Toxicity   | 0.30 (term) / 0.50 (threat) | mixed → 1.0 |

If the false-positive rate is too high in production, raise the per-hit
weight floor or prune the lexicon — do not extend it speculatively.
