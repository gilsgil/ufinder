// ufinder.go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner" // NECESSÁRIO: go get github.com/briandowns/spinner
	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
)

// CONFIGURAÇÃO
const MaxConcurrentTools = 1

// --- HELPERS VISUAIS ---

// Ícones e cores modernas
var (
	iconCheck  = color.New(color.FgGreen, color.Bold).Sprint("✔")
	iconFire   = color.New(color.FgHiYellow).Sprint("⚡")
	iconBox    = color.New(color.FgCyan).Sprint("📦")
	iconSearch = color.New(color.FgHiBlue).Sprint("🔎")

	colorTool = color.New(color.FgHiWhite, color.Bold).SprintFunc()
	colorTime = color.New(color.FgHiBlack).SprintFunc() // Cinza escuro para o tempo
	colorNew  = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	colorZero = color.New(color.FgHiBlack).SprintFunc() // Discreto se for zero
)

func printBanner() {
	// Limpa a tela antes de começar (opcional, remove se não gostar)
	fmt.Print("\033[H\033[2J")

	myFigure := figure.NewFigure("UFINDER", "slant", true)
	color.Cyan(myFigure.String())
	fmt.Println(color.New(color.FgHiBlack).Sprint("\n   by Gilson Oliveira"))
	fmt.Println("")
}

func printHeader(domain, folder string) {
	fmt.Printf("   %s Target: %s\n", iconFire, color.HiWhiteString(domain))
	fmt.Printf("   %s Output: %s\n", iconBox, color.HiWhiteString(folder))
	fmt.Println(strings.Repeat(color.HiBlackString("─"), 60))
	fmt.Println("")
}

// --- HELPERS LÓGICOS ---

func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func countLines(filePath string) int {
	if !fileExists(filePath) {
		return 0
	}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("wc -l < %s", filePath)).Output()
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

