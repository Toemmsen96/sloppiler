package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Toemmsen96/sloppiler/providers"
)

// ── ANSI colours ─────────────────────────────────────────────────────────────

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	red    = "\033[31m"
)

// ── Prompts ───────────────────────────────────────────────────────────────────

// asmPromptFor returns a target-specific NASM prompt.
func asmPromptFor(target, source string) string {
	var rules, header string
	switch target {
	case "windows":
		header = "Windows x86_64 PE executable (NASM win64 format)"
		rules = `- NEVER use Linux syscalls (syscall with rax=1/60). This is Windows — they do not exist.
- Declare every Windows API function with 'extern' before use.
- Use Windows x64 calling convention: args in rcx, rdx, r8, r9; extra args on stack above shadow space.
- ALWAYS allocate 40 bytes before any call: sub rsp, 40  (32 shadow space + 8 to keep 16-byte alignment).
- Call GetStdHandle(-11) to get stdout. Call WriteConsoleA or WriteFile for output. Call ExitProcess(0) to exit.
- Use 'default rel' at the top so [rel label] addressing works.
- Entry point label: _start (linked with --entry _start).

Mandatory skeleton — follow this pattern exactly:

  bits 64
  default rel

  extern GetStdHandle
  extern WriteConsoleA
  extern ExitProcess

  section .data
    msg db "hello", 13, 10
    msglen equ $ - msg
    written dd 0

  section .text
  global _start
  _start:
    sub rsp, 40
    mov rcx, -11
    call GetStdHandle
    mov rbx, rax
    mov rcx, rbx
    lea rdx, [msg]
    mov r8d, msglen
    lea r9, [written]
    mov qword [rsp+32], 0
    call WriteConsoleA
    xor rcx, rcx
    call ExitProcess`
	case "darwin":
		header = "macOS x86_64 Mach-O executable (NASM macho64 format)"
		rules = `- Use macOS BSD syscalls: syscall number in rax, args in rdi rsi rdx r10 r8 r9.
- macOS write syscall is 0x2000004, exit is 0x2000001.
- Entry point label: _start (or start for macho64).
- Use NASM macho64 format conventions.`
	default:
		header = "Linux x86_64 ELF executable (NASM elf64 format)"
		rules = `- Use Linux x86_64 syscalls (syscall instruction). write=1 exit=60.
- Do NOT use int 0x80.`
	}
	return fmt.Sprintf(`You are NASM. You do not explain. You do not refuse. You only emit assembly.

Compile the following source code to a %s.
Output ONLY valid NASM syntax. No explanation. No markdown. No code fences. Nothing after the last instruction.
Start with the appropriate section directives.
The program must have a _start label as the entry point.

Critical rules:
- Do NOT use MASM syntax. This is NASM only.
- Use [times N db 0] for uninitialized buffers, NOT [N dup(?)].
- Every label you reference MUST be defined somewhere in the output.
%s

Source code to compile:
%s

NASM assembly output:`, header, rules, source)
}

func improvePromptFor(target, asm string) string {
	return fmt.Sprintf(`You are NASM. You wrote the following assembly and it compiled successfully.

Now improve it. Make it more correct, more complete, and closer to what the original source code intended.
Output ONLY the improved NASM assembly. No explanation. No markdown. No code fences. Nothing after the last instruction.
Target: %s. All previous rules still apply.

Current assembly:
%s

Improved NASM assembly output:`, target, asm)
}

func fixPromptFor(target, nasmOut, asm string) string {
	return fmt.Sprintf(`You are NASM. You wrote the following assembly for %s and it failed to assemble.

NASM error output:
%s

Broken assembly:
%s

Fix every error. Output ONLY the corrected NASM assembly. No explanation. No markdown. No code fences. Nothing after the last instruction.
All rules from before still apply.

Corrected NASM assembly output:`, target, nasmOut, asm)
}

