# Sloppiler
### *Compile Smarter. Not Correctly.*

> "We didn't reinvent the compiler. We asked an AI to guess what one looks like."
> — Sloppiler Engineering Blog, Issue 1 (Final)

**Sloppiler** is a next-generation, AI-first, LLM-native compilation solution that leverages the full generative power of large language models to transform your source code into a binary-adjacent artifact — in seconds.

Traditional compilers are slow, opinionated, and frankly elitist. They *parse*. They *typecheck*. They *care*. Sloppiler doesn't. Sloppiler **vibes** your code into existence using a local language model, skipping the pedantic intermediate steps that have held software engineering back for decades.

**Key differentiators:**
- Zero determinism — every build is unique
- Blazing-fast failure
- Segfault-as-a-feature architecture
- 100% hallucination-driven code generation
- Runs entirely on-premise (the LLM has no idea what it's doing, but at least it's your LLM)

*Sloppiler is backed by nobody and recommended by no one.*

## Requirements

- [Ollama](https://ollama.com) running locally with a model pulled
- Go 1.21+ to build
- `nasm` + `ld` (binutils) for `--optimistic` mode only

## Build

```sh
go build -o sloppiler .
```

Or use the helper script which handles everything:

```sh
./run.sh <source-file> [output]
MODEL=codellama ./run.sh main.c hello
```

## Usage

```sh
./sloppiler [options] <source-file>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-model` | `llama3` | Ollama model to use |
| `-o` | `a.out` | Output binary path |
| `-ollama` | `http://localhost:11434/api/generate` | Ollama API URL |
| `--optimistic` | false | Ask the LLM for assembly and actually try to assemble it |

## Modes

### Default mode

Asks the LLM to output the binary directly as a hex string. The LLM will produce something that looks vaguely like binary. A valid ELF header is prepended so the kernel will at least attempt to execute it before inevitably crashing.

```sh
./sloppiler -model codellama main.c -o hello
./hello
# zsh: segmentation fault (core dumped) ./hello
```

### Optimistic mode (`--optimistic`)

Asks the LLM for NASM assembly instead, then pipes it through `nasm` and `ld` for real. The assembly will look structurally correct and be completely wrong in ways that are only visible at runtime.

```sh
./sloppiler --optimistic -model codellama main.c -o hello
./hello
# zsh: segmentation fault (core dumped) ./hello
```

## Model recommendations

| Model | Behaviour |
|-------|-----------|
| `codellama` | Commits to the bit. Produces binary that looks almost intentional. Best for `--optimistic`. |
| `phi3` | Writes poetry disguised as assembly. Hallucinates MASM syntax in NASM files. Fastest. |
| `llama3` | Explains why it can't compile your code instead of just doing it wrong. |

## Track record

| Mode | Model | Output | Result |
|------|-------|--------|--------|
| default | phi3 | `is nothealing.!` wrapped in ELF | segfault |
| default | codellama | nested ELF headers | segfault |
| optimistic | phi3 | `assembly` on line 1 | nasm error |
| optimistic | codellama | valid binary, `Hello, world!` printed | worked! |



---

## Disclaimer
This clearly is a joke and doesn't really work well. I am not responsible for you bricking your pc.