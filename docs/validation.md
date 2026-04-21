# Validation

Validation is intentionally simple and inspectable.

## What it checks

- claim-token overlap against cited evidence text
- unsupported named entities against evidence text
- citations map to retrieved chunk IDs
- `validation.min_evidence_coverage` threshold

This is not a formal truth verifier.
It reduces obvious unsupported claims while keeping complexity low.

## Guardrail behaviour examples

Sample refusal message:

```text
Sorry, but I could not relate your question to the content I have.
```

Sample grounded answer shape:

```json
{
  "answer": "Cheap change is mainly about reducing coupling and using explicit interfaces so changes stay local.",
  "citations": ["chunk_123abc"]
}
```

## Retrieval gate reminder

Validation runs after retrieval gating.
The model is only allowed to answer when retrieval passes scope checks.
