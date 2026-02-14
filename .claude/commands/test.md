# Test

Run the full CI pipeline locally to verify changes before pushing.

## Instructions

Run each step sequentially, stopping on the first failure. Report results as a summary table at the end.

### Step 1: Tests with Race Detection and Coverage

```bash
go test -v -race -coverprofile=coverage.out ./...
```

If any test fails, stop and report the failure.

### Step 2: Coverage Threshold (70%)

```bash
COVERAGE_LINE=$(go tool cover -func=coverage.out | tail -1)
COVERAGE=$(echo "$COVERAGE_LINE" | sed 's/.*\s\([0-9]*\.[0-9]*\)%.*/\1/')
echo "Coverage: ${COVERAGE}%"
```

If coverage is below 70%, report it as a failure.

### Step 3: Lint

First verify the config is valid, then run the linter. If golangci-lint is not installed, install it first.

```bash
golangci-lint config verify
```

If config verify fails, stop and report the issue.

```bash
golangci-lint run ./...
```

If golangci-lint is not installed, install it first:
```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0
```

If lint fails, report the issues.

### Step 4: Vulnerability Scan

```bash
govulncheck ./...
```

If govulncheck is not installed, install it first:
```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
```

### Results

After all steps complete (or on first failure), display a summary:

```
Pipeline Results:
  Tests:         PASS/FAIL
  Coverage:      XX.X% (threshold: 70%)
  Lint Config:   PASS/FAIL
  Lint:          PASS/FAIL
  Vuln Scan:     PASS/FAIL
```
