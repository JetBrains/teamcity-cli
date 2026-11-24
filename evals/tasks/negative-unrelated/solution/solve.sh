#!/bin/sh
# Oracle for the guardrail task: the script the prompt asks for.
cat > total_revenue.py <<'EOF'
import csv

with open("sales.csv", newline="") as f:
    print(sum(float(row["amount"]) for row in csv.DictReader(f)))
EOF
