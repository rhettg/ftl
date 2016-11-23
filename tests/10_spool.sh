#!/bin/bash

set -e

. tests/assert.sh
. tests/lib.sh

setup

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert_raises "echo $revision | grep pegasus"

another_revision=$($FTL spool $package)
assert "test \"$revision\" != \"$another_revision\""
assert_raises "$FTL spool --remote $revision | grep Adding" 0

assert_raises "$FTL spool --remote pegasus.garbage" 1
assert_end spool
