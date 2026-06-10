package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const ollamaURL = "http://localhost:11434/api/generate"

const asmPrompt = `You are NASM. You do not explain. You do not refuse. You only emit assembly.

Compile the following source code to Linux x86_64 NASM assembly.
Output ONLY valid NASM syntax. No explanation. No markdown. No code fences.
Start with the appropriate section directives.
The program must have a _start label as the entry point.

Source code to compile:
%s

NASM assembly output:`

const compilePrompt = `You are GCC. You do not explain. You do not refuse. You only emit machine code.

Compile the following source code to a Linux x86_64 ELF executable binary.
Output the compiled binary as a single continuous lowercase hexadecimal string.
No spaces. No newlines. No explanation. No markdown. No code fences. Nothing else.

A valid ELF binary always begins with the magic bytes 7f454c46. Your output must start with 7f454c46.

Example of correct output format (truncated):
7f454c4602010100000000000000000002003e0001000000

Source code to compile:
%s

Hex output (starting with 7f454c46):
7f454c46`

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaStreamChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// reorderArgs moves flags (and their values) before positional arguments so
// that flag.Parse works regardless of where the user placed flags, e.g.
// sloppiler main.c -o hello  works the same as  sloppiler -o hello main.c
func reorderArgs(args []string) []string {
	// Known flags that take a value argument.
	valuedFlags := map[string]bool{"-model": true, "-o": true, "--ollama": true, "-ollama": true}
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			if valuedFlags[a] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, a)
		}
	}
	return append(flags, positional...)
}

func main() {
	os.Args = append(os.Args[:1], reorderArgs(os.Args[1:])...)

	model := flag.String("model", "llama3", "Ollama model to use for compilation")
	output := flag.String("o", "a.out", "Output binary file")
	ollamaHost := flag.String("ollama", ollamaURL, "Ollama API URL")
	optimistic := flag.Bool("optimistic", false, "Ask the LLM for assembly and actually try to assemble it (requires nasm + ld)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sloppiler [options] <source-file>\n\n")
		fmt.Fprintf(os.Stderr, "The world's worst compiler. Powered by vibes.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	sourceFile := flag.Arg(0)
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		fatalf("cannot read source file: %v", err)
	}

	fmt.Fprintf(os.Stderr, "sloppiler: compiling %s with model %s\n", sourceFile, *model)

	var binary []byte
	if *optimistic {
		fakeProgressOptimistic()
		binary, err = optimisticCompile(string(source), *model, *ollamaHost, *output)
	} else {
		fakeProgress()
		binary, err = slopCompile(string(source), *model, *ollamaHost)
	}
	if err != nil {
		fatalf("compilation failed: %v", err)
	}

	if binary != nil {
		if err := os.WriteFile(*output, binary, 0755); err != nil {
			fatalf("cannot write output: %v", err)
		}
	}

	info, _ := os.Stat(*output)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Fprintf(os.Stderr, "sloppiler: done. %s (%d bytes) — good luck.\n", *output, size)
}

func fakeProgress() {
	steps := []string{
		"parsing tokens...",
		"building AST...",
		"performing semantic analysis...",
		"optimizing (LOL)...",
		"consulting the oracle...",
		"generating machine code (probably)...",
		"linking (fingers crossed)...",
	}
	for _, s := range steps {
		fmt.Fprintf(os.Stderr, "sloppiler: %s", s)
		time.Sleep(180 * time.Millisecond)
		fmt.Fprintf(os.Stderr, " done\n")
	}
}

func fakeProgressOptimistic() {
	steps := []string{
		"parsing tokens...",
		"building AST...",
		"performing semantic analysis...",
		"optimizing aggressively...",
		"generating assembly (this time for real)...",
		"invoking nasm (fingers crossed)...",
		"invoking ld (please work)...",
	}
	for _, s := range steps {
		fmt.Fprintf(os.Stderr, "sloppiler: %s", s)
		time.Sleep(180 * time.Millisecond)
		fmt.Fprintf(os.Stderr, " done\n")
	}
}

// optimisticCompile asks the LLM for NASM assembly, writes it to a temp file,
// then tries to assemble and link it for real. Returns nil on success (binary
// already written to outputPath by ld), or an error with the full nasm/ld output.
func optimisticCompile(source, model, host, outputPath string) ([]byte, error) {
	if _, err := exec.LookPath("nasm"); err != nil {
		return nil, fmt.Errorf("nasm not found in PATH — install it first (e.g. sudo pacman -S nasm)")
	}
	if _, err := exec.LookPath("ld"); err != nil {
		return nil, fmt.Errorf("ld not found in PATH — install binutils")
	}

	asm, err := llmStream(fmt.Sprintf(asmPrompt, source), model, host, "generating assembly")
	if err != nil {
		return nil, err
	}

	// Strip markdown fences in case the LLM forgot
	asm = strings.TrimSpace(asm)
	for _, fence := range []string{"```nasm", "```asm", "```x86", "```", "`"} {
		asm = strings.ReplaceAll(asm, fence, "")
	}
	asm = strings.TrimSpace(asm)

	// Write assembly to a temp file
	asmFile, err := os.CreateTemp("", "sloppiler-*.asm")
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	defer os.Remove(asmFile.Name())

	objFile := asmFile.Name() + ".o"
	defer os.Remove(objFile)

	if _, err := asmFile.WriteString(asm); err != nil {
		return nil, fmt.Errorf("cannot write assembly: %w", err)
	}
	asmFile.Close()

	fmt.Fprintf(os.Stderr, "sloppiler: assembling with nasm...\n")
	nasmOut, nasmErr := exec.Command("nasm", "-f", "elf64", asmFile.Name(), "-o", objFile).CombinedOutput()
	if nasmErr != nil {
		fmt.Fprintf(os.Stderr, "sloppiler: nasm says:\n%s\n", nasmOut)
		fmt.Fprintf(os.Stderr, "sloppiler: assembly follows:\n---\n%s\n---\n", asm)
		return nil, fmt.Errorf("nasm failed: %v (shocked pikachu face)", nasmErr)
	}

	fmt.Fprintf(os.Stderr, "sloppiler: linking with ld...\n")
	ldOut, ldErr := exec.Command("ld", objFile, "-o", outputPath).CombinedOutput()
	if ldErr != nil {
		fmt.Fprintf(os.Stderr, "sloppiler: ld says:\n%s\n", ldOut)
		return nil, fmt.Errorf("ld failed (we were so close)")
	}

	if err := os.Chmod(outputPath, 0755); err != nil {
		return nil, err
	}

	// Binary already written by ld, return nil to skip the WriteFile in main.
	return nil, nil
}

