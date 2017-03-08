#!/usr/bin/env bash

# Check gofmt
echo "==> Checking QingCloud provider for unchecked errors..."
echo "==> NOTE: at this time we only look for uncheck errors in the QingCloud package"

if ! which errcheck > /dev/null; then
    echo "==> Installing errcheck..."
    go get -u github.com/kisielk/errcheck
fi

err_files=$(errcheck -ignoretests -ignore \
  'github.com/hashicorp/terraform/helper/schema:Set' \
  -ignore 'bytes:.*' \
  -ignore 'io:Close|Write' \
  ./qingcloud/...)

if [[ -n ${err_files} ]]; then
    echo 'Unchecked errors found in the following places:'
    echo "${err_files}"
    echo "Please handle returned errors. You can check directly with \`make errcheck\`"
    exit 1
fi

exit 0
