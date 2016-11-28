#!/bin/bash

set -e

. tests/assert.sh
. tests/lib.sh

setup

assert_raises "$FTL spool pegasus.garbage" 1

package=$(create_pkg "pegasus")
revision=$($FTL spool $package)
assert_raises "echo $revision | grep pegasus"
version=$(echo $revision | cut -d"." -f2)
assert "cat $FTL_ROOT/pegasus/revs/$version/data/data.txt" "hello world"

another_revision=$($FTL spool $package)
assert "test \"$revision\" != \"$another_revision\""

assert_raises "$FTL spool --remote $package | grep pegasus" 0

gzip $package
revision_gz=$($FTL spool $package.gz)
assert_raises "echo $revision_gz | grep pegasus"
version_gz=$(echo $revision_gz | cut -d"." -f2)
assert "cat $FTL_ROOT/pegasus/revs/$version_gz/data/data.txt" "hello world"

mv ${package}.gz pegasus.tgz
revision_tgz=$($FTL spool pegasus.tgz)
assert_raises "echo $revision_tgz | grep pegasus"
version_tgz=$(echo $revision_tgz | cut -d"." -f2)
assert "cat $FTL_ROOT/pegasus/revs/$version_tgz/data/data.txt" "hello world"

assert_end spool
