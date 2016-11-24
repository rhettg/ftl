#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

assert "$FTL list" ""
assert "$FTL list --remote" ""

package=$(create_pkg "pegasus")

revision=$($FTL spool $package)
assert "$FTL list pegasus" "$revision"

revision=$($FTL spool --remote $package)
assert "$FTL list --remote pegasus" "$revision"
assert_end list
