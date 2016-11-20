#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert_raises "echo $revision | grep pegasus"

another_revision=$($FTL spool $package)
assert "test \"$revision\" != \"$another_revision\""
assert "$FTL spool --remote $revision" "$revision"

assert_raises "$FTL spool --remote pegasus.garbage" 1
assert_end spool
