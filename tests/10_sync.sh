#!/bin/bash

set -e

. tests/assert.sh
. tests/lib.sh

setup

package=$(create_pkg "pegasus")
revision=$($FTL spool --remote $package)
version=$(echo $revision | cut -d"." -f2)

mkdir -p $FTL_ROOT/pegasus
assert_raises "$FTL sync"

assert_raises "$FTL list pegasus | grep $revision"
assert "cat $FTL_ROOT/pegasus/revs/$version/data/data.txt" "hello world"
assert_raises "test -d $FTL_ROOT/pegasus/current" 1

assert_raises "$FTL jump --remote $revision"
assert_raises "$FTL list --remote pegasus | grep $revision | grep active"

assert_raises "$FTL sync"

assert_raises "$FTL list pegasus | grep $revision | grep active"

assert "cat $FTL_ROOT/pegasus/current/data/data.txt" "hello world"
assert_end sync
