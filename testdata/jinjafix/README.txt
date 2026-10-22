Reproduce the Python reference:
  python3 testdata/jinjafix/reference.py

Run the shared-input Go regression:
  go test -count=1 -v -run ^TestJinjaFixRegression$ .

The test commit records Python results and failing Go results from the same
inputs. The fix commit records passing Go results and the full Go test suite.
Worker processes bound panics, stack exhaustion and nontermination.
