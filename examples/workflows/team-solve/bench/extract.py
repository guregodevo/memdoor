#!/usr/bin/env python3
"""The last ```python block of a model answer -> solution.py (the whole text if there is none)."""
import re, sys
text = open(sys.argv[1]).read()
blocks = re.findall(r"```(?:python|py)?\s*\n(.*?)```", text, re.S)
open(sys.argv[2], "w").write((blocks[-1] if blocks else text).rstrip() + "\n")
print("wrote", sys.argv[2], "from", "a code block" if blocks else "the whole answer")
