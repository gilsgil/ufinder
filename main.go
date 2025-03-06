// ufinder.go
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
)

// Helper: verifica se um arquivo existe
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// Helper: lê todas as linhas de um arquivo e retorna um slice
func readLines(filePath string) ([]string, error) {
	var lines []string
	file, err := os.Open(filePath)
	if err != nil {
		return lines, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// Helper: escreve as linhas em um arquivo (substituindo o conteúdo)
func writeLines(lines []string, filePath string) error {
	content := strings.Join(lines, "\n")
	return ioutil.WriteFile(filePath, []byte(content+"\n"), 0644)
}

// Helper: remove duplicatas e ordena as linhas
func uniqueSort(lines []string) []string {
	set := make(map[string]struct{})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			set[line] = struct{}{}
		}
	}
	var unique []string
	for line := range set {
		unique = append(unique, line)
	}
	sort.Strings(unique)
	return unique
}

// Imprime o banner em ASCII
func printBanner() {
	myFigure := figure.NewFigure("UFINDER", "slant", true)
	color.Cyan(myFigure.String())
	color.Yellow("\nby Gilson Oliveira\n")
}

// Imprime estatísticas da ferramenta com cores
func printStat(tool string, count, newCount int) {
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s URLs found: %d (New: %d)\n", green("["+strings.ToUpper(tool)+"]"), count, newCount)
}

// Executa o comando da ferramenta, atualiza os arquivos e calcula os novos itens
func runTool(command, toolName, outputFile string) (int, int) {
	rawFile := outputFile + ".raw"
	lastFile := filepath.Join(filepath.Dir(outputFile), "last_"+toolName+".txt")

	// Lê o conjunto antigo (raw)
	oldSet := make(map[string]struct{})
	if fileExists(rawFile) {
		if lines, err := readLines(rawFile); err == nil {
			for _, line := range lines {
				oldSet[line] = struct{}{}
			}
		}
	}

	// Garante que o diretório existe
	os.MkdirAll(filepath.Dir(outputFile), 0755)

	// Executa o comando usando o shell
	cmd := exec.Command("sh", "-c", command)
	var outb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		color.Red("%s error: %v", toolName, err)
		return 0, 0
	}
	outputStr := outb.String()
	newLinesRaw := strings.Split(outputStr, "\n")
	var newLines []string
	for _, l := range newLinesRaw {
		trim := strings.TrimSpace(l)
		if trim != "" {
			newLines = append(newLines, trim)
		}
	}

	// Junta os resultados antigos e novos e calcula o delta
	newSet := make(map[string]struct{})
	delta := make(map[string]struct{})
	for k := range oldSet {
		newSet[k] = struct{}{}
	}
	for _, line := range newLines {
		if _, exists := newSet[line]; !exists {
			newSet[line] = struct{}{}
			delta[line] = struct{}{}
		}
	}

	var allLines []string
	for line := range newSet {
		allLines = append(allLines, line)
	}
	allLines = uniqueSort(allLines)
	additional := len(delta)

	// Grava os arquivos raw e de saída
	if err := writeLines(allLines, rawFile); err != nil {
		color.Red("Error writing raw file (%s): %v", rawFile, err)
	}
	if err := writeLines(allLines, outputFile); err != nil {
		color.Red("Error writing output file (%s): %v", outputFile, err)
	}

	// Grava os novos itens em last_{tool}.txt
	var deltaLines []string
	for line := range delta {
		deltaLines = append(deltaLines, line)
	}
	deltaLines = uniqueSort(deltaLines)
	if err := writeLines(deltaLines, lastFile); err != nil {
		color.Red("Error writing last file (%s): %v", lastFile, err)
	}

	// Garante que o arquivo original não tenha duplicatas (usando sort -u)
	exec.Command("sh", "-c", fmt.Sprintf("sort -u -o %s %s", outputFile, outputFile)).Run()

	return len(allLines), additional
}

