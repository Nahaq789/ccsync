@test:
    go test -v -cover ./... 2>&1 | grep -E '^\s*--- (PASS|FAIL)|coverage:'
