#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

setup

package=$(create_pkg "batman")
revision=$($FTL spool $package)
assert_raises "echo $revision | grep batman"

another_revision=$($FTL spool $package)
assert "test \"$revision\" != \"$another_revision\""
assert_end spool
