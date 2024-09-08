package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"os/signal"
	"syscall"

	colorjson "github.com/TylerBrock/colorjson"
	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/chzyer/readline"
	cobra "github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed default_macros.txt
var defaultMacros string
var executeStatementFunc = executeStatement

var ShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an interactive shell",
	Run:   runShell,
}

var executor = parser.GetQueryExecutorInstance()
var execTime time.Duration
var completer = &CyphernetesCompleter{}
var printQueryExecutionTime bool = true
var disableColorJsonOutput bool = false
var disableGraphOutput bool = true
var graphLayoutLR bool = true
var multiLineInput bool = true
var macroManager = NewMacroManager()

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

func SetQueryExecutor(exec *parser.QueryExecutor) {
	executor = exec
}

var getCurrentContextFunc = getCurrentContextFromConfig

func getCurrentContext() (string, error) {
	return getCurrentContextFunc()
}

func getCurrentContextFromConfig() (string, error) {
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
	keywordsRegex       = regexp.MustCompile(`(?i)\b(match|where|set|delete|create|sum|count|as)\b`)
	bracketsRegex       = regexp.MustCompile(`[\(\)\[\]\{\}\<\>]`)
	variableRegex       = regexp.MustCompile(`"(.*?)"`)
	identifierRegex     = regexp.MustCompile(`0m(\w+):(\w+)`)
	propertiesRegex     = regexp.MustCompile(`\{([^{}]*(\{[^{}]*\}[^{}]*)*)\}`)
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

	// Coloring for identifiers (left and right of the colon)
	lineStr = identifierRegex.ReplaceAllString(lineStr, "\033[33m$1\033[0m:\033[94m$2\033[0m") // Orange for left, Light blue for right

	// Coloring everything after RETURN in purple
	lineStr = returnRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := returnRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			rest := parts[2]
			// Apply white color to dots and asterisks in the JSONPath list
			rest = returnJsonPathRegex.ReplaceAllString(rest, "\033[37m$1\033[35m")
			// Apply white color to commas and keep the rest purple
			rest = strings.ReplaceAll(rest, ",", "\033[37m,\033[35m")
			return "\033[35m" + strings.ToUpper(parts[1]) + rest
		}
		return match
	})

	// in lineStr, find all text (.*) between { and } and strip all color codes:
	// - all text that looks like "\x1b[33m" or "\x1b[36m" or "\x1b[90m" or "\x1b[37m" or "\x1b[35m" or "\x1b[0m"
	// - all text that looks like "\x1b[([0-9;]+)m"
	// - all text that looks like "\033[33m" or "\033[36m" or "\033[90m" or "\033[37m" or "\033[35m" or "\033[0m"
	// // Strip color codes from text between { and }
	// lineStr = regexp.MustCompile(`\{([^}]*)\}`).ReplaceAllStringFunc(lineStr, func(match string) string {
	// 	// Remove all ANSI color codes
	// 	stripped := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`).ReplaceAllString(match, "")

	// 	// Apply new coloring while preserving original spacing
	// 	colored := propertiesRegex.ReplaceAllStringFunc(stripped, func(propMatch string) string {
	// 		parts := propertiesRegex.FindStringSubmatch(propMatch)
	// 		if len(parts) == 4 {
	// 			key := parts[1]
	// 			spacing := parts[2]
	// 			value := parts[3]
	// 			return fmt.Sprintf("\033[33m%s\033[0m%s\033[36m%s\033[0m", key, spacing, value)
	// 		}
	// 		return propMatch
	// 	})

	// 	// Ensure color is reset at the end
	// 	return colored
	// })

	// Colorize properties
	lineStr = regexp.MustCompile(propertiesRegex.String()).ReplaceAllStringFunc(lineStr, func(match string) string {
		return colorizeProperties(match)
	})

	// Ensure color is reset at the end of the entire line
	lineStr += "\033[0m"
	return []rune(lineStr)
}

func colorizeProperties(obj string) string {
	// Remove existing color codes
	stripped := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`).ReplaceAllString(obj, "")

	// Remove the outer curly braces before processing
	stripped = strings.TrimPrefix(stripped, "{")
	stripped = strings.TrimSuffix(stripped, "}")

	// Colorize properties
	colored := regexp.MustCompile(`("?\w+"?)(\s*:\s*)("[^"]*"|[^,{}]+|(\{[^{}]*\}))`).ReplaceAllStringFunc(stripped, func(prop string) string {
		parts := regexp.MustCompile(`("?\w+"?)(\s*:\s*)(.+)`).FindStringSubmatch(prop)
		if len(parts) == 4 {
			key := parts[1]
			spacing := parts[2]
			value := parts[3]

			// Check if value is a nested object
			if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
				value = colorizeProperties(value) // Recursively colorize nested objects
			} else {
				value = "\033[36m" + value + "\033[0m" // Cyan for non-object values
			}

			return fmt.Sprintf("\033[33m%s\033[0m%s%s", key, spacing, value)
		}
		return prop
	})

	// Add back the outer curly braces
	return "{" + colored + "}"
}

