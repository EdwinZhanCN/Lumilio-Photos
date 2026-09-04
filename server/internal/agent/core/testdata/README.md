# Agent checkpoint compatibility fixtures

`eino-v0.9.6-confirmation-checkpoint.bin` is an Eino `v0.9.6`
`adk.ChatModelAgent` checkpoint captured after the same streaming,
`compose.StatefulInterrupt`, and targeted-confirmation path used by Lumilio.
The adjacent `.root-id` file is the root interrupt ID emitted by that exact
checkpoint and is required for `ResumeWithParams`.

The fixture is intentionally generated outside the Server module so its old
Eino dependency cannot enter the shipping dependency graph. To regenerate it,
copy `generator/go.mod.txt` to `go.mod` and `generator/main.go.txt` to `main.go`
inside a temporary directory, then run:

```sh
go mod tidy
go run . /absolute/path/to/eino-v0.9.6-confirmation-checkpoint.bin
```

Both fixture files must be regenerated together. Ordinary Server tests only
read them; they never execute the old dependency.
