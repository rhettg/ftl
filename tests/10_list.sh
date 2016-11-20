#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

assert "$FTL list" ""
assert "$FTL list --remote" ""

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert "$FTL list pegasus --remote" "$revision"

mkdir -p $FTL_ROOT/pegasus
assert_raises "$FTL spool --remote $revision"

assert "$FTL list pegasus" "$revision"
assert_end list
