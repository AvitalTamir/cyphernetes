package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	colorjson "github.com/TylerBrock/colorjson"
	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/chzyer/readline"
	cobra "github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var ShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an interactive shell",
	Run:   runShell,
}

var completer = &CyphernetesCompleter{}
var printQueryExecutionTime bool = true
var disableColorJsonOutput bool = false
var multiLineInput bool = true

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func shellPrompt() string {
	ns := parser.Namespace
	color := "32"
	if ns == "" {
		ns = "ALL NAMESPACES"
		color = "31"
	}
	// Get the name of the current Kubernetes context
	context, err := getCurrentContext()
	if err != nil {
		fmt.Println("Error getting current context: ", err)
		return ""
	}

	return fmt.Sprintf("\033[%sm(%s) %s »\033[0m ", color, context, ns)
}

func getCurrentContext() (string, error) {
	// Use the local kubeconfig context
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: clientcmd.RecommendedHomeFile},
		&clientcmd.ConfigOverrides{
			CurrentContext: "",
		}).RawConfig()
	if err != nil {
		fmt.Println("Error creating in-cluster config")
		return "", err
	}
	currentContextName := config.CurrentContext
	return currentContextName, nil
}

type syntaxHighlighter struct{}

var (
	keywordsRegex       = regexp.MustCompile(`(?i)\b(match|where|set|delete|create)\b`)
	bracketsRegex       = regexp.MustCompile(`[\(\)\[\]\{\}\<\>]`)
	variableRegex       = regexp.MustCompile(`"(.*?)"`)
	identifierRegex     = regexp.MustCompile(`0m(\w+):(\w+)`)
	propertiesRegex     = regexp.MustCompile(`\{(\w+): "([^"]+)"\}`)
	returnRegex         = regexp.MustCompile(`(?i)(return)(\s+.*)`)
	returnJsonPathRegex = regexp.MustCompile(`(\.|\*)`)
)

func (h *syntaxHighlighter) Paint(line []rune, pos int) []rune {
	lineStr := string(line)

	// Coloring for brackets ((), {}, [], <>)
	lineStr = bracketsRegex.ReplaceAllString(lineStr, "\033[37m$0\033[0m") // White for brackets

	// Coloring for keywords
	lineStr = keywordsRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := keywordsRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			return "\033[35m" + strings.ToUpper(parts[1]) + "\033[0m"
		}
		return match
	})

	// Coloring for quoted variables
	lineStr = variableRegex.ReplaceAllString(lineStr, "\033[90m$0\033[0m") // Dark grey for quoted variables

	// Apply coloring for properties in format {key: "value", ...}
	lineStr = propertiesRegex.ReplaceAllString(lineStr, "{\033[33m$1\033[0m: \033[36m$2\033[0m}") // Yellow for key, Cyan for value

	// Coloring for identifiers (left and right of the colon)
	lineStr = identifierRegex.ReplaceAllString(lineStr, "\033[33m$1\033[0m:\033[94m$2\033[0m") // Orange for left, Light blue for right

	// Coloring everything after RETURN in purple
	lineStr = returnRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := returnRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			// Color "RETURN" in purple and keep the rest of the string in the same color
			rest := parts[2]
			// Apply white color to dots and asterisks in the JSONPath list
			rest = returnJsonPathRegex.ReplaceAllString(rest, "\033[37m$1\033[35m")
			return "\033[35m" + strings.ToUpper(parts[1]) + rest
		}
		return match
	})

	// add color reset to the end of the line
	lineStr += "\033[0m"
	return []rune(lineStr)
}

