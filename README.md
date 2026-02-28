# scg

A fast, native [Scoop](https://scoop.sh) CLI for Windows, written in Go.

## Installation

```powershell
scoop bucket add amrkmn https://github.com/amrkmn/bucket
scoop install amrkmn/scg
```

Or build from source:

```powershell
git clone https://github.com/amrkmn/scg
cd scg
go build -ldflags "-X main.Version=0.1.0 -s -w" -o dist/scg.exe ./cmd
```

## Commands

| Command | Description |
|---|---|
| `scg list` | List installed apps |
| `scg search <query>` | Search for apps across all buckets |
| `scg info <app>` | Show information about an app |
| `scg status` | Show update status for installed apps |
| `scg prefix <app>` | Show the install prefix of an app |
| `scg which <app>` | Show the path to an executable managed by Scoop |
| `scg cleanup` | Remove old versions of installed apps |
| `scg config` | Get, set, or delete configuration values |
| `scg version` | Show the scg version |
| `scg completion` | Generate PowerShell autocompletion script |

### Bucket management

| Command | Description |
|---|---|
| `scg bucket list` | List installed buckets |
| `scg bucket add <name> [url]` | Add a bucket |
| `scg bucket remove <name>` | Remove a bucket |
| `scg bucket update [name...]` | Update buckets |
| `scg bucket known` | List known buckets |
| `scg bucket unused` | List buckets with no installed apps |

## PowerShell completion

To load completions in your current session:

```powershell
scg completion | Out-String | Invoke-Expression
```

To load completions for every new session, add the following line to your PowerShell profile:

```powershell
Invoke-Expression (scg completion | Out-String)
```

To write it to your profile automatically:

```powershell
Add-Content $PROFILE "`nInvoke-Expression (scg completion | Out-String)"
```

## License

[Apache-2.0](LICENSE)