// Filtra cada arquivo da ferramenta para manter apenas as URLs exclusivas (que só aparecem naquele arquivo)
// OBS.: Essa função é mantida para exibição de estatísticas por ferramenta,
// mas ela não afetará o arquivo mestre final.
func filterUniquePerTool(toolFiles map[string]string) {
	freq := make(map[string]int)
	for _, file := range toolFiles {
		if fileExists(file) {
			if lines, err := readLines(file); err == nil {
				for _, line := range lines {
					freq[line]++
				}
			}
		}
	}

	for tool, file := range toolFiles {
		if fileExists(file) {
			lines, err := readLines(file)
			if err != nil {
				color.Red("Error reading file %s: %v", file, err)
				continue
			}
			var uniqueLines []string
			for _, line := range lines {
				// Só mantém a linha se ela aparecer somente nessa ferramenta
				if freq[line] == 1 {
					uniqueLines = append(uniqueLines, line)
				}
			}
			uniqueLines = uniqueSort(uniqueLines)
			tmpFile := file + ".tmp"
			if err := writeLines(uniqueLines, tmpFile); err != nil {
				color.Red("Error writing temporary file %s: %v", tmpFile, err)
				continue
			}
			if err := os.Rename(tmpFile, file); err != nil {
				color.Red("Error renaming temporary file %s: %v", tmpFile, err)
				continue
			}
			out, err := exec.Command("sh", "-c", fmt.Sprintf("wc -l < %s", file)).Output()
			filteredCount := "unknown"
			if err == nil {
				filteredCount = strings.TrimSpace(string(out))
			}
			color.Cyan("[%s] original file filtered: kept %v truly unique entries.", strings.ToUpper(tool), filteredCount)
		}
	}
}

// Agrega os resultados de vários arquivos em um único arquivo mestre (urls.txt)
// Aqui, concatenamos TODOS os resultados (de cada ferramenta) e só depois aplicamos a remoção de duplicatas.
func aggregateResults(toolFiles map[string]string, urlsFile string) {
	var allLines []string

	// Se o arquivo mestre já existir, lê os resultados já acumulados
	if fileExists(urlsFile) {
		if existing, err := readLines(urlsFile); err == nil {
			allLines = append(allLines, existing...)
		}
	}

	// Concatena os resultados de cada ferramenta
	for _, file := range toolFiles {
		if fileExists(file) {
			if lines, err := readLines(file); err == nil {
				allLines = append(allLines, lines...)
			}
		}
	}

	// Aplica o uniq para manter somente URLs únicas e ordenadas
	allLines = uniqueSort(allLines)
	if err := writeLines(allLines, urlsFile); err != nil {
		color.Red("Error writing master file (%s): %v", urlsFile, err)
	}
}

