. tests/assert.sh

TMP_DIR=$(mktemp -d)

create_pkg () {
  fname="$TMP_DIR/$1.tar"

  pushd "$TMP_DIR" >/dev/null
  mkdir data
  echo "hello world" > data/data.txt
  tar -czf "$fname" data
  rm -rf data
  popd >/dev/null

  echo "$fname"
}

setup () {
  if [ -z "$FTL_BUCKET" ]; then
    echo "Missing bucket"
    exit 1
  fi

  if ! which aws > /dev/null; then
    echo "Can't find aws cli"
    exit 1
  fi

  if [[ "$(aws s3 ls s3://$FTL_BUCKET | wc -l)" -ne 0 ]]; then
    echo "safety error, non-empty bucket $FTL_BUCKET"
    exit 1
  fi

  export FTL_ROOT="${TMP_DIR}/ftl"
  mkdir "$FTL_ROOT"
  trap "cleanup" EXIT
}

cleanup () {
  aws s3 rm --quiet --recursive s3://$FTL_BUCKET

  test -n "$TMP_DIR" || exit 1  # just cause i'm scared
  rm -rf "$TMP_DIR"
}
