---
name: money-precision
description: "Rules for representing, computing, rounding and reconciling money amounts without precision errors. Use when writing or reviewing code, queries, spreadsheets or reports that store, sum, convert, round or display prices, balances, invoices, ledgers, returns or any other currency amount."
---

# Money precision

## Representation

- Never store or compute money in binary floating point (`float`, `double`, JavaScript `number`, pandas `float64`). `0.1 + 0.2` is not `0.3` in binary floating point.
- Use a decimal type or integer minor units:
  - Python uses `decimal.Decimal`, built from strings: `Decimal("19.99")`, never `Decimal(19.99)`.
  - Go uses `int64` minor units or `github.com/shopspring/decimal`.
  - JavaScript and TypeScript use integer minor units in `bigint`, or a decimal library such as `decimal.js`.
  - SQL uses `NUMERIC(precision, scale)` or `DECIMAL`, never `FLOAT` or `REAL`.
- Store every amount with its currency as an ISO 4217 code (`USD`, `EUR`, `JPY`). An amount without a currency is a bug.
- Minor units depend on the currency. `USD` has 2 decimal places, `JPY` has 0, `KWD` has 3. Look up the exponent and do not assume 2.
- Parse input at the boundary. Convert strings, CSV cells and API fields to decimal once, when they enter the program, and reject values that do not parse.

## Rounding

- Round only at defined points: when you display a value, post it to a ledger, or settle a payment. Keep full precision in intermediate steps.
- Name the rounding mode explicitly every time. Never rely on a library default.
  - Half-even (banker's rounding) is the usual default for accounting and statistics because it does not bias sums.
  - Half-up is common on invoices and receipts. Use it where a rule or contract requires it.
  - Truncation (round toward zero) applies only where a rule says so, such as some tax computations.
- When you split an amount (allocation, installments, tax lines), make the parts sum to the whole. Round each part, then assign the leftover minor units to parts in a fixed, documented order.
- Write down the rounding rule next to the code or in the report method.

## Currency and units

- Never add amounts in different currencies. Convert first, with a named rate, a rate date and a rate source.
- Store the rate used with the converted amount so the conversion can be reproduced.
- State the unit on every figure: currency, and scale (`USD thousands`, `EUR millions`). Never mix scales in one table.
- Percent and basis points are different units. 1 percent equals 100 bp. Say which one a field holds.
- Distinguish rates from returns. Say whether a return is simple or log, and whether it is annualized.

## Sign conventions

- Pick one sign convention per dataset and document it. For example, debits positive and credits negative, or inflows positive and outflows negative.
- Do not store direction twice, such as a negative amount with a `type: debit` flag. Use one or the other.
- When data from two systems uses different conventions, convert at the boundary and record that you did.
- In reports, show negatives the same way everywhere, either `-1,234.00` or `(1,234.00)`.

## Reconciliation

- Every total must be recomputable from its parts. After you sum, compare the computed total with the source total.
- When totals differ, report the difference with its sign and unit. Do not adjust a figure silently to force a match.
- Check the usual causes in this order: missing or duplicated rows, sign flips, currency or scale mismatch, rounding at the wrong step, and date cutoffs.
- A difference of a few minor units after rounding is a rounding difference. Book it to a named rounding line, not into another item.
- Keep the reconciliation as a table that shows source total, computed total, difference and explanation.

## Checks before you finish

- Search the change for float types and float literals used on money.
- Confirm that each amount carries a currency, and that each rounding call names its mode.
- Run a test where parts must sum exactly to the total, including an amount that does not divide evenly, such as 100.00 split three ways.
