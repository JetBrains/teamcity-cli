#!/bin/sh
# Usage: backend-integration.sh <run number>. Outcomes depend on shared test infrastructure, not on the commit.
. ci/lib.sh
P=com.acme.shop.it
n="$1"

t() { # t <name> <duration> <failing run numbers> <message>
  if in_list "$n" "$3"; then fail "$P.$1" "$2" "$4"; else pass "$P.$1" "$2"; fi
}

t "CheckoutFlowIT.completesOrderWithSavedCard" 5120 "2 8 10 13 14 17 20 23 26" "payment gateway sandbox timed out after 5000 ms"
t "CheckoutFlowIT.rejectsExpiredCard" 870 "" ""
t "InventorySyncIT.reconcilesConcurrentUpdates" 2340 "4 11 19 24" "stock level 7 differs from warehouse 8"
t "SearchIndexIT.reindexesAfterSchemaChange" 9450 "3 16" "index shop-products is red"
if [ "$n" -ge 7 ]; then
  fail "$P.LegacyExportIT.exportsCsv" 410 "sftp://legacy-erp.acme.example: connection refused"
else
  pass "$P.LegacyExportIT.exportsCsv" 410
fi
t "OrderHistoryIT.paginatesLargeAccounts" 1630 "" ""
t "NotificationIT.sendsOrderConfirmation" 760 "" ""