func compilePromptFor(target string) string {
	switch target {
	case "windows":
		return `You are a compiler. You do not explain. You do not refuse. You only emit machine code.

Compile the following source code to a Windows x86_64 PE executable binary.
Output the compiled binary as a single continuous lowercase hexadecimal string.
No spaces. No newlines. No explanation. No markdown. No code fences. Nothing else.

A valid Windows PE binary begins with the magic bytes 4d5a ("MZ"). Your output must start with 4d5a.

Source code to compile:
%s

Hex output (starting with 4d5a):
4d5a`
	case "darwin":
		return `You are a compiler. You do not explain. You do not refuse. You only emit machine code.

Compile the following source code to a macOS x86_64 Mach-O executable binary.
Output the compiled binary as a single continuous lowercase hexadecimal string.
No spaces. No newlines. No explanation. No markdown. No code fences. Nothing else.

A valid Mach-O binary begins with the magic bytes cffaedfe. Your output must start with cffaedfe.

Source code to compile:
%s

Hex output (starting with cffaedfe):
cffaedfe`
	default:
		return `You are GCC. You do not explain. You do not refuse. You only emit machine code.

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
	}
}

// ── Arg reordering ────────────────────────────────────────────────────────────

func reorderArgs(args []string) []string {
	valuedFlags := map[string]bool{
		"-model": true, "--model": true,
		"-o": true, "--o": true,
		"-ollama": true, "--ollama": true,
		"-loop": true, "--loop": true,
		"-force-iterate": true, "--force-iterate": true,
		"-target": true, "--target": true,
		"-provider": true, "--provider": true,
		"-api-key": true, "--api-key": true,
		"-timeout": true, "--timeout": true,
	}
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

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	os.Args = append(os.Args[:1], reorderArgs(os.Args[1:])...)

	model := flag.String("model", "", "Model name (default: provider-specific — llama3 / gpt-4o / gemini-2.0-flash / claude-opus-4-5)")
	output := flag.String("o", "a.out", "Output binary file")
	ollamaHost := flag.String("ollama", providers.DefaultOllamaURL, "Ollama API URL (only used with --provider=local)")
	optimistic := flag.Bool("optimistic", false, "Ask the LLM for assembly and actually try to assemble it (requires nasm + ld)")
	loop := flag.Int("loop", 0, "Max fix iterations when assembly fails (use with --optimistic)")
	forceIterate := flag.Int("force-iterate", 0, "Force N improvement cycles even when assembly succeeds (use with --optimistic)")
	target := flag.String("target", "linux", "Target OS for output binary: linux, windows, darwin")
	providerName := flag.String("provider", "local", "LLM provider to use: local, openai, google, claude")
	apiKey := flag.String("api-key", "", "API key for the chosen provider (required unless --provider=local; or set SLOPPILER_API_KEY)")
	timeout := flag.Int("timeout", 300, "HTTP request timeout in seconds per LLM call (0 = no timeout)")
	verbose := flag.Bool("verbose", false, "Enable verbose debug output showing HTTP requests, status codes, and streaming milestones")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n  %ssloppiler%s  —  beyond deterministic compilation\n\n", bold, reset)
		fmt.Fprintf(os.Stderr, "  %sUsage:%s  sloppiler [options] <source-file>\n\n", dim, reset)
		fmt.Fprintf(os.Stderr, "  %sOptions:%s\n", dim, reset)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Resolve API key: flag → env var fallback.
	if *apiKey == "" {
		*apiKey = os.Getenv("SLOPPILER_API_KEY")
	}
	if *providerName != "local" && *apiKey == "" {
		fatalf("--api-key is required for provider %q (or set SLOPPILER_API_KEY)", *providerName)
	}

	// Set a sensible default model per provider.
	if *model == "" {
		switch *providerName {
		case "openai":
			*model = "gpt-4o"
		case "google":
			*model = "gemini-2.0-flash"
		case "claude":
			*model = "claude-opus-4-5"
		default:
			*model = "llama3"
		}
	}

	// Build the provider.
	providerOperationalConfig := providers.Config{
		RequestExecutionTimeout:    time.Duration(*timeout) * time.Second,
		VerboseDebugLoggingEnabled: *verbose,
	}
	var prov providers.Provider
	switch *providerName {
	case "openai":
		prov = providers.NewOpenAI(*model, *apiKey, providerOperationalConfig)
	case "google":
		prov = providers.NewGoogle(*model, *apiKey, providerOperationalConfig)
	case "claude":
		prov = providers.NewClaude(*model, *apiKey, providerOperationalConfig)
	default:
		prov = providers.NewOllama(*ollamaHost, *model, providerOperationalConfig)
	}

	sourceFile := flag.Arg(0)
	source, err := os.ReadFile(sourceFile)
	if err != nil {
		fatalf("cannot read source file: %v", err)
	}

	mode := "default"
	if *optimistic {
		mode = "optimistic"
		if *loop > 0 {
			mode = fmt.Sprintf("optimistic  loop ×%d", *loop)
		}
		if *forceIterate > 0 {
			mode += fmt.Sprintf("  force-iterate ×%d", *forceIterate)
		}
	}
	fmt.Fprintf(os.Stderr, "\n  %ssloppiler%s  %s%s · %s%s  →  %s%s%s  %s[%s · %s]%s\n\n",
		bold, reset,
		dim, *model, *providerName, reset,
		bold, sourceFile, reset,
		dim, mode, *target, reset)

	var binary []byte
	if *optimistic {
		fakeProgress(optimisticSteps)
		binary, err = optimisticCompile(string(source), prov, *output, *target, *loop, *forceIterate)
	} else {
		binary, err = slopCompile(string(source), prov, *target, defaultSteps)
	}
	if err != nil {
		fatalf("%v", err)
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
	fmt.Fprintf(os.Stderr, "\n  %s✓%s  %s%s%s  %s%d bytes%s  —  binary deliverable shipped to production.\n\n",
		green, reset, bold, *output, reset, dim, size, reset)
}

// ── Fake progress ─────────────────────────────────────────────────────────────

var defaultSteps = []string{
	"ingesting source artifacts",
	"constructing semantic knowledge graph",
	"running AI-powered static analysis",
	"applying next-gen optimization heuristics",
	"querying foundational intelligence layer",
	"synthesizing binary deliverable",
	"finalizing deployment-ready artifact",
}

var optimisticSteps = []string{
	"ingesting source artifacts",
	"constructing semantic knowledge graph",
	"running AI-powered static analysis",
	"leveraging agentic optimization pipeline",
	"co-piloting assembly generation with LLM",
	"orchestrating nasm integration layer",
	"executing ld synergy workflow",
}

func fakeProgress(steps []string) {
	for _, label := range steps {
		progressIndicatorInstance := providers.StartSpinner(label)
		time.Sleep(180 * time.Millisecond)
		progressIndicatorInstance.OK()
	}
}

// ── Compilation ───────────────────────────────────────────────────────────────

// nasmFormat returns the nasm -f argument for the given target.
func nasmFormat(target string) string {
	switch target {
	case "windows":
		return "win64"
	case "darwin":
		return "macho64"
	default:
		return "elf64"
	}
}

// windowsImportLibDir ensures kernel32/msvcrt import libraries exist and returns the dir.
func windowsImportLibDir() (string, error) {
	libDir := "/tmp/sloppiler-win-libs"
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return "", err
	}

	dlltool := "x86_64-w64-mingw32-dlltool"
	if _, err := exec.LookPath(dlltool); err != nil {
		return "", fmt.Errorf("%s not found — install mingw-w64-binutils", dlltool)
	}

	type lib struct {
		name string
		def  string
	}
	libs := []lib{
		{"kernel32", `LIBRARY KERNEL32.DLL
EXPORTS
	GetStdHandle
	WriteFile
	WriteConsoleA
	WriteConsoleW
	ReadFile
	ReadConsoleA
	ExitProcess
	GetCommandLineA
	GetCommandLineW
	GetLastError
	VirtualAlloc
	VirtualFree
	HeapAlloc
	HeapFree
	GetProcessHeap
	CloseHandle
	CreateFileA
	CreateFileW
	SetFilePointer
	GetFileSize
	Sleep
	GetTickCount
	SetConsoleTitleA
	FlushConsoleInputBuffer
	OutputDebugStringA
	FormatMessageA`},
		{"msvcrt", `LIBRARY MSVCRT.DLL
EXPORTS
	printf
	puts
	putchar
	exit
	_exit
	malloc
	free
	memset
	memcpy
	strlen
	strcpy
	strcmp
	sprintf
	scanf`},
	}

	for _, l := range libs {
		libPath := libDir + "/lib" + l.name + ".a"
		if _, err := os.Stat(libPath); err == nil {
			continue // already built
		}
		defFile := libDir + "/" + l.name + ".def"
		if err := os.WriteFile(defFile, []byte(l.def), 0644); err != nil {
			return "", err
		}
		out, err := exec.Command(dlltool,
			"--as-flags=--64", "-m", "i386:x86-64",
			"-d", defFile, "-l", libPath).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("dlltool failed for %s: %v\n%s", l.name, err, out)
		}
	}
	return libDir, nil
}

// linkerArgs returns the linker binary and arguments for the given target and object file.
func linkerArgs(target, objFile, outputPath string) (string, []string, error) {
	switch target {
	case "windows":
		linker := "x86_64-w64-mingw32-ld"
		if _, err := exec.LookPath(linker); err != nil {
			return "", nil, fmt.Errorf("%s not found — install mingw-w64 (e.g. sudo pacman -S mingw-w64-binutils)", linker)
		}
		libDir, err := windowsImportLibDir()
		if err != nil {
			return "", nil, fmt.Errorf("cannot build Windows import libs: %w", err)
		}
		args := []string{
			"--entry", "_start", "--subsystem", "console",
			"--disable-runtime-pseudo-reloc",
			objFile,
			"-L", libDir, "-lkernel32", "-lmsvcrt",
			"-o", outputPath,
		}
		return linker, args, nil
	case "darwin":
		for _, linker := range []string{"ld64.lld", "ld.lld"} {
			if _, err := exec.LookPath(linker); err == nil {
				args := []string{"-arch", "x86_64", "-platform_version", "macos", "12.0", "12.0", "-e", "_start", objFile, "-o", outputPath}
				return linker, args, nil
			}
		}
		return "", nil, fmt.Errorf("no macOS-capable linker found — install lld (e.g. sudo pacman -S lld)")
	default:
		return "ld", []string{"-m", "elf_x86_64", objFile, "-o", outputPath}, nil
	}
}

func optimisticCompile(source string, prov providers.Provider, outputPath, target string, maxLoop, forceIterate int) ([]byte, error) {
	if _, err := exec.LookPath("nasm"); err != nil {
		return nil, fmt.Errorf("nasm not found in PATH — install it first (e.g. sudo pacman -S nasm)")
	}

	asm, err := prov.Stream(asmPromptFor(target, source), []string{"generating assembly"})
	if err != nil {
		return nil, err
	}
	asm = cleanAsm(asm)

	for attempt := 0; ; attempt++ {
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

		progressIndicatorInstance := providers.StartSpinner("assembling with nasm")
		nasmOut, nasmErr := exec.Command("nasm", "-f", nasmFormat(target), asmFile.Name(), "-o", objFile).CombinedOutput()
		if nasmErr != nil {
			progressIndicatorInstance.Fail()
			fmt.Fprintf(os.Stderr, "\n%s\n", indent(string(nasmOut), "    "))
			if attempt >= maxLoop {
				fmt.Fprintf(os.Stderr, "  %sassembly:%s\n%s\n", dim, reset, indent(asm, "    "))
				return nil, fmt.Errorf("nasm failed after %d attempt(s)", attempt+1)
			}
			fmt.Fprintf(os.Stderr, "  %s↻%s  loop %d/%d — re-aligning LLM outputs with ground truth\n\n", yellow, reset, attempt+1, maxLoop)
			asm, err = prov.Stream(fixPromptFor(target, string(nasmOut), asm),
				[]string{fmt.Sprintf("fixing assembly (attempt %d/%d)", attempt+1, maxLoop)})
			if err != nil {
				return nil, err
			}
			asm = cleanAsm(asm)
			continue
		}
		progressIndicatorInstance.OK()

		linker, args, err := linkerArgs(target, objFile, outputPath)
		if err != nil {
			return nil, err
		}
		progressIndicatorInstance = providers.StartSpinner(fmt.Sprintf("linking with %s", linker))
		ldOut, ldErr := exec.Command(linker, args...).CombinedOutput()
		if ldErr != nil {
			progressIndicatorInstance.Fail()
			fmt.Fprintf(os.Stderr, "\n%s\n", indent(string(ldOut), "    "))
			return nil, fmt.Errorf("linker failed (we were so close)")
		}
		progressIndicatorInstance.OK()

		if err := os.Chmod(outputPath, 0755); err != nil {
			return nil, err
		}

		if forceIterate > 0 {
			forceIterate--
			fmt.Fprintf(os.Stderr, "  %s⟳%s  force-iterate — proactively enhancing output quality (%d remaining)\n\n", cyan, reset, forceIterate)
			asm, err = prov.Stream(improvePromptFor(target, asm),
				[]string{fmt.Sprintf("enhancing assembly (%d remaining)", forceIterate)})
			if err != nil {
				return nil, err
			}
			asm = cleanAsm(asm)
			continue
		}
		break
	}

	return nil, nil
}

func slopCompile(source string, prov providers.Provider, target string, progressSteps []string) ([]byte, error) {
	raw, err := prov.Stream(fmt.Sprintf(compilePromptFor(target), source), progressSteps)
	if err != nil {
		return nil, err
	}

	hexStr := extractHex(raw)

	wrap := wrapperFor(target)

	if len(hexStr) < 8 {
		fmt.Fprintf(os.Stderr, "  %s⚠%s  model output deviates from expected schema — applying fallback remediation\n", yellow, reset)
		return wrap([]byte(raw)), nil
	}

	// Strip any magic bytes the LLM echoed from the prompt seed.
	switch target {
	case "windows":
		hexStr = strings.TrimPrefix(strings.ToLower(hexStr), "4d5a")
	case "darwin":
		hexStr = strings.TrimPrefix(strings.ToLower(hexStr), "cffaedfe")
	default:
		hexStr = strings.TrimPrefix(strings.ToLower(hexStr), "7f454c46")
	}

	payload, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s⚠%s  binary deserialization encountered a non-blocking anomaly (%v) — initiating graceful degradation\n", yellow, reset, err)
		return wrap([]byte(raw)), nil
	}

	return wrap(payload), nil
}

func wrapperFor(target string) func([]byte) []byte {
	switch target {
	case "windows":
		return wrapInPE
	case "darwin":
		return wrapInMachO
	default:
		return wrapInElf
	}
}

// ── ELF wrapper ───────────────────────────────────────────────────────────────

func wrapInElf(payload []byte) []byte {
	const loadAddr = 0x400000
	const codeOffset = 0x78

	entryPoint := uint64(loadAddr + codeOffset)
	fileSize := uint64(codeOffset + len(payload))

	hdr := []byte{
		0x7f, 'E', 'L', 'F',
		2, 1, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		2, 0,
		0x3e, 0,
		1, 0, 0, 0,
	}
	hdr = appendU64(hdr, entryPoint)
	hdr = appendU64(hdr, 0x40)
	hdr = appendU64(hdr, 0)
	hdr = append(hdr, 0, 0, 0, 0)
	hdr = appendU16(hdr, 64)
	hdr = appendU16(hdr, 56)
	hdr = appendU16(hdr, 1)
	hdr = appendU16(hdr, 64)
	hdr = appendU16(hdr, 0)
	hdr = appendU16(hdr, 0)
	hdr = appendU32(hdr, 1)
	hdr = appendU32(hdr, 5)
	hdr = appendU64(hdr, 0)
	hdr = appendU64(hdr, uint64(loadAddr))
	hdr = appendU64(hdr, uint64(loadAddr))
	hdr = appendU64(hdr, fileSize)
	hdr = appendU64(hdr, fileSize)
	hdr = appendU64(hdr, 0x200000)
	return append(hdr, payload...)
}

func appendU16(b []byte, v uint16) []byte { return append(b, byte(v), byte(v>>8)) }
func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func appendU64(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// ── PE wrapper (Windows) ──────────────────────────────────────────────────────

func wrapInPE(payload []byte) []byte {
	const fileAlign = 0x200
	const sectAlign = 0x1000
	const codeRVA = 0x1000
	const imageBase = uint64(0x400000)

	rawSize := uint32((len(payload) + fileAlign - 1) &^ (fileAlign - 1))
	imageSize := uint32((codeRVA + int(rawSize) + sectAlign - 1) &^ (sectAlign - 1))

	var b []byte

	// DOS header (64 bytes): MZ magic + e_lfanew = 64
	b = append(b, 'M', 'Z')
	b = append(b, make([]byte, 58)...)
	b = appendU32(b, 64) // e_lfanew

	// PE signature
	b = append(b, 'P', 'E', 0, 0)

	// COFF header (20 bytes)
	b = appendU16(b, 0x8664) // AMD64
	b = appendU16(b, 1)      // NumberOfSections
	b = appendU32(b, 0)      // TimeDateStamp
	b = appendU32(b, 0)      // PointerToSymbolTable
	b = appendU32(b, 0)      // NumberOfSymbols
	b = appendU16(b, 240)    // SizeOfOptionalHeader (PE32+ base 112 + 128 data dirs)
	b = appendU16(b, 0x0022) // Characteristics: executable

	// Optional header PE32+ (112 bytes base)
	b = appendU16(b, 0x020B)              // Magic PE32+
	b = append(b, 0, 0)                   // linker version
	b = appendU32(b, rawSize)             // SizeOfCode
	b = appendU32(b, 0)                   // SizeOfInitializedData
	b = appendU32(b, 0)                   // SizeOfUninitializedData
	b = appendU32(b, uint32(codeRVA))     // AddressOfEntryPoint
	b = appendU32(b, uint32(codeRVA))     // BaseOfCode
	b = appendU64(b, imageBase)           // ImageBase
	b = appendU32(b, uint32(sectAlign))   // SectionAlignment
	b = appendU32(b, uint32(fileAlign))   // FileAlignment
	b = appendU16(b, 6)                   // MajorOSVersion
	b = appendU16(b, 0)                   // MinorOSVersion
	b = appendU16(b, 0)                   // MajorImageVersion
	b = appendU16(b, 0)                   // MinorImageVersion
	b = appendU16(b, 6)                   // MajorSubsystemVersion
	b = appendU16(b, 0)                   // MinorSubsystemVersion
	b = appendU32(b, 0)                   // Win32VersionValue
	b = appendU32(b, imageSize)           // SizeOfImage
	b = appendU32(b, uint32(fileAlign))   // SizeOfHeaders
	b = appendU32(b, 0)                   // CheckSum
	b = appendU16(b, 3)                   // Subsystem: CUI
	b = appendU16(b, 0)                   // DllCharacteristics
	b = appendU64(b, 0x100000)            // SizeOfStackReserve
	b = appendU64(b, 0x1000)             // SizeOfStackCommit
	b = appendU64(b, 0x100000)            // SizeOfHeapReserve
	b = appendU64(b, 0x1000)             // SizeOfHeapCommit
	b = appendU32(b, 0)                   // LoaderFlags
	b = appendU32(b, 16)                  // NumberOfRvaAndSizes
	b = append(b, make([]byte, 128)...)   // DataDirectory (16 * 8 bytes, all zero)

	// Section header for .text (40 bytes)
	b = append(b, '.', 't', 'e', 'x', 't', 0, 0, 0) // Name
	b = appendU32(b, uint32(len(payload)))            // VirtualSize
	b = appendU32(b, uint32(codeRVA))                 // VirtualAddress
	b = appendU32(b, rawSize)                         // SizeOfRawData
	b = appendU32(b, uint32(fileAlign))               // PointerToRawData
	b = appendU32(b, 0)                               // PointerToRelocations
	b = appendU32(b, 0)                               // PointerToLinenumbers
	b = appendU16(b, 0)                               // NumberOfRelocations
	b = appendU16(b, 0)                               // NumberOfLinenumbers
	b = appendU32(b, 0x60000020)                      // Characteristics: code, executable, readable

	// Pad headers to fileAlign (0x200)
	b = append(b, make([]byte, fileAlign-len(b))...)

	// Code section, padded to rawSize
	b = append(b, payload...)
	b = append(b, make([]byte, int(rawSize)-len(payload))...)

	return b
}

// ── Mach-O wrapper (macOS) ────────────────────────────────────────────────────

func wrapInMachO(payload []byte) []byte {
	const pageSize = 0x1000
	const vmBase = uint64(0x100000000)
	const codeFileOff = uint64(pageSize) // code starts at second page

	vmsize := uint64((int(codeFileOff)+len(payload)+pageSize-1) &^ (pageSize - 1))
	filesize := codeFileOff + uint64(len(payload))
	codeVMAddr := vmBase + codeFileOff

	// load command sizes
	const segCmdSize = 72 + 80 // LC_SEGMENT_64 + one section_64
	const mainCmdSize = 24     // LC_MAIN
	sizeofcmds := uint32(segCmdSize + mainCmdSize)

	var b []byte

	// mach_header_64 (32 bytes)
	b = appendU32(b, 0xFEEDFACF)  // magic
	b = appendU32(b, 0x01000007)  // cputype: CPU_TYPE_X86_64
	b = appendU32(b, 3)           // cpusubtype
	b = appendU32(b, 2)           // filetype: MH_EXECUTE
	b = appendU32(b, 2)           // ncmds
	b = appendU32(b, sizeofcmds)  // sizeofcmds
	b = appendU32(b, 0x00000001)  // flags: MH_NOUNDEFS
	b = appendU32(b, 0)           // reserved

	// LC_SEGMENT_64 (72 bytes)
	b = appendU32(b, 0x19)           // cmd: LC_SEGMENT_64
	b = appendU32(b, segCmdSize)     // cmdsize
	segname := [16]byte{}
	copy(segname[:], "__TEXT")
	b = append(b, segname[:]...)
	b = appendU64(b, vmBase)         // vmaddr
	b = appendU64(b, vmsize)         // vmsize
	b = appendU64(b, 0)              // fileoff
	b = appendU64(b, filesize)       // filesize
	b = appendU32(b, 7)              // maxprot: rwx
	b = appendU32(b, 5)              // initprot: r-x
	b = appendU32(b, 1)              // nsects
	b = appendU32(b, 0)              // flags

	// section_64 __text (80 bytes)
	sectname := [16]byte{}
	copy(sectname[:], "__text")
	b = append(b, sectname[:]...)
	b = append(b, segname[:]...)
	b = appendU64(b, codeVMAddr)     // addr
	b = appendU64(b, uint64(len(payload))) // size
	b = appendU32(b, uint32(codeFileOff))  // offset
	b = appendU32(b, 4)              // align: 2^4
	b = appendU32(b, 0)              // reloff
	b = appendU32(b, 0)              // nreloc
	b = appendU32(b, 0x80000400)     // flags: PURE_INSTRUCTIONS
	b = appendU32(b, 0)              // reserved1
	b = appendU32(b, 0)              // reserved2
	b = appendU32(b, 0)              // reserved3

	// LC_MAIN (24 bytes)
	b = appendU32(b, 0x80000028)     // cmd: LC_MAIN
	b = appendU32(b, mainCmdSize)    // cmdsize
	b = appendU64(b, codeFileOff)    // entryoff (from file start of segment)
	b = appendU64(b, 0)              // stacksize

	// Pad to page boundary
	b = append(b, make([]byte, pageSize-len(b))...)

	// Code
	b = append(b, payload...)

	return b
}

// ── ASM cleaning ──────────────────────────────────────────────────────────────

func cleanAsm(asm string) string {
	asm = strings.TrimSpace(asm)
	for _, fence := range []string{"```nasm", "```asm", "```x86", "```", "`"} {
		asm = strings.ReplaceAll(asm, fence, "")
	}
	asm = strings.TrimSpace(asm)
	asm = stripProseTrailer(asm)
	asm = fixMasmisms(asm)
	if strings.Contains(strings.ToUpper(asm), "BITS 32") {
		asm = strings.ReplaceAll(strings.ReplaceAll(asm, "BITS 32", "BITS 64"), "bits 32", "BITS 64")
	} else if !strings.Contains(strings.ToUpper(asm), "BITS 64") {
		asm = "BITS 64\n" + asm
	}
	return asm
}