func runShellCommand(command string, verbose bool) error {
	if verbose {
		color.New(color.FgHiBlack).Printf("[CMD] %s\n", command)
	}
	cmd := exec.Command("sh", "-c", command)
	if verbose {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// runTool com visual moderno e spinner
func runTool(command, toolName, outputFile string, verbose bool) {
	start := time.Now()
	prevCount := countLines(outputFile)

	// Inicia Spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Estilo "dots"
	s.Suffix = fmt.Sprintf("  Running %s...", colorTool(strings.ToUpper(toolName)))
	s.Color("cyan")
	s.Start()

	// --- Lógica de Execução (Mantida Idêntica) ---
	if toolName == "waymore" {
		tempWaymore := outputFile + ".tmp"
		cmdWithTemp := strings.Replace(command, outputFile, tempWaymore, 1)
		os.Remove(tempWaymore)

		runShellCommand(cmdWithTemp, verbose)

		if fileExists(tempWaymore) {
			runShellCommand(fmt.Sprintf("cat %s >> %s", tempWaymore, outputFile), verbose)
			os.Remove(tempWaymore)
		}
	} else {
		fullCommand := fmt.Sprintf("%s >> %s", command, outputFile)
		runShellCommand(fullCommand, verbose)
	}

	// Ordenação individual
	sortCmd := fmt.Sprintf("sort -u %s -o %s", outputFile, outputFile)
	runShellCommand(sortCmd, verbose)
	// ---------------------------------------------

	s.Stop() // Para o spinner

	// Estatísticas
	elapsed := time.Since(start).Round(time.Second)
	currentCount := countLines(outputFile)
	newInThisTool := currentCount - prevCount

	// Formatação Visual (Alinhamento em colunas)
	// %-12s = Alinha texto à esquerda com 12 espaços
	// %6s   = Alinha à direita

	toolLabel := fmt.Sprintf("%-12s", strings.ToUpper(toolName))
	timeLabel := fmt.Sprintf("%6s", elapsed)
	totalLabel := fmt.Sprintf("%8d urls", currentCount)

	var newLabel string
	if newInThisTool > 0 {
		newLabel = colorNew(fmt.Sprintf("+%d new", newInThisTool))
	} else {
		newLabel = colorZero("0 new")
	}

	// Output final da linha
	fmt.Printf(" %s %s  %s  %s  %s\n",
		iconCheck,
		colorTool(toolLabel),
		colorTime(timeLabel),
		totalLabel,
		newLabel,
	)
}

func aggregateAndClean(toolFiles map[string]string, urlsFile string, oldGlobalCount int) {
	// Spinner para a agregação
	fmt.Println("")
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = "  Aggregating and deduplicating results..."
	s.Color("yellow")
	s.Start()

	rawCombined := urlsFile + ".tmp"
	os.Remove(rawCombined)

	// Salvar URLs antigas antes de agregar (para calcular diff depois)
	oldURLs := make(map[string]bool)
	if fileExists(urlsFile) {
		oldContent, err := os.ReadFile(urlsFile)
		if err == nil {
			for _, line := range strings.Split(string(oldContent), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					oldURLs[line] = true
				}
			}
		}
	}

	var filesToMerge []string
	if fileExists(urlsFile) {
		filesToMerge = append(filesToMerge, urlsFile)
	}
	for _, f := range toolFiles {
		if fileExists(f) {
			filesToMerge = append(filesToMerge, f)
		}
	}

	if len(filesToMerge) > 0 {
		cmdCat := fmt.Sprintf("cat %s >> %s", strings.Join(filesToMerge, " "), rawCombined)
		runShellCommand(cmdCat, false) // Agregação interna não precisa de verbose
		cmdSort := fmt.Sprintf("sort -u %s -o %s", rawCombined, urlsFile)
		runShellCommand(cmdSort, false)
		os.Remove(rawCombined)
	}

	s.Stop()

	// Calcular URLs novas (diff entre arquivo final e antigas)
	var newURLs []string
	if fileExists(urlsFile) {
		newContent, err := os.ReadFile(urlsFile)
		if err == nil {
			for _, line := range strings.Split(string(newContent), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !oldURLs[line] {
					newURLs = append(newURLs, line)
				}
			}
		}
	}

	// Ordenar as novas URLs (ascending)
	sort.Strings(newURLs)

	// Salvar last_results.txt
	lastResultsFile := filepath.Join(filepath.Dir(urlsFile), "last_results.txt")
	if len(newURLs) > 0 {
		os.WriteFile(lastResultsFile, []byte(strings.Join(newURLs, "\n")+"\n"), 0644)
	} else {
		// Arquivo vazio se não houver novas URLs
		os.WriteFile(lastResultsFile, []byte{}, 0644)
	}

	// Stats Finais
	newGlobalCount := countLines(urlsFile)
	realNewURLs := len(newURLs)

	// Caixa de Resumo Moderno
	fmt.Println("")
	fmt.Println(color.HiBlackString("┌──────────────────────────────────────────────┐"))
	fmt.Printf("│  %s                 │\n", color.HiWhiteString("FINAL RESULTS SUMMARY"))
	fmt.Println(color.HiBlackString("├──────────────────────────────────────────────┤"))
	fmt.Printf("│  Previous Total     : %-22d │\n", oldGlobalCount)
	fmt.Printf("│  Current Total      : %-22d │\n", newGlobalCount)
	fmt.Println(color.HiBlackString("│                                              │"))

	if realNewURLs > 0 {
		fmt.Printf("│  %s : %-22s │\n", color.HiGreenString("UNIQUE NEW URLS"), colorNew(fmt.Sprintf("+%d", realNewURLs)))
	} else {
		fmt.Printf("│  %s           : %-22s │\n", "Unique New URLs", color.HiBlackString("0"))
	}
	fmt.Println(color.HiBlackString("└──────────────────────────────────────────────┘"))

	// Mostrar as novas URLs no terminal (ordenadas ascending)
	if len(newURLs) > 0 {
		fmt.Println("")
		fmt.Println(color.HiCyanString("┌──────────────────────────────────────────────┐"))
		fmt.Printf("│  %s                       │\n", color.HiWhiteString("NEW URLS FOUND"))
		fmt.Println(color.HiCyanString("└──────────────────────────────────────────────┘"))
		for _, url := range newURLs {
			fmt.Printf("  %s %s\n", color.HiGreenString("→"), url)
		}
	}
	fmt.Println("")
}

func discovery(domain, folderName string, toolsArg string, verbose bool) {
	baseDir := folderName
	endpointsDir := filepath.Join(baseDir, "endpoints")
	os.MkdirAll(endpointsDir, 0755)
	urlsFile := filepath.Join(endpointsDir, "urls.txt")
	oldGlobalCount := countLines(urlsFile)

	printHeader(domain, folderName)

	toolFiles := map[string]string{
		"waymore":     filepath.Join(endpointsDir, "waymore.txt"),
		"waybackurls": filepath.Join(endpointsDir, "waybackurls.txt"),
		"gau":         filepath.Join(endpointsDir, "gau.txt"),
		"xurlfind3r":  filepath.Join(endpointsDir, "xurlfind3r.txt"),
		"urlscan":     filepath.Join(endpointsDir, "urlscan.txt"),
		"urlfinder":   filepath.Join(endpointsDir, "urlfinder.txt"),
		"ducker":      filepath.Join(endpointsDir, "ducker.txt"),
	}

	toolCommands := map[string]string{
		"waybackurls": fmt.Sprintf("waybackurls %s", domain),
		"gau":         fmt.Sprintf("gau %s --subs", domain),
		"xurlfind3r":  fmt.Sprintf("xurlfind3r -d %s --include-subdomains -s", domain),
		"urlscan": fmt.Sprintf(`curl -s "https://urlscan.io/api/v1/search/?q=page.domain:%s&size=10000" -H "API-Key: %s" | jq -r '.results[].page.url'`,
			domain, os.Getenv("URLSCAN")),
		"urlfinder": fmt.Sprintf("urlfinder -d %s -all", domain),
		"ducker":    fmt.Sprintf("ducker -q 'site:%s'", domain),
		"waymore":   fmt.Sprintf("waymore -i %s -mode U -oU %s", domain, toolFiles["waymore"]),
	}

	var selectedTools []string
	if toolsArg != "" {
		selectedTools = strings.Split(toolsArg, ",")
	} else {
		for tool := range toolFiles {
			selectedTools = append(selectedTools, tool)
		}
	}

	sem := make(chan struct{}, MaxConcurrentTools)
	var wg sync.WaitGroup

	for _, tool := range selectedTools {
		tool = strings.TrimSpace(tool)
		cmdStr, exists := toolCommands[tool]
		if !exists && tool != "waymore" {
			continue
		}

		wg.Add(1)
		go func(t, c string) {
			defer wg.Done()
			sem <- struct{}{}
			runTool(c, t, toolFiles[t], verbose)
			<-sem
		}(tool, cmdStr)
	}
	wg.Wait()

	aggregateAndClean(toolFiles, urlsFile, oldGlobalCount)
}

func init() {
	// Garante que ~/go/bin esteja no PATH para ferramentas instaladas via go install
	home, err := os.UserHomeDir()
	if err == nil {
		goBin := filepath.Join(home, "go", "bin")
		path := os.Getenv("PATH")
		if !strings.Contains(path, goBin) {
			os.Setenv("PATH", goBin+string(os.PathListSeparator)+path)
		}
	}
}

func main() {
	domain := flag.String("d", "", "Target domain")
	folderName := flag.String("f", "", "Output folder")
	toolsArg := flag.String("t", "", "Tools list")
	verbose := flag.Bool("v", false, "Verbose mode")
	flag.Parse()

	if *folderName == "" || *domain == "" {
		// Mensagem de erro mais bonita
		fmt.Println("")
		color.Red("  ✖ Error: Missing arguments.")
		fmt.Println("  Usage: ufinder -d domain.com -f output_folder")
		fmt.Println("")
		os.Exit(1)
	}

	printBanner()
	discovery(*domain, *folderName, *toolsArg, *verbose)
}
