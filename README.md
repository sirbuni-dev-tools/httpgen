# httpgen

httpgen reads the Java source of a Spring Boot API and writes ready-to-run request files for the IntelliJ HTTP Client, plus JSON request bodies filled with realistic fake values. Point it at a controller, a GraphQL resolver or a whole source folder, and it writes one request per endpoint: the URL, the parameters, the headers and a body you can send straight away.

It understands:

- **Spring MVC REST controllers**: `@RestController`, `@Controller` and `@RequestMapping` classes with `@GetMapping`, `@PostMapping`, `@PutMapping`, `@PatchMapping`, `@DeleteMapping` and `@RequestMapping` methods.
- **SPQR GraphQL resolvers** (code-first GraphQL): `@GraphQLQuery` and `@GraphQLMutation` methods, with argument and type names worked out the way SPQR names them in the schema.

It is a single program with no dependencies: no IDE, no JVM and no running application are needed to generate the files. You only need the API running when you send the requests.

## Contents

- [Why use it](#why-use-it)
- [What it produces](#what-it-produces)
- [Installation](#installation)
  - [Which download do I need?](#which-download-do-i-need)
  - [macOS](#macos)
  - [Linux](#linux)
  - [Windows](#windows)
  - [With Go](#with-go)
  - [Check the installation](#check-the-installation)
- [Quick start](#quick-start)
- [Usage](#usage)
  - [List the API: --list](#list-the-api---list)
  - [Request files: --req](#request-files---req)
  - [JSON payloads: --json](#json-payloads---json)
  - [Files as Base64: --base64](#files-as-base64---base64)
  - [Options reference](#options-reference)
  - [Built-in help and man page](#built-in-help-and-man-page)
- [Running the generated requests](#running-the-generated-requests)
- [How fake values are chosen](#how-fake-values-are-chosen)
- [For scripts and AI agents](#for-scripts-and-ai-agents)
- [Troubleshooting](#troubleshooting)
- [Updating](#updating)
- [Uninstalling](#uninstalling)
- [Development](#development)
- [License](#license)

## Why use it

Writing requests by hand means reading every controller, copying paths, remembering which parameters are path variables and which are query parameters, and inventing a body for each DTO. httpgen does that in one command and keeps doing it as the API grows:

- **One request per endpoint**, with a title and a unique `# @name`, ready to run.
- **Bodies with realistic values**: a field called `email` gets an email address, `phoneNumber` a phone number, `@Size(max = 10)` a value that fits, an enum one of its constants.
- **Safe to run again**: new endpoints are appended, and requests you already edited are left exactly as they are.
- **Ids you control**: path variables and ids become variables at the top of the file (`@uid = ...`), so an update or delete never hits a random record.
- **GraphQL ready**: each resolver file starts with a login request that stores the access token for the requests below it.
- **Same output everywhere**: macOS, Linux and Windows produce the same files.

## What it produces

Given this controller:

```java
@RestController
@RequestMapping("/api/departments")
public class DepartmentController {
    @GetMapping
    public List<DepartmentDto> search(@RequestParam(required = false) String name) { ... }

    @GetMapping("/{uid}")
    public DepartmentDto findByUid(@PathVariable UUID uid) { ... }

    @PostMapping
    public DepartmentDto create(@RequestBody DepartmentDto department) { ... }

    @DeleteMapping("/{uid}")
    public void delete(@PathVariable UUID uid) { ... }
}
```

and this DTO:

```java
@Data
public class DepartmentDto {
    private UUID uid;
    @NotBlank @Size(max = 10)
    private String code;
    @NotBlank
    private String name;
    @Email
    private String contactEmail;
    private Integer headcount;
    private LocalDate established;
    private DepartmentStatus status;   // enum: ACTIVE, MERGED, CLOSED
}
```

`httpgen --req` writes `.local/http/DepartmentController.http`:

```http
@uid = 308dc45c-b3d4-41bf-9428-1087809cb78d

### GET search
# @name search
GET {{HOST}}/api/departments
    ?name={{$random.commerce.department}}

### GET find by uid
# @name findByUid
GET {{HOST}}/api/departments/{{uid}}

### POST create
# @name create
POST {{HOST}}/api/departments
Content-Type: application/json

{
  "uid": null,
  "code": "{{$random.alphabetic(5)}}",
  "name": "{{$random.commerce.department}}",
  "contactEmail": "{{$random.internet.emailAddress}}",
  "headcount": {{$random.integer(1, 1001)}},
  "established": "2026-08-02",
  "status": "CLOSED"
}

### DELETE delete
# @name delete
DELETE {{HOST}}/api/departments/{{uid}}
```

Things to notice:

- `{{HOST}}` comes from an environment file httpgen writes next to the requests, with the port read from your `application.properties` or `application.yml`.
- `{{$random...}}` values are generated fresh by the HTTP Client every time you send the request.
- The record's own `uid` is `null` in the create body: the server assigns it.
- Set `@uid` once at the top to the record you want to read or delete.

For a GraphQL resolver, the requests look like this (the login request at the top of the file is left out here):

```http
### GRAPHQL department by uid
# @name departmentByUid
GRAPHQL {{HOST}}/graphql
Authorization: Bearer {{access_token}}

query DepartmentByUid($uid: UUID!) {
  departmentByUid(uid: $uid) {
    uid
    code
    name
    contactEmail
    headcount
    established
    status
  }
}

{
  "uid": "{{uid}}"
}
```

## Installation

httpgen is one executable file. Pick your operating system below; each section starts with the easiest method.

### Which download do I need?

Every release is on the [Releases page](https://github.com/sirbuni-dev-tools/httpgen/releases). The file names follow `httpgen_<version>_<system>_<processor>`:

| Your computer | File to download |
|---|---|
| Mac with Apple Silicon (M1, M2, M3, M4 and later) | `httpgen_<version>_darwin_arm64.tar.gz` |
| Mac with an Intel processor | `httpgen_<version>_darwin_amd64.tar.gz` |
| Linux on a usual PC or server (x86-64) | `.deb`, `.rpm` or `.tar.gz` ending in `linux_amd64` |
| Linux on 64-bit ARM (Raspberry Pi OS 64-bit, AWS Graviton) | `.deb`, `.rpm` or `.tar.gz` ending in `linux_arm64` |
| Windows 10 or 11 | `httpgen_<version>_windows_amd64.zip` (also runs on Windows 11 on ARM, through its x64 emulation) |

Not sure which processor you have?

- **macOS**: Apple menu > About This Mac. "Chip: Apple M..." means `arm64`; "Processor: Intel" means `amd64`. In Terminal, `uname -m` prints `arm64` or `x86_64` (`x86_64` means `amd64`).
- **Linux**: run `uname -m`. `x86_64` means `amd64`; `aarch64` means `arm64`.

### macOS

#### Option 1: Homebrew (recommended)

[Homebrew](https://brew.sh) is the usual way to install command line tools on a Mac, and it handles updates for you. If `brew --version` already works in your Terminal, skip the first command.

Install Homebrew (it asks for your Mac password and explains each step):

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Then install httpgen:

```bash
brew install --cask sirbuni-dev-tools/tap/httpgen
```

That one command adds the `sirbuni-dev-tools/tap` source to Homebrew, installs `httpgen` on your PATH and installs its man page.

#### Option 2: Download the file yourself

Open Terminal (Applications > Utilities > Terminal) and run these lines one at a time. Change `VERSION` to the latest version shown on the Releases page, and `ARCH` to `amd64` on an Intel Mac.

```bash
VERSION=0.1.0
ARCH=arm64
curl -LO "https://github.com/sirbuni-dev-tools/httpgen/releases/download/v${VERSION}/httpgen_${VERSION}_darwin_${ARCH}.tar.gz"
tar -xzf "httpgen_${VERSION}_darwin_${ARCH}.tar.gz"
mkdir -p ~/.local/bin
mv httpgen ~/.local/bin/
```

Make sure `~/.local/bin` is on your PATH (the list of folders your Terminal searches for commands). This adds it for the default zsh shell, then reloads the settings:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

If you downloaded the file with a web browser instead of `curl`, macOS marks it as downloaded from the internet and refuses to open it, because the binary is not signed by Apple yet. Clear that mark once:

```bash
xattr -d com.apple.quarantine ~/.local/bin/httpgen
```

Optional, to read the manual with `man httpgen`:

```bash
sudo mkdir -p /usr/local/share/man/man1 && sudo cp man/httpgen.1.gz /usr/local/share/man/man1/
```

### Linux

#### Option 1: .deb package (Debian, Ubuntu, Linux Mint, Pop!_OS)

Check your architecture with `dpkg --print-architecture` (`amd64` or `arm64`), then:

```bash
VERSION=0.1.0
ARCH=amd64
curl -LO "https://github.com/sirbuni-dev-tools/httpgen/releases/download/v${VERSION}/httpgen_${VERSION}_linux_${ARCH}.deb"
sudo apt install "./httpgen_${VERSION}_linux_${ARCH}.deb"
```

The `./` matters: it tells `apt` to install the file in the current folder instead of searching online. The package puts `httpgen` in `/usr/bin` and the manual in `/usr/share/man/man1`.

#### Option 2: .rpm package (Fedora, RHEL, Rocky Linux, AlmaLinux, openSUSE)

```bash
VERSION=0.1.0
ARCH=amd64
curl -LO "https://github.com/sirbuni-dev-tools/httpgen/releases/download/v${VERSION}/httpgen_${VERSION}_linux_${ARCH}.rpm"
sudo dnf install "./httpgen_${VERSION}_linux_${ARCH}.rpm"
```

On openSUSE use `sudo zypper install` instead of `sudo dnf install`.

#### Option 3: Homebrew on Linux

If you already use [Homebrew on Linux](https://brew.sh), the macOS command works here too:

```bash
brew install --cask sirbuni-dev-tools/tap/httpgen
```

#### Option 4: Download the file yourself (any distribution)

```bash
VERSION=0.1.0
ARCH=amd64
curl -LO "https://github.com/sirbuni-dev-tools/httpgen/releases/download/v${VERSION}/httpgen_${VERSION}_linux_${ARCH}.tar.gz"
tar -xzf "httpgen_${VERSION}_linux_${ARCH}.tar.gz"
mkdir -p ~/.local/bin
mv httpgen ~/.local/bin/
```

Most distributions already have `~/.local/bin` on the PATH after you log in again. If `httpgen --version` says "command not found", add it for bash:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Windows

#### Option 1: Scoop (recommended)

[Scoop](https://scoop.sh) installs command line tools for your user account only, without administrator rights, and puts them on your PATH for you.

Open **PowerShell** (press the Windows key, type `PowerShell`, press Enter; do not choose "Run as administrator"). If `scoop --version` already works, skip to the last two commands.

Allow PowerShell to run the Scoop installer, then install Scoop:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

```powershell
irm get.scoop.sh | iex
```

Add the httpgen bucket (the list Scoop installs httpgen from) and install it:

```powershell
scoop bucket add sirbuni-dev-tools https://github.com/sirbuni-dev-tools/scoop-bucket
```

```powershell
scoop install sirbuni-dev-tools/httpgen
```

#### Option 2: Download the file yourself

1. Download `httpgen_<version>_windows_amd64.zip` from the [Releases page](https://github.com/sirbuni-dev-tools/httpgen/releases).
2. Right-click the zip, choose **Extract All...**, and extract it to a folder you will keep, for example `C:\Users\<you>\AppData\Local\Programs\httpgen`.
3. Add that folder to your PATH:
   1. Press the Windows key, type `environment variables`, and open **Edit environment variables for your account**.
   2. Under **User variables**, select **Path** and click **Edit...**.
   3. Click **New**, paste the folder path from step 2, and click **OK** on every window.
4. Close and reopen PowerShell or Command Prompt; a window that was already open does not see the new PATH.

The same steps in PowerShell, run from the folder that holds the downloaded zip:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\httpgen"
New-Item -ItemType Directory -Force $dir | Out-Null
Expand-Archive .\httpgen_0.1.0_windows_amd64.zip -DestinationPath $dir -Force
Unblock-File "$dir\httpgen.exe"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

`Unblock-File` removes the "downloaded from the internet" mark, so Windows does not warn about the file each time. Open a new PowerShell window afterwards.

### With Go

If you have [Go](https://go.dev/dl/) 1.26 or newer:

```bash
go install github.com/sirbuni-dev-tools/httpgen/cmd/httpgen@latest
```

The program lands in `$(go env GOPATH)/bin`, which must be on your PATH. A Go build reports its version as `dev`; use one of the methods above if you want the release number in `httpgen --version`.

### Check the installation

Open a new terminal window and run:

```bash
httpgen --version
```

```
httpgen 0.1.0 (commit cc3c48e, built 2026-10-28T05:31:19Z)
```

If you see "command not found" (macOS, Linux) or "is not recognized as an internal or external command" (Windows), see [Troubleshooting](#troubleshooting).

**Optional: check the download is genuine.** Each release has a `checksums.txt` file. Download it into the same folder as your file, then compare:

```bash
# macOS
grep darwin_arm64.tar.gz checksums.txt | shasum -a 256 -c -
```

```bash
# Linux
sha256sum --ignore-missing -c checksums.txt
```

```powershell
# Windows: compare the printed hash with the line for your file in checksums.txt
Get-FileHash .\httpgen_0.1.0_windows_amd64.zip -Algorithm SHA256
```

The macOS and Linux commands print `OK` next to the file name when it matches.

## Quick start

From the root of your Spring Boot project:

```bash
# 1. See what httpgen finds
httpgen --list src/main/java

# 2. Preview the requests for one controller without writing anything
httpgen --req src/main/java/com/example/hr/web/DepartmentController.java --stdout

# 3. Write request files for every controller and resolver
httpgen --req src/main/java
```

Then open the files under `.local/http/` in IntelliJ IDEA, choose the `local` environment, start your application and click the run arrow next to a request. See [Running the generated requests](#running-the-generated-requests).

Add `.local/` to your project's `.gitignore`: it holds generated files and the private environment file with your password.

## Usage

Every command takes a `.java` file or a folder. With a folder, httpgen looks at every `.java` file inside it.

### List the API: --list

```bash
httpgen --list src/main/java
```

```
query     departmentByUid         DepartmentResolver.find
mutation  saveDepartment          DepartmentResolver.save
GET       /api/departments        DepartmentController.search
GET       /api/departments/{uid}  DepartmentController.findByUid
POST      /api/departments        DepartmentController.create
DELETE    /api/departments/{uid}  DepartmentController.delete
```

Add `--format json` for a JSON array with the kind, verb, path or operation name, class, method, file and line of each entry.

### Request files: --req

```bash
httpgen --req src/main/java
```

```
.local/http/DepartmentResolver.http: added login, departmentByUid, saveDepartment
.local/http/http-client.env.json: updated
.local/http/http-client.private.env.json: updated
.local/http/DepartmentController.http: added search, findByUid, create, delete
```

What it writes, for each controller or resolver:

- **`<module>/.local/http/<Class>.http`**, where `<module>` is the nearest folder above the source file that holds `pom.xml`, `build.gradle` or `build.gradle.kts`. Multi-module projects get one `.local/http` per module.
- **`http-client.env.json`**, holding `HOST` (and the login settings for GraphQL), created or extended with missing keys only. `HOST` is `http://localhost:<server.port><server.servlet.context-path>` from your `application.properties` or `application.yml`, or `http://localhost:8080`.
- **`http-client.private.env.json`** (GraphQL only), holding `CLIENT_SECRET` and `PASSWORD`. Fill these in yourself, and never commit this file.

Running it again is safe:

```
.local/http/DepartmentResolver.http: nothing new, 3 already there
.local/http/DepartmentController.http: nothing new, 4 already there
```

A request counts as present when its request line (for GraphQL, its operation line) is already in the file, so your edits to headers and bodies are kept. Only new endpoints are appended.

Useful variations:

```bash
# Preview without writing anything
httpgen --req src/main/java --stdout

# Show what would change, without changing it
httpgen --req src/main/java --dry-run

# Only some methods of one controller
httpgen --req src/main/java/com/example/hr/web/DepartmentController.java --method create --method delete

# Fixed values instead of {{$random...}} placeholders
httpgen --req src/main/java --mode fixed

# Point HOST somewhere else when the env file is created
httpgen --req src/main/java --host http://localhost:9090
```

**GraphQL login.** A resolver's file starts with a login request:

```http
### POST login (run first; the token lasts until it expires)
# @name login
POST {{AUTH_URL}}
Authorization: Basic {{CLIENT_ID}} {{CLIENT_SECRET}}
Content-Type: application/x-www-form-urlencoded

grant_type={{GRANT_TYPE}}&
username={{USERNAME}}&
password={{PASSWORD}}

> {% client.global.set("access_token", response.body.access_token); %}
```

Run it once; it saves `access_token` for every GraphQL request below it. `AUTH_URL` defaults to `HOST/oauth2/token`; change it once in `http-client.env.json` if your login lives elsewhere. Leave the login request out with `--no-login`.

### JSON payloads: --json

For a class, `--json` writes one JSON document. For a controller or resolver, it writes one per endpoint: the request body, or the parameters as one object.

```bash
httpgen --json src/main/java/com/example/hr/dto/DepartmentDto.java --stdout
```

```json
{
  "uid": "be159df2-19ce-45f5-ba17-fa3e808dfb04",
  "code": "tdghv",
  "name": "Camping gear",
  "contactEmail": "emiliamills8@martinez.info",
  "headcount": 482,
  "established": "2027-03-24",
  "status": "MERGED"
}
```

Without `--stdout`, files go to `<module>/.local/json/<Class>.json` or `<Class>.<method>.json`. Values are fixed by default so the JSON pastes anywhere; `--mode placeholders` writes `{{$random...}}` placeholders instead, and `--seed <n>` makes the values repeatable.

### Files as Base64: --base64

APIs that accept uploads inside JSON expect the file as a Base64 string. httpgen encodes a file, or one of its built-in samples when you just need something valid to send: `pdf` (a one-page PDF), `csv`, `jpeg` (or `jpg`) and `png`.

```bash
httpgen --base64 ./contract.pdf
httpgen --base64 png --data-uri
httpgen --base64 pdf --json-field attachment
```

The last one prints a JSON object ready to paste into a request body:

```json
{
  "attachment": "JVBERi0xLjQKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyAv...",
  "attachmentName": "sample.pdf",
  "attachmentType": "application/pdf"
}
```

`--data-uri` adds the `data:<type>;base64,` prefix, `--wrap 76` breaks the output into lines, and `--out <file>` writes to a file.

### Options reference

One action per command:

| Action | What it does |
|---|---|
| `--req <path>` | Write or extend `.http` request files |
| `--json <path>` | Write JSON payloads with fake values |
| `--base64 <file or sample>` | Print a file, or a built-in sample, as Base64 |
| `--list <path>` | Print one line per endpoint or operation |
| `--help [topic]`, `-h` | Show help; `--help topics` lists every topic |
| `--man` | Print the whole manual as a man page |
| `--version` | Print the version, commit and build date |

Options:

| Option | Used with | Meaning | Default |
|---|---|---|---|
| `--method <name>` | `--req`, `--json` | Only this Java method; repeat for several | every method |
| `--class <name>` | `--req`, `--json`, `--list` | Only this class, top-level or nested | every class |
| `--out <path>` | `--req`, `--json`, `--base64` | Output folder (for `--base64`, an output file) | `<module>/.local/http` or `/json` |
| `--stdout` | `--req`, `--json` | Print instead of writing files | off |
| `--dry-run` | `--req` | Report what would change, change nothing | off |
| `--mode <placeholders\|fixed>` | `--req`, `--json` | `{{$random...}}` placeholders or fixed fake values | `placeholders` for `--req`, `fixed` for `--json` |
| `--seed <n>` | `--req`, `--json` | Repeatable fake values | random |
| `--depth <n>` | `--req`, `--json` | How deep nested objects and GraphQL selections go | 3 |
| `--host <url>` | `--req` | `HOST` for a new env file | from your application config |
| `--env <name>` | `--req` | Environment to create or extend in the env files | `local` |
| `--no-login` | `--req` | Leave out the GraphQL login request | off |
| `--data-uri` | `--base64` | Prefix with `data:<type>;base64,` | off |
| `--wrap <n>` | `--base64` | Line length; not allowed with `--json-field` | 0 (one line) |
| `--json-field <name>` | `--base64` | Print a JSON object with the value, file name and type | off |
| `--format <text\|json>` | `--req`, `--json`, `--list` | Report as text or JSON | `text` |
| `--quiet`, `-q` | `--req`, `--json`, `--base64` | Do not list the files written | off |
| `--no-pager` | `--help` | Print help directly instead of through a pager | off |

### Built-in help and man page

httpgen carries its full manual:

```bash
httpgen --help            # overview
httpgen --help req        # one page per action: req, json, base64, list
httpgen --req --help      # the same page
httpgen --help env        # the environment files and their keys
httpgen --help agents     # recipes for scripts and AI agents
httpgen --help topics     # every topic
```

Long pages open in your pager (`less`) in a terminal. Homebrew and the Linux packages install the man page, so `man httpgen` works there.

## Running the generated requests

The files use the IntelliJ HTTP Client format, including `{{$random...}}` placeholders, `GRAPHQL` requests and response scripts, so they run in:

- **IntelliJ IDEA**, with its built-in HTTP Client. Open a `.http` file, choose `local` in the **Run with** list at the top of the editor, and click the green arrow next to a request. For GraphQL, run `login` first.
- **ijhttp**, JetBrains' command line runner, for scripts and CI:

```bash
ijhttp --env-file .local/http/http-client.env.json --private-env-file .local/http/http-client.private.env.json --env local .local/http/DepartmentController.http
```

ijhttp is also published as a Docker image. When the API runs on your own machine, generate the files with `--host http://host.docker.internal:<port>` so the container can reach it:

```bash
docker run --rm -v "$PWD/.local/http":/workdir jetbrains/intellij-http-client --env-file http-client.env.json --env local DepartmentController.http
```

## How fake values are chosen

For every field, httpgen picks a value in this order:

1. **Validation annotations**: `@Email`, `@Pattern`, `@Size`, `@Min`, `@Max`, `@Positive`, `@Past`, `@Future` and similar.
2. **The field name**: `email`, `phoneNumber`, `firstName`, `city`, `amount`, `year`, `percentage` and many more, matched regardless of case and underscores, and on the last words of a longer name (`managerEmail` is an email).
3. **The Java type**: strings, numbers, booleans, dates and times, UUIDs, enums (one of their constants), lists, maps and nested objects.

A few rules make bodies work against a real API:

- A payload's own `id`, `uid` or `uuid` is `null`, and `@GeneratedValue` fields are left out of request bodies.
- A password and its confirmation get the same strong value.
- Paging starts at page 0 and sorting is left unset; search filter lists are empty.
- Path variables and ids are file variables you set, never random values.
- Nesting stops at `--depth` levels, and a class that refers back to itself is cut off rather than repeated.

## For scripts and AI agents

httpgen is built to be called from scripts and by AI coding agents, which spend far fewer tokens on one command than on reading controllers and writing requests:

```bash
# See the API as JSON
httpgen --list src/main/java --format json

# Get one endpoint's request without writing files
httpgen --req src/main/java/com/example/hr/web/DepartmentController.java --method create --stdout

# Check what a run would change
httpgen --req src/main/java --dry-run --format json
```

- Output is plain text when it is not going to a terminal, and help never opens a pager when piped.
- Exit codes are stable:

| Code | Meaning |
|---|---|
| 0 | Success, including when every request was already in its file |
| 1 | Nothing found to generate |
| 2 | Usage error, such as an unknown option or a missing file |
| 3 | A Java file could not be parsed; the file and line are reported |

## Troubleshooting

**"command not found: httpgen" (macOS, Linux) or "'httpgen' is not recognized" (Windows)**

The folder holding `httpgen` is not on your PATH, or your terminal was opened before you changed it. Open a new terminal first. If it still fails, check where you put the file and follow the PATH steps for your system under [Installation](#installation). With Scoop, run `scoop which httpgen` to confirm it is installed.

**macOS: "httpgen cannot be opened because the developer cannot be verified"**

The binary is not signed by Apple yet, and macOS blocks unsigned files downloaded with a browser. Homebrew clears this for you. For a manual download, run (with your own path):

```bash
xattr -d com.apple.quarantine ~/.local/bin/httpgen
```

Or open System Settings > Privacy & Security, scroll down, and click **Open Anyway** next to the message about httpgen.

**Windows: "Windows protected your PC"**

SmartScreen warns about new programs downloaded from the internet. Run `Unblock-File` on `httpgen.exe` as shown in the Windows manual steps, or click **More info** > **Run anyway**. Scoop installs are not affected.

**"no Spring request mappings or SPQR resolvers found" (exit code 1)**

The path holds no controller or resolver methods httpgen recognizes. Run `httpgen --list <path>` to see what it finds, check that you pointed it at the source folder (usually `src/main/java`), and that `--method` and `--class` match the Java names exactly.

**A GraphQL query has no fields to select**

SPQR exposes a field through a getter, or a Lombok `@Getter` or `@Data` on the class. A DTO with private fields and no getters has nothing to select, so httpgen writes the operation without a selection. Add getters or Lombok to the DTO, the same fix SPQR itself needs.

**"cannot parse ..." (exit code 3)**

A Java file could not be read; the message names the file and line. httpgen reads declarations (classes, fields, method signatures and annotations), not method bodies, so this usually means the file does not compile either. If it does compile, please open an issue with the message.

## Updating

| Installed with | Update with |
|---|---|
| Homebrew | `brew update` then `brew upgrade --cask httpgen` |
| Scoop | `scoop update` then `scoop update httpgen` |
| `.deb` | Download the new `.deb` and run `sudo apt install ./<file>.deb` again |
| `.rpm` | Download the new `.rpm` and run `sudo dnf install ./<file>.rpm` again |
| Manual download | Download the new archive and replace the `httpgen` file in the same folder |
| Go | `go install github.com/sirbuni-dev-tools/httpgen/cmd/httpgen@latest` |

## Uninstalling

| Installed with | Remove with |
|---|---|
| Homebrew | `brew uninstall --cask httpgen`, and optionally `brew untap sirbuni-dev-tools/tap` |
| Scoop | `scoop uninstall httpgen`, and optionally `scoop bucket rm sirbuni-dev-tools` |
| `.deb` | `sudo apt remove httpgen` |
| `.rpm` | `sudo dnf remove httpgen` |
| Manual download | Delete the `httpgen` file (`httpgen.exe` on Windows) and remove its folder from your PATH |
| Go | `rm "$(go env GOPATH)/bin/httpgen"` |

httpgen keeps no settings of its own. The `.local/http` and `.local/json` folders it wrote inside your projects stay until you delete them.

## Development

Needs Go 1.26 or newer.

```bash
make test build
```

The binary lands in `bin/httpgen`. Other targets: `make vet`, `make fmt`, and `make sync-rules`, which copies the shared fake-value rules from the API HTTP Generator IntelliJ plugin.

**Branches and releases**

- Work happens on feature branches and is merged into `dev` through pull requests. CI on `dev` runs formatting, vet, the tests and a full snapshot build, and never publishes anything.
- Merging `dev` into `main` releases. The version comes from the [Conventional Commits](https://www.conventionalcommits.org) since the last release:

| Commit type | Release |
|---|---|
| `fix:` | patch, such as 0.1.0 to 0.1.1 |
| `feat:` | minor, such as 0.1.0 to 0.2.0 |
| `feat!:` or a `BREAKING CHANGE:` footer | major |
| `docs:`, `chore:`, `ci:`, `refactor:`, `test:` only | no release |

A release publishes the archives, the `.deb` and `.rpm` packages and checksums to GitHub Releases, and updates the Homebrew cask and the Scoop manifest.

## License

MIT. See [LICENSE](LICENSE).