// Função principal de descoberta de URLs com execução concorrente (goroutines)
func discovery(domain, folderName string, compare bool, toolsArg string) {
	baseDir := folderName
	endpointsDir := filepath.Join(baseDir, "endpoints")
	os.MkdirAll(endpointsDir, 0755)

	urlsFile := filepath.Join(endpointsDir, "urls.txt")
	oldGlobal := make(map[string]struct{})
	if fileExists(urlsFile) {
		if lines, err := readLines(urlsFile); err == nil {
			for _, line := range lines {
				oldGlobal[line] = struct{}{}
			}
		}
	}
	oldGlobalCount := len(oldGlobal)

	// Define os comandos para cada ferramenta
	toolCommands := map[string]string{
		"waymore":     fmt.Sprintf("waymore -i %s -mode U -oU %s", domain, filepath.Join(endpointsDir, "waymore.txt")),
		"waybackurls": fmt.Sprintf("waybackurls %s", domain),
		"gau":         fmt.Sprintf("gau %s --subs", domain),
		"xurlfind3r":  fmt.Sprintf("xurlfind3r -d %s --include-subdomains -s", domain),
		"urlscan": fmt.Sprintf(`curl -s "https://urlscan.io/api/v1/search/?q=domain:%s&size=10000" -H "API-Key: %s" | jq -r '.results[].page.url'`,
			domain, os.Getenv("URLSCAN")),
		"urlfinder": fmt.Sprintf("urlfinder -d %s -all", domain),
		"ducker":    fmt.Sprintf("ducker -q 'site:%s' -c 1000", domain),
	}
	toolFiles := map[string]string{
		"waymore":     filepath.Join(endpointsDir, "waymore.txt"),
		"waybackurls": filepath.Join(endpointsDir, "waybackurls.txt"),
		"gau":         filepath.Join(endpointsDir, "gau.txt"),
		"xurlfind3r":  filepath.Join(endpointsDir, "xurlfind3r.txt"),
		"urlscan":     filepath.Join(endpointsDir, "urlscan.txt"),
		"urlfinder":   filepath.Join(endpointsDir, "urlfinder.txt"),
		"ducker":      filepath.Join(endpointsDir, "ducker.txt"),
	}

	// Se o domínio não for informado, apenas compara os arquivos existentes
	if domain == "" {
		if compare {
			compareUniqueURLs(toolFiles)
		} else {
			color.Red("Error: You must provide a domain (-d) or use -c to compare existing results.")
		}
		return
	}

	// Seleciona as ferramentas a serem executadas (todas se não especificado)
	var selectedTools []string
	if toolsArg != "" {
		for _, t := range strings.Split(toolsArg, ",") {
			selectedTools = append(selectedTools, strings.TrimSpace(t))
		}
	} else {
		for tool := range toolCommands {
			selectedTools = append(selectedTools, tool)
		}
	}

	// Executa cada ferramenta em uma goroutine
	var wg sync.WaitGroup
	for _, tool := range selectedTools {
		if cmdStr, exists := toolCommands[tool]; exists {
			wg.Add(1)
			go func(tool, cmdStr string) {
				defer wg.Done()
				// Ajuste: uso do Printf com cor ciano para garantir que a mensagem seja colorida
				//color.New(color.FgCyan).Printf("\nRunning %s...\n", strings.Title(tool))
				total, newFound := runTool(cmdStr, tool, toolFiles[tool])
				printStat(tool, total, newFound)
			}(tool, cmdStr)
		} else {
			color.Red("Invalid tool: %s", tool)
		}
	}
	wg.Wait()

	// IMPORTANTE: AGORA, agregamos os resultados **antes** de qualquer filtragem adicional
	color.Cyan("\nAggregating results into urls.txt...")
	aggregateResults(toolFiles, urlsFile)
	newGlobal := make(map[string]struct{})
	if fileExists(urlsFile) {
		if lines, err := readLines(urlsFile); err == nil {
			for _, line := range lines {
				newGlobal[line] = struct{}{}
			}
		}
	}
	totalGlobal := len(newGlobal)
	additionalGlobal := totalGlobal - oldGlobalCount
	color.Green("\n[TOTAL] Unique URLs: %d (Previously: %d, New: %d)", totalGlobal, oldGlobalCount, additionalGlobal)

	// Agora, se desejar, você pode filtrar os outputs individuais (para exibição por ferramenta)
	color.Cyan("\nFiltering individual tool outputs for truly exclusive entries...")
	filterUniquePerTool(toolFiles)

	// Se houver opção de comparação, use-a
	if compare {
		compareUniqueURLs(toolFiles)
	}
}

// Compara os URLs únicos de cada ferramenta e exibe estatísticas
func compareUniqueURLs(toolFiles map[string]string) {
	allURLs := make(map[string]struct{})
	toolUniqueCounts := make(map[string]int)

	for tool, file := range toolFiles {
		if fileExists(file) {
			lines, err := readLines(file)
			if err != nil {
				continue
			}
			uniqueCount := 0
			for _, line := range lines {
				if _, exists := allURLs[line]; !exists {
					uniqueCount++
					allURLs[line] = struct{}{}
				}
			}
			toolUniqueCounts[tool] = uniqueCount
		}
	}

	color.Cyan("\nComparison of Unique URLs Per Tool:\n")
	type kv struct {
		Tool  string
		Count int
	}
	var counts []kv
	for k, v := range toolUniqueCounts {
		counts = append(counts, kv{k, v})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].Count > counts[j].Count
	})
	for _, item := range counts {
		color.Yellow("%s: %d unique URLs", strings.ToUpper(item.Tool), item.Count)
	}
	color.Magenta("\nTOTAL UNIQUE URLs ACROSS ALL TOOLS: %d", len(allURLs))
}

func main() {
	domain := flag.String("d", "", "Target domain")
	folderName := flag.String("f", "", "Output folder name (required)")
	compare := flag.Bool("c", false, "Compare unique URLs found per tool")
	toolsArg := flag.String("t", "", "Run specific tool(s), comma-separated (e.g., waymore,urlfinder)")
	flag.Parse()

	if *folderName == "" {
		color.Red("Error: Output folder name (-f) is required.")
		flag.Usage()
		os.Exit(1)
	}

	printBanner()
	discovery(*domain, *folderName, *compare, *toolsArg)
}
