#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

assert "$FTL list" ""
assert "$FTL list --remote" ""
assert_end list
