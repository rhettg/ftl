#!/bin/bash

set -e

. tests/lib.sh
. tests/assert.sh

assert_raises 'test -n "$FTL"'
assert_raises 'test -n "$FTL_ROOT"'
assert_raises 'test -n "$FTL_BUCKET"'
assert_raises 'test -n "$AWS_DEFAULT_REGION"'
assert_end setup
