#!/bin/bash

set -e

. tests/assert.sh
. tests/lib.sh

setup

assert_raises "$FTL spool pegasus.garbage" 1

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert_raises "echo $revision | grep pegasus"

another_revision=$($FTL spool $package)
assert "test \"$revision\" != \"$another_revision\""

assert_raises "$FTL spool --remote $package | grep pegasus" 0
assert_end spool