func runShell(cmd *cobra.Command, args []string) {
	historyFile := os.Getenv("HOME") + "/.cyphernetes/history"
	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 shellPrompt(),
		HistoryFile:            historyFile,
		AutoComplete:           completer,
		InterruptPrompt:        "", // Set this to empty string
		EOFPrompt:              "exit",
		Painter:                &syntaxHighlighter{},
		DisableAutoSaveHistory: true,
		HistorySearchFold:      true,
		FuncFilterInputRune:    filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	// Set up a channel to receive interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Cyphernetes Interactive Shell")
	fmt.Println("Type 'exit' or press Ctrl-D to exit")
	fmt.Println("Type 'help' for information on how to use the shell")
	// Initialize the GRV cache
	parser.FetchAndCacheGVRs(executor.Clientset)
	initResourceSpecs()

	var cmds []string
	var input string
	executing := false

	go func() {
		for range sigChan {
			handleInterrupt(rl, &cmds, &executing)
		}
	}()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					fmt.Println("\nExiting...")
					return
				}
				continue
			} else if err == io.EOF {
				break
			}
			fmt.Println("Error reading input:", err)
			continue
		}

		if strings.HasPrefix(line, ":") {
			// Execute macro immediately
			result, err := executeMacro(line)
			if err != nil {
				fmt.Printf("Error >> %s\n", err)
			} else {
				if !disableColorJsonOutput {
					result = colorizeJson(result)
				}
				fmt.Println(result)
				if printQueryExecutionTime {
					fmt.Printf("\nMacro executed in %s\n\n", execTime)
				}
			}
			rl.SaveHistory(line)
			continue
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
			input = strings.TrimSpace(cmd)
		} else {
			input = strings.TrimSpace(line)
		}
		rl.SaveHistory(input)

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
		} else if input == "\\lm" {
			fmt.Println("Registered macros:")
			for name, macro := range macroManager.Macros {
				description := macro.Description
				if description == "" {
					description = "No description provided"
				}
				// print a line that looks like this but make it colorful so that the command, args and description have distinct colors: (":%s %v - %s\n", name, macro.Args, description)
				fmt.Printf("\033[33m:%s\033[0m \033[36m%v\033[0m - \033[35m%s\033[0m\n", name, macro.Args, description)
			}
		} else if input == "\\r" {
			// Toggle colorized JSON output
			disableColorJsonOutput = !disableColorJsonOutput
			fmt.Printf("Raw output mode: %t\n", disableColorJsonOutput)
		} else if input == "\\m" {
			// Toggle multi-line input mode
			multiLineInput = !multiLineInput
			fmt.Printf("Multi-line input mode: %t\n", multiLineInput)
		} else if input == "\\g" {
			// Toggle graph output
			disableGraphOutput = !disableGraphOutput
			fmt.Printf("Disable graph output: %t\n", disableGraphOutput)
		} else if input == "\\gl" {
			// Toggle graph layout
			graphLayoutLR = !graphLayoutLR
			if graphLayoutLR {
				fmt.Println("Graph layout: Left to Right")
			} else {
				fmt.Println("Graph layout: Top to Bottom")
			}
		} else if input == "help" {
			fmt.Println("Cyphernetes Interactive Shell")
			fmt.Println("exit               - Exit the shell")
			fmt.Println("help               - Print this help message")
			fmt.Println("\\n <namespace>|all - Change the namespace context")
			fmt.Println("\\gl                - Toggle graph layout (Left to Right or Top to Bottom)")
			fmt.Println("\\g                 - Toggle graph output")
			fmt.Println("\\m                 - Toggle multi-line input mode (execute query on ';')")
			fmt.Println("\\q                 - Toggle print query execution time")
			fmt.Println("\\r                 - Toggle raw output (disable colorized JSON)")
			fmt.Println("\\d                 - Toggle debug mode")
			fmt.Println("\\cc                - Clear the cache")
			fmt.Println("\\pc                - Print the cache")
			fmt.Println("\\lm                - List all registered macros")
			fmt.Println(":macro_name [args] - Execute a macro")
		} else if input != "" {
			executing = true
			// Process the input if not empty
			result, graph, err := processQuery(input)
			executing = false
			if err != nil {
				fmt.Printf("Error >> %s\n", err)
				continue
			}
			if !disableGraphOutput {
				graphAscii, err := drawGraph(graph, result)
				if err != nil {
					fmt.Printf("Error >> %s\n", err)
				} else {
					fmt.Println(graphAscii)
				}
			}
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
}

