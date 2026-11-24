#!/bin/sh
echo "Type-checking frontend"
if grep -q 'quantity: string' frontend/src/cart.ts; then
  echo "src/cart.ts(4,3): error TS2322: Type 'string' is not assignable to type 'number'."
  exit 2
fi
echo "Bundled 38 modules"
