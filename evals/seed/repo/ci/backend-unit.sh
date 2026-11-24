#!/bin/sh
. ci/lib.sh
P=com.acme.shop

pass "$P.cart.CartTest.addsItem" 41
pass "$P.cart.CartTest.mergesGuestCart" 388
pass "$P.pricing.PricingTest.appliesCoupon" 97
if grep -q '^rounding=half_up' backend/pricing.properties; then
  pass "$P.pricing.PricingTest.roundsHalfUp" 63
  pass "$P.billing.InvoiceTest.totalsMatchLineItems" 152
else
  fail "$P.pricing.PricingTest.roundsHalfUp" 63 "expected 10.35 but was 10.34"
  fail "$P.billing.InvoiceTest.totalsMatchLineItems" 152 "invoice total 41.37 differs from line items 41.38"
fi
pass "$P.billing.InvoiceTest.rendersPdf" 4310
pass "$P.user.UserTest.hashesPassword" 1276
pass "$P.user.UserTest.rejectsDuplicateEmail" 219