func processQuery(query string) (string, parser.Graph, error) {
	startTime := time.Now()

	query = strings.TrimSuffix(query, ";")

	var result string
	var graph parser.Graph
	var err error

	if strings.HasPrefix(query, ":") {
		macroName := strings.TrimPrefix(query, ":")
		parts := strings.Fields(macroName)
		macroName = parts[0]
		args := parts[1:]

		statements, err := macroManager.ExecuteMacro(macroName, args)
		if err != nil {
			return "", parser.Graph{}, err
		}

		var results []string
		var graphInternal parser.Graph
		for i, stmt := range statements {
			result, err := executeStatementFunc(stmt)
			if err != nil {
				return "", parser.Graph{}, fmt.Errorf("error executing statement %d: %w", i+1, err)
			}

			// unmarshal the result into a map[string]interface{}
			var resultMap map[string]interface{}
			err = json.Unmarshal([]byte(result), &resultMap)
			if err != nil {
				return "", parser.Graph{}, fmt.Errorf("error unmarshalling result: %w", err)
			}

			buildDataAndGraph(resultMap, &result, &graphInternal)

			// add the result to the graph
			graph = mergeGraphs(graph, graphInternal)

			// check if the result has a key called "Data"
			if resultMap["Data"] != nil {
				results = append(results, resultMap["Data"].(string))
			}
		}

		result = strings.Join(results, "\n")
	} else {
		res, err := executeStatement(query)
		if err != nil {
			return "", parser.Graph{}, err
		}
		var resultMap map[string]interface{}
		err = json.Unmarshal([]byte(res), &resultMap)
		if err != nil {
			return "", parser.Graph{}, fmt.Errorf("error unmarshalling result: %w", err)
		}

		buildDataAndGraph(resultMap, &result, &graph)
	}

	execTime = time.Since(startTime)
	return result, graph, err
}

func buildDataAndGraph(resultMap map[string]interface{}, result *string, graph *parser.Graph) error {
	// check if interface is nil
	if graphInternal, ok := resultMap["Graph"]; ok {
		// check that graphInternal has "Nodes" and "Edges"
		if nodes, ok := graphInternal.(map[string]interface{})["Nodes"]; ok {
			nodeIds := nodes.([]interface{})
			for _, nodeId := range nodeIds {
				graph.Nodes = append(graph.Nodes, parser.Node{
					Id:   nodeId.(map[string]interface{})["Id"].(string),
					Name: nodeId.(map[string]interface{})["Name"].(string),
					Kind: nodeId.(map[string]interface{})["Kind"].(string),
				})
			}
		}
		if edges, ok := graphInternal.(map[string]interface{})["Edges"]; ok {
			for _, edge := range edges.([]interface{}) {
				// check LeftNode and RightNode are not nil
				if edge.(map[string]interface{})["From"] != nil && edge.(map[string]interface{})["To"] != nil && edge.(map[string]interface{})["Type"] != nil {
					graph.Edges = append(graph.Edges, parser.Edge{
						From: edge.(map[string]interface{})["From"].(string),
						To:   edge.(map[string]interface{})["To"].(string),
						Type: edge.(map[string]interface{})["Type"].(string),
					})
				}
			}
		}
	}

	if data, ok := resultMap["Data"]; ok {
		resultBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("error marshalling data: %w", err)
		}
		*result = string(resultBytes)
	} else {
		*result = "{}"
	}
	return nil
}

func executeStatement(query string) (string, error) {
	ast, err := parser.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("error parsing query >> %s", err)
	}

	results, err := executor.Execute(ast, "")
	if err != nil {
		return "", fmt.Errorf("error executing query >> %s", err)
	}

	// Check if results is nil or empty
	if results.Data == nil || (reflect.ValueOf(results.Data).Kind() == reflect.Map && len(results.Data) == 0) {
		return "{}", nil
	}

	json, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling results >> %s", err)
	}
	return string(json), nil
}

func colorizeJson(jsonString string) string {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonString), &obj)
	if err != nil {
		// Not a valid JSON object, likely an empty response.
		// We simply return the non-colorized payload.
		return jsonString
	}

	f := colorjson.NewFormatter()
	f.Indent = 2
	s, err := f.Marshal(obj)
	if err != nil {
		// This was valid JSON that got colored but cannot be marshaled back
		// This indicated an issue in the coloring itself and should be examined
		// We print out the error message and return the non-colorized payload
		fmt.Println("Error marshalling colorized json: ", err)
		return jsonString
	}
	return string(s)
}

func init() {
	rootCmd.AddCommand(ShellCmd)

	// Create the .cyphernetes directory if it doesn't exist
	if _, err := os.Stat(os.Getenv("HOME") + "/.cyphernetes"); os.IsNotExist(err) {
		os.MkdirAll(os.Getenv("HOME")+"/.cyphernetes", os.ModePerm)
	}

	// Load default macros from the embedded content
	if err := macroManager.LoadMacrosFromString("default_macros.txt", defaultMacros); err != nil {
		fmt.Printf("Error loading default macros: %v\n", err)
	}

	// Load user macros
	userMacrosFile := os.Getenv("HOME") + "/.cyphernetes/macros"
	if _, err := os.Stat(userMacrosFile); err == nil {
		if err := macroManager.LoadMacrosFromFile(userMacrosFile); err != nil {
			fmt.Printf("Error loading user macros: %v\n", err)
		}
	}
}

func handleInterrupt(rl *readline.Instance, cmds *[]string, executing *bool) {
	if *executing {
		// If we're executing a query, do nothing
		return
	}

	if len(*cmds) == 0 {
		// If the input is empty, exit the program
		fmt.Println("\nExiting...")
		os.Exit(0)
	}

	// Clear the current input and reset the prompt
	*cmds = []string{}
	rl.SetPrompt(shellPrompt())
	rl.Refresh()
}
