#!/bin/sh
echo "Compiling backend sources"
if grep -q 'com.acme.shop.legacy' backend/src/Cart.java; then
  echo "backend/src/Cart.java:3: error: package com.acme.shop.legacy does not exist"
  echo "1 error"
  exit 1
fi
echo "Compiled 214 classes"
