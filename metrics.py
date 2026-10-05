#!/usr/bin/env python3
"""Compute key financial metrics for AAPL/MSFT/NVDA from SEC EDGAR company-facts files."""

import csv
import json
import re

TICKERS = ["AAPL", "MSFT", "NVDA"]
METRICS = [
    ("RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues"),
    ("OperatingIncomeLoss",),
    ("NetIncomeLoss",),
]
ANNUAL = re.compile(r"^CY(\d{4})$")


def annual_facts(data, concepts):
    """Annual (full-year, 10-K, fp FY, CY frame) values keyed by calendar year,
    preferring the first concept and falling back to later ones per missing year."""
    us = data["facts"]["us-gaap"]
    out = {}
    for concept in concepts:
        node = us.get(concept, {})
        for entry in node.get("units", {}).get("USD", []):
            m = ANNUAL.match(entry.get("frame", ""))
            if entry.get("fp") == "FY" and entry.get("form") in ("10-K", "DEF 14A") and m:
                out.setdefault(int(m.group(1)), entry["val"])
    return out


def fmt(v):
    return f"{v:.1f}"


def main():
    rows = []
    for ticker in TICKERS:
        with open(f"data/{ticker}.json") as f:
            data = json.load(f)
        rev = annual_facts(data, METRICS[0])
        op = annual_facts(data, METRICS[1])
        ni = annual_facts(data, METRICS[2])
        years = sorted(set(rev) & set(op) & set(ni))[-3:]
        for y in years:
            growth = ""
            if rev.get(y - 1):
                growth = fmt((rev[y] / rev[y - 1] - 1) * 100)
            rows.append({
                "ticker": ticker,
                "year": y,
                "revenue_bn": fmt(rev[y] / 1e9),
                "operating_margin_pct": fmt(op[y] / rev[y] * 100),
                "net_margin_pct": fmt(ni[y] / rev[y] * 100),
                "revenue_growth_pct": growth,
            })
    with open("METRICS.csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)


if __name__ == "__main__":
    main()
