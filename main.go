// ufinder.go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// Helper: conta as linhas de um arquivo de forma eficiente
func countLines(filePath string) int {
	if !fileExists(filePath) {
		return 0
	}
	// Usamos o 'wc -l' que é extremamente rápido e eficiente
	out, err := exec.Command("sh", "-c", fmt.Sprintf("wc -l < %s", filePath)).Output()
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

// Imprime o banner em ASCII
func printBanner() {
	myFigure := figure.NewFigure("UFINDER", "slant", true)
	color.Cyan(myFigure.String())
	color.Yellow("\nby Gilson Oliveira\n\n")
}

// Imprime estatísticas da ferramenta com cores
func printStat(tool string, count, newCount int) {
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s URLs found: %d (New: %d)\n", green("["+strings.ToUpper(tool)+"]"), count, newCount)
}

// Executa um comando shell e lida com erros de forma padronizada
func runShellCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	// cmd.Stderr = os.Stderr // Descomente para debugar erros dos comandos
	return cmd.Run()
}

// Executa o comando da ferramenta, atualiza os arquivos e calcula os novos itens de forma eficiente
func runTool(command, toolName, outputFile string) (int, int) {
	rawFile := outputFile + ".raw"       // Arquivo que acumula TODOS os resultados da ferramenta
	newOutputFile := outputFile + ".new" // Arquivo temporário para a nova saída
	lastFile := filepath.Join(filepath.Dir(outputFile), "last_"+toolName+".txt")

	// Garante que o diretório existe
	os.MkdirAll(filepath.Dir(outputFile), 0755)

	// 1. Executa o comando e redireciona a saída DIRETAMENTE para o arquivo .new
	// Isso evita carregar a saída do comando na memória do Go
	fullCommand := fmt.Sprintf("%s | sort -u > %s", command, newOutputFile)
	if err := runShellCommand(fullCommand); err != nil {
		color.Red("[%s] Error running command: %v", strings.ToUpper(toolName), err)
		return countLines(rawFile), 0
	}

	var newFoundCount int
	// 2. Calcula os itens novos (delta) usando 'comm', que é feito para isso.
	// comm -13 compara dois arquivos ordenados e mostra apenas as linhas únicas do segundo.
	if fileExists(rawFile) {
		// Garante que o arquivo raw está ordenado antes de comparar
		runShellCommand(fmt.Sprintf("sort -u -o %s %s", rawFile, rawFile))

		deltaCommand := fmt.Sprintf("comm -13 %s %s > %s", rawFile, newOutputFile, lastFile)
		runShellCommand(deltaCommand)
		newFoundCount = countLines(lastFile)
	} else {
		// Se o arquivo .raw não existe, todos os resultados são novos.
		runShellCommand(fmt.Sprintf("cp %s %s", newOutputFile, lastFile))
		newFoundCount = countLines(lastFile)
	}

	// 3. Atualiza o arquivo .raw com os novos resultados, sem carregar nada na memória
	// Concatena o antigo e o novo, ordena e remove duplicatas, e salva no lugar.
	updateCommand := fmt.Sprintf("cat %s %s | sort -u -o %s", rawFile, newOutputFile, rawFile)
	if fileExists(rawFile) {
		runShellCommand(updateCommand)
	} else {
		// Se o .raw não existia, o .new se torna o novo .raw
		os.Rename(newOutputFile, rawFile)
	}

	// 4. O output final da ferramenta (ex: gau.txt) é uma cópia do .raw atualizado
	runShellCommand(fmt.Sprintf("cp %s %s", rawFile, outputFile))

	// Limpa o arquivo temporário
	os.Remove(newOutputFile)

	totalCount := countLines(rawFile)
	return totalCount, newFoundCount
}

// Agrega os resultados de vários arquivos em um único arquivo mestre (urls.txt) de forma eficiente
func aggregateResults(toolFiles []string, urlsFile string) {
	// Cria uma lista de arquivos que realmente existem para evitar erros no 'cat'
	var existingFiles []string
	if fileExists(urlsFile) {
		existingFiles = append(existingFiles, urlsFile)
	}
	for _, file := range toolFiles {
		if fileExists(file) {
			existingFiles = append(existingFiles, file)
		}
	}

	if len(existingFiles) == 0 {
		return // Nenhum arquivo para agregar
	}

	// Concatena todos os arquivos, ordena, remove duplicatas e salva no arquivo mestre.
	// Esta operação usa memória mínima, independentemente do tamanho dos arquivos.
	aggregateCommand := fmt.Sprintf("cat %s | sort -u -o %s", strings.Join(existingFiles, " "), urlsFile)
	if err := runShellCommand(aggregateCommand); err != nil {
		color.Red("Error aggregating results into master file: %v", err)
	}
}