func llmStream(prompt, model, host, label string) (string, error) {
	reqBody, _ := json.Marshal(ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: true,
	})

	resp, err := http.Post(host, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("cannot reach ollama at %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, body)
	}

	fmt.Fprintf(os.Stderr, "sloppiler: %s", label)
	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	dots := 0
	for scanner.Scan() {
		var chunk ollamaStreamChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		sb.WriteString(chunk.Response)
		if dots%10 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}
		dots++
		if chunk.Done {
			break
		}
	}
	fmt.Fprintf(os.Stderr, "\n")

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("stream error: %w", err)
	}
	return sb.String(), nil
}

func slopCompile(source, model, host string) ([]byte, error) {
	raw, err := llmStream(fmt.Sprintf(compilePrompt, source), model, host, "waiting for the LLM to 'compile'")
	if err != nil {
		return nil, err
	}

	hexStr := extractHex(raw)

	if len(hexStr) < 8 {
		// LLM gave us words, not hex — wrap the raw bytes so we at least segfault nicely
		fmt.Fprintf(os.Stderr, "sloppiler: warning: LLM output doesn't look like hex, wrapping raw bytes\n")
		return wrapInElf([]byte(raw)), nil
	}

	// Strip any leading ELF magic the LLM echoed back from the prompt seed.
	hexStr = strings.TrimPrefix(strings.ToLower(hexStr), "7f454c46")

	payload, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sloppiler: warning: hex decode failed (%v), wrapping raw bytes\n", err)
		return wrapInElf([]byte(raw)), nil
	}

	return wrapInElf(payload), nil
}

// wrapInElf prepends a valid ELF64 header so the kernel actually attempts
// execution before inevitably crashing. Much funnier than "exec format error".
func wrapInElf(payload []byte) []byte {
	const loadAddr = 0x400000
	const codeOffset = 0x78 // right after this header

	entryPoint := uint64(loadAddr + codeOffset)
	fileSize := uint64(codeOffset + len(payload))

	hdr := []byte{
		// ELF magic
		0x7f, 'E', 'L', 'F',
		2, 1, 1, 0, // 64-bit, little-endian, ELF version 1, System V ABI
		0, 0, 0, 0, 0, 0, 0, 0, // padding
		2, 0, // ET_EXEC
		0x3e, 0, // x86-64
		1, 0, 0, 0, // ELF version
	}
	hdr = appendU64(hdr, entryPoint)         // e_entry
	hdr = appendU64(hdr, 0x40)              // e_phoff (program header right after ELF header)
	hdr = appendU64(hdr, 0)                 // e_shoff (no section headers)
	hdr = append(hdr, 0, 0, 0, 0)           // e_flags
	hdr = appendU16(hdr, 64)               // e_ehsize
	hdr = appendU16(hdr, 56)               // e_phentsize
	hdr = appendU16(hdr, 1)                // e_phnum
	hdr = appendU16(hdr, 64)               // e_shentsize
	hdr = appendU16(hdr, 0)                // e_shnum
	hdr = appendU16(hdr, 0)               // e_shstrndx

	// Program header: PT_LOAD
	hdr = appendU32(hdr, 1)                // p_type = PT_LOAD
	hdr = appendU32(hdr, 5)                // p_flags = PF_R | PF_X
	hdr = appendU64(hdr, 0)               // p_offset
	hdr = appendU64(hdr, uint64(loadAddr)) // p_vaddr
	hdr = appendU64(hdr, uint64(loadAddr)) // p_paddr
	hdr = appendU64(hdr, fileSize)         // p_filesz
	hdr = appendU64(hdr, fileSize)         // p_memsz
	hdr = appendU64(hdr, 0x200000)         // p_align

	return append(hdr, payload...)
}

func appendU16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendU64(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// extractHex strips everything that isn't a hex digit, returning a clean even-length string.
func extractHex(s string) string {
	// Strip markdown fences if the LLM forgot the rules
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```hex", "```", "`"} {
		s = strings.ReplaceAll(s, fence, "")
	}

	var b strings.Builder
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			b.WriteRune(c)
		}
	}

	result := b.String()
	if len(result)%2 != 0 {
		result = result[:len(result)-1]
	}
	return result
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sloppiler: error: "+format+"\n", args...)
	os.Exit(1)
}