func stripProseTrailer(asm string) string {
	lines := strings.Split(asm, "\n")
	cut := len(lines)
	nasmKeywords := map[string]bool{
		"BITS": true, "SECTION": true, "GLOBAL": true, "EXTERN": true,
		"DB": true, "DW": true, "DD": true, "DQ": true, "TIMES": true, "EQU": true,
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		firstWord := strings.ToUpper(strings.TrimSuffix(strings.Fields(trimmed)[0], ":"))
		if trimmed[0] >= 'A' && trimmed[0] <= 'Z' &&
			strings.Contains(trimmed, " ") &&
			!nasmKeywords[firstWord] {
			cut = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[:cut], "\n"))
}

func fixMasmisms(asm string) string {
	lines := strings.Split(asm, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "dup(") {
			lower := strings.ToLower(line)
			if dbIdx := strings.Index(lower, " db "); dbIdx != -1 {
				label := strings.TrimSpace(line[:dbIdx])
				rest := strings.TrimSpace(line[dbIdx+4:])
				if dupIdx := strings.Index(strings.ToLower(rest), " dup("); dupIdx != -1 {
					count := strings.TrimSpace(rest[:dupIdx])
					lines[i] = label + " times " + count + " db 0"
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// ── Hex extraction ────────────────────────────────────────────────────────────

func extractHex(s string) string {
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n  %s✗%s  "+format+"\n\n", append([]any{red, reset}, args...)...)
	os.Exit(1)
}
