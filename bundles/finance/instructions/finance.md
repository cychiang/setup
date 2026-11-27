## Finance rules

- Represent money as decimal or integer minor units, never binary floating point. Follow the `money-precision` skill.
- Every amount carries its currency (ISO 4217) and its scale. Every rate carries its date and source.
- Round only at display, posting or settlement, with a named rounding mode.
- Totals must reconcile to their parts. Report any difference instead of adjusting figures.
- Analysis and reports are not financial advice. Say so when the output could be read as a recommendation.
