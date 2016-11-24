#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert_raises "$FTL jump $revision"
assert_raises "$FTL list pegasus | grep $revision | grep active"
assert_raises "test -d $FTL_ROOT/pegasus/current"
assert "cat $FTL_ROOT/pegasus/current/data/data.txt" "hello world"
assert_end jump
