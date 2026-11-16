# Hub copy diff

`mwanachama-backend-comm` is the custodian of the comm module. The hub (`projects/hub-steward/code/wakala-api/modules/comm`) holds a copy made on 2026-11-04 and changed it afterwards. Every change made in that copy exists here, so the two differ by the import path only (hub issue CM108, the comm part of ISS-105).

## Command

```
scripts/hub-copy-diff.sh [path-to-hub-modules-comm]
```

The default path is `../projects/hub-steward/code/wakala-api/modules/comm`, relative to this repository's parent folder. The exit code is 1 when the trees differ, 0 when they do not; a clean result still exits 1 while `Only in backend` test files exist, so read the output, not the code. The script and `scripts/hubdiff` are the ones of `mwanachama-backend-git` (G78).

## What it does

1. Copies the `*.go` and `*.json` files of this repository and of the hub folder into two temporary folders. `cmd`, `documentation` and `scripts` are left out, and so are `mcp` and `spec`, which exist only here: the hub never carried them.
2. Rewrites the hub's imports: `wakalaserver/modules/shared/` becomes `github.com/aosanya/mwanachama-backend-shared/` and `wakalaserver/modules/comm` becomes `github.com/aosanya/mwanachama-backend-comm`.
3. Runs every Go file on both sides through `scripts/hubdiff`, which parses it, drops all comments and prints it with `gofmt`, so comments and layout do not show as differences.
4. Runs `diff -r backend hub`.

## Result of 2026-11-16

Every non-test Go file and both JSON files (`comm.blueprint.json`, `comm.operations.json`) are identical after the rewrite, and every test file of the hub exists here and is identical. Nothing in the output is `diff` or `Only in hub`. What remains is 83 `Only in backend` lines, all test files this repository wrote that the hub copy never carried: 46 in the root package, 32 in `routes` and 5 in `models`. They test code that is identical in both.

## The hub commits carried over

One commit here per hub commit or group, each naming the hub hash: eef9ff73, ec77c144, b1f9348c, a37f9023, 997c7b76, 20050ce0, 3de4ffa2, 921429eb (the comm half), c9907728, e97a230a (the comm half), and the test-only commits 3351da2a, 3607f941, bb9b7712, f09917d3, 728c352f, 6b9e8f54, c5145b9f, e03d5c84, d43aea5f, ab087f5a, c6d68aca, f2f44c31 and 4ea6d46f.

Files a commit touched lost their comments (the repository rule is no comments in code). The test-only commits were replayed before 921429eb because its test uses the fixture that 6b9e8f54 introduced.