func discovery(domain, folderName string, toolsArg string) {
	baseDir := folderName
	endpointsDir := filepath.Join(baseDir, "endpoints")
	os.MkdirAll(endpointsDir, 0755)

	urlsFile := filepath.Join(endpointsDir, "urls.txt")
	oldGlobalCount := countLines(urlsFile)

	// Define os comandos para cada ferramenta
	// IMPORTANTE: waymore com -oU já salva em arquivo, então tratamos de forma diferente
	toolCommands := map[string]string{
		"waybackurls": fmt.Sprintf("waybackurls %s", domain),
		"gau":         fmt.Sprintf("gau %s --subs", domain),
		"xurlfind3r":  fmt.Sprintf("xurlfind3r -d %s --include-subdomains -s", domain),
		"urlscan": fmt.Sprintf(`curl -s "https://urlscan.io/api/v1/search/?q=domain:%s&size=10000" -H "API-Key: %s" | jq -r '.results[].page.url'`,
			domain, os.Getenv("URLSCAN")),
		"urlfinder": fmt.Sprintf("urlfinder -d %s -all", domain),
		"ducker":    fmt.Sprintf("ducker -q 'site:%s' -c 1000", domain),
	}
	// Arquivo de saída de cada ferramenta
	toolFiles := map[string]string{
		"waymore":     filepath.Join(endpointsDir, "waymore.txt"),
		"waybackurls": filepath.Join(endpointsDir, "waybackurls.txt"),
		"gau":         filepath.Join(endpointsDir, "gau.txt"),
		"xurlfind3r":  filepath.Join(endpointsDir, "xurlfind3r.txt"),
		"urlscan":     filepath.Join(endpointsDir, "urlscan.txt"),
		"urlfinder":   filepath.Join(endpointsDir, "urlfinder.txt"),
		"ducker":      filepath.Join(endpointsDir, "ducker.txt"),
	}

	if domain == "" {
		color.Red("Error: You must provide a domain (-d).")
		return
	}

	// Seleciona as ferramentas a serem executadas
	var selectedTools []string
	if toolsArg != "" {
		selectedTools = strings.Split(toolsArg, ",")
	} else {
		for tool := range toolFiles { // Usamos toolFiles para incluir 'waymore'
			selectedTools = append(selectedTools, tool)
		}
	}

	// Executa cada ferramenta em uma goroutine
	var wg sync.WaitGroup
	for _, tool := range selectedTools {
		tool = strings.TrimSpace(tool)

		// Tratamento especial para waymore, que já tem flag de output
		if tool == "waymore" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				waymoreOutputFile := toolFiles["waymore"]
				// O comando do waymore já salva no arquivo, então o adaptamos para nosso fluxo
				waymoreCmd := fmt.Sprintf("waymore -i %s -mode U -oU %s", domain, waymoreOutputFile+".new")
				total, newFound := runTool(waymoreCmd, "waymore", waymoreOutputFile)
				printStat("waymore", total, newFound)
			}()
			continue // Pula para a próxima ferramenta
		}

		if cmdStr, exists := toolCommands[tool]; exists {
			wg.Add(1)
			go func(tool, cmdStr string) {
				defer wg.Done()
				total, newFound := runTool(cmdStr, tool, toolFiles[tool])
				printStat(tool, total, newFound)
			}(tool, cmdStr)
		} else {
			color.Red("Invalid tool specified: %s", tool)
		}
	}
	wg.Wait()

	color.Cyan("\nAggregating all results into urls.txt...")
	var filesToAggregate []string
	for _, toolName := range selectedTools {
		if file, ok := toolFiles[strings.TrimSpace(toolName)]; ok {
			filesToAggregate = append(filesToAggregate, file)
		}
	}

	aggregateResults(filesToAggregate, urlsFile)

	totalGlobal := countLines(urlsFile)
	additionalGlobal := totalGlobal - oldGlobalCount
	color.Green("\n[TOTAL] Unique URLs: %d (Previously: %d, New: %d)", totalGlobal, oldGlobalCount, additionalGlobal)

	// A função `filterUniquePerTool` foi removida pois era a principal causa
	// do alto consumo de memória. Calcular "URLs verdadeiramente únicas por ferramenta"
	// exige carregar todos os resultados na memória, o que é inviável para grandes volumes.
}

func main() {
	domain := flag.String("d", "", "Target domain (required)")
	folderName := flag.String("f", "", "Output folder name (required)")
	toolsArg := flag.String("t", "", "Run specific tool(s), comma-separated (e.g., waymore,gau)")
	flag.Parse()

	if *folderName == "" || *domain == "" {
		color.Red("Error: Both domain (-d) and folder name (-f) are required.")
		flag.Usage()
		os.Exit(1)
	}

	printBanner()
	discovery(*domain, *folderName, *toolsArg)
}