func runShell(cmd *cobra.Command, args []string) {
	historyFile := os.Getenv("HOME") + "/.cyphernetes_history"
	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 shellPrompt(),
		HistoryFile:            historyFile,
		AutoComplete:           completer,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
		Painter:                &syntaxHighlighter{},
		DisableAutoSaveHistory: true,

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()
	rl.CaptureExitSignal()

	fmt.Println("Cyphernetes Interactive Shell")
	fmt.Println("Type 'exit' or press Ctrl-D to exit")
	fmt.Println("Type 'help' for information on how to use the shell")
	// Initialize the GRV cache
	parser.FetchAndCacheGVRs(executor.Clientset)
	initResourceSpecs()

	var cmds []string
	var input string

	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, Ctrl-D
			break
		}
		if multiLineInput {
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			cmds = append(cmds, line)
			if !strings.HasSuffix(line, ";") && !strings.HasPrefix(line, "\\") && line != "exit" && line != "help" {
				rl.SetPrompt(">>> ")
				continue
			}
			cmd := strings.Join(cmds, " ")
			cmd = strings.TrimSuffix(cmd, ";")
			cmds = cmds[:0]
			rl.SetPrompt(shellPrompt())
			rl.SaveHistory(cmd)

			input = strings.TrimSpace(cmd)
		} else {
			input = strings.TrimSpace(line)
		}

		if input == "exit" {
			break
		}

		if strings.HasPrefix(input, "\\n ") {
			input = strings.TrimPrefix(input, "\\n ")
			if strings.ToLower(input) == "all" {
				parser.Namespace = ""
			} else {
				parser.Namespace = strings.ToLower(input)
			}
			rl.SetPrompt(shellPrompt())
		} else if input == "\\d" {
			// Toggle debug mode
			if parser.LogLevel == "debug" {
				parser.LogLevel = "info"
			} else {
				parser.LogLevel = "debug"
			}
			fmt.Printf("Debug mode: %s\n", parser.LogLevel)
		} else if input == "\\q" {
			// Toggle print query execution time
			if printQueryExecutionTime {
				printQueryExecutionTime = false
			} else {
				printQueryExecutionTime = true
			}
			fmt.Printf("Print query execution time: %t\n", printQueryExecutionTime)
		} else if input == "\\pc" {
			// Print the cache
			parser.PrintCache()
		} else if input == "\\cc" {
			// Clear the cache
			parser.ClearCache()
			fmt.Println("Cache cleared")
		} else if input == "\\r" {
			// Toggle colorized JSON output
			disableColorJsonOutput = !disableColorJsonOutput
			fmt.Printf("Raw output mode: %t\n", disableColorJsonOutput)
		} else if input == "\\m" {
			// Toggle multi-line input mode
			multiLineInput = !multiLineInput
			fmt.Printf("Multi-line input mode: %t\n", multiLineInput)
		} else if input == "help" {
			fmt.Println("Cyphernetes Interactive Shell")
			fmt.Println("exit               - Exit the shell")
			fmt.Println("help               - Print this help message")
			fmt.Println("\\n <namespace>|all - Change the namespace context")
			fmt.Println("\\m                 - Toggle multi-line input mode (execute query on ';')")
			fmt.Println("\\q                 - Toggle print query execution time")
			fmt.Println("\\r                 - Toggle raw output (disable colorized JSON)")
			fmt.Println("\\d                 - Toggle debug mode")
			fmt.Println("\\cc                - Clear the cache")
			fmt.Println("\\pc                - Print the cache")
		} else if input != "" {
			// Process the input if not empty
			result, err := processQuery(input)
			if err != nil {
				fmt.Printf("Error >> %s\n", err)
			} else {
				if !disableColorJsonOutput {
					result = colorizeJson(result)
				}
				if result != "{}" {
					fmt.Println(result)
				}
				if printQueryExecutionTime {
					fmt.Printf("\nQuery executed in %s\n\n", execTime)
				}
			}
		}
		// Add input to history
		rl.SaveHistory(input)
	}
}

// Execute the query against the Kubernetes API.
var executor = parser.GetQueryExecutorInstance()
var execTime time.Duration

func processQuery(query string) (string, error) {
	// take a measurement of the time it takes to execute the query
	startTime := time.Now()

	// Parse the query to get an AST.
	ast, err := parser.ParseQuery(query)

	if err != nil {
		// Handle error.
		return "", fmt.Errorf("error parsing query >> %s", err)
	}

	results, err := executor.Execute(ast)
	if err != nil {
		// Handle error.
		return "", fmt.Errorf("error executing query >> %s", err)
	}

	// Measure the time it took to execute the query
	execTime = time.Since(startTime)

	// Print the results as pretty JSON.
	json, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		// Handle error.
		return "", fmt.Errorf("error marshalling results >> %s", err)
	}
	return string(json), nil
}

func colorizeJson(jsonString string) string {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonString), &obj)
	if err != nil {
		fmt.Println("Error unmarshalling json: ", err)
		return jsonString
	}

	f := colorjson.NewFormatter()
	f.Indent = 2
	s, err := f.Marshal(obj)
	if err != nil {
		fmt.Println("Error marshalling json: ", err)
		return jsonString
	}
	return string(s)
}

func init() {
	rootCmd.AddCommand(ShellCmd)

	// Here you can define flags and configuration settings for the 'shell' subcommand if needed
}
