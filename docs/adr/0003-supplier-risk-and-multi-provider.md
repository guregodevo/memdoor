# ADR 0003 — One supplier is a single point of legal failure

Status: OPEN QUESTION (2026-08-20) · Affects ADR 0001 · Blocks nothing today

## Context

The managed lane rents on Memdoor's own pooled Vast.ai account and charges
customers the provider's rate plus a commission (ADR 0001). Reading Vast's
Terms while asking whether that is permitted turned up Prohibited Activities
item 10:

> "Using the Company Services as part of any effort to compete with Company or
> to provide services as a service bureau"

Plus: *"You may not use anyone else's account at any time"*, and "Authorized
Users" defined as employees, consultants, contractors, and agents of the
account holder — not paying customers.

"Service bureau" is the standard term for running third parties' workloads on
your account for a fee, which describes the managed lane closely. The term is
not defined in the document, the clause sits in a website-conduct list, and
aggregating GPU supply is a common business — so this is a question, not a
verdict. But it is not a question to answer by reading harder.

Removing the bring-your-own-key lane (2026-08-20) increased the exposure: that
arrangement was unambiguously fine, because the customer was the account
holder.

## Decision

None yet. Two things happen in parallel:

1. **Ask Vast in writing** whether renting on our account for paying customers
   with a margin is permitted, and whether a reseller or partner agreement
   exists. Providers generally want aggregators who bring them volume; their
   answer is worth more than any clause interpretation. Then an hour of a
   lawyer's time — EU VAT on digital services and merchant-of-record duties are
   separate questions from the ToS.

2. **Remove the single-supplier dependency** regardless of the answer
   (MUST ⭐ #0.3 — Marketplace efficiency). The market has 40+ neoclouds and 9 marketplaces; several run
   partner programmes precisely because aggregators bring volume. Sourcing from
   more than one is a better price for customers AND the hedge here.

## If the answer is no

The product does not need rewriting; the account holder changes.
Per-customer sub-accounts — where the customer is the account holder and
Memdoor orchestrates, provisioning the credential rather than asking the user
to paste one — keeps the experience and moves the ToS relationship. That would
supersede ADR 0001's trust placement, not the architecture around it.

## Exposure today

One $5 test balance and the operator's own rentals. This is the cheapest
moment this question will ever have.
