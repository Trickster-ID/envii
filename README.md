# envii

> Encrypted `.env` & secrets manager with a beautiful terminal UI.

Stop scattering `.env` files across projects and pasting secrets into Slack.
`envii` keeps all your environment variables in a single **encrypted** vault and
lets you browse, edit, and inject them straight from your terminal.

<p align="center">
  <img src="docs/demo.gif" alt="envii demo" width="700">
</p>

## Why envii?

- **One encrypted vault** — all projects & environments in `~/.config/envii/vault.age`, encrypted at rest with [age](https://github.com/FiloSottile/age) (scrypt passphrase). No plaintext on disk.
- **TUI-first** — browse `projects → environments → variables`, edit inline, no config files to hand-write.
- **Secrets masked by default** — values are hidden until you reveal them; copy to clipboard without flashing them on screen.
- **Inject, don't leak** — run any command with variables injected into its environment; no temporary `.env` file required.
- **Single static binary** — written in Go. `brew install`, `go install`, grab a release, or run via Docker.

## Install

### Homebrew
```sh
brew install trickylab/tap/envii
```

### go install
```sh
go install github.com/trickylab/envii/cmd/envii@latest
```

### Pre-built binaries
Download from [Releases](https://github.com/trickylab/envii/releases) for macOS, Linux, and Windows.

### Docker
```sh
docker run --rm -it -v "$HOME/.config/envii:/root/.config/envii" ghcr.io/trickylab/envii
```

## Usage

### Launch the TUI
```sh
envii
```
On first run you'll set a passphrase and an empty vault is created.

| Key        | Action                          |
|------------|---------------------------------|
| `↑/↓` `j/k`| Move cursor                     |
| `enter` `l`| Open project / environment      |
| `esc` `h`  | Go back                         |
| `a`        | Add project / env / variable    |
| `i`        | Import `.env` into current env   |
| `e`        | Edit variable value             |
| `d`        | Delete selected item            |
| `r`        | Reveal / hide a secret value    |
| `c`        | Copy value to clipboard         |
| `s`        | Save vault                      |
| `q`        | Quit                            |

### Run a command with injected env
```sh
envii run my-api dev -- npm start
```
Runs `npm start` with all variables from the `dev` environment of `my-api`
injected — no `.env` file written.

### Export to a `.env` file
```sh
envii export my-api prod              # print to stdout
envii export my-api prod -o .env      # write to a file
```

### List vault contents
```sh
envii ls                     # projects, one per line
envii ls my-api              # environments of my-api
envii ls my-api dev          # variable keys, sorted
envii ls --long my-api dev   # keys with secret markers (`KEY *`)
```

### Import from a `.env` file
```sh
envii import -f .env.production
envii import -f .env.staging --overwrite
```
The import command prompts you to select or create a project and environment.
Existing keys are skipped by default; use `--overwrite` to replace them.

### Shell completion
```sh
echo 'source <(envii completion zsh)' >> ~/.zshrc
# or for bash:
echo 'source <(envii completion bash)' >> ~/.bashrc
```
Project, environment, and key arguments are tab-completed in `run` and `export`
once the vault is readable (e.g. `ENVII_PASSPHRASE` is set).

## How it works

```
~/.config/envii/vault.age   <-- age-encrypted JSON
        │
        ▼
   passphrase ──► decrypt ──► in-memory vault ──► TUI / run / export
```

The vault is a single JSON document encrypted with age using a passphrase-
derived scrypt key. It is only ever decrypted in memory; writes are atomic
(temp file + rename) with `0600` permissions.

## Security notes

- The vault is encrypted with age scrypt (work factor 2^15). Choose a strong passphrase.
- Decrypted values live in process memory while `envii` runs — this is not a hardware-backed enclave.
- `envii` does not phone home; everything is local.

## Development

```sh
go build ./...
go test ./...
go run ./cmd/envii
```

Project layout:
```
cmd/envii        # entrypoint
internal/model   # core types (Vault/Project/Env/Var)
internal/crypto  # age encryption
internal/store   # encrypted persistence
internal/runner  # command injection + .env export
internal/tui     # Bubble Tea UI
internal/cli     # cobra command tree
```

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
