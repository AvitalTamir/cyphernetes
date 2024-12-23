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

	"github.com/AvitalTamir/cyphernetes/pkg/core"
	"github.com/AvitalTamir/cyphernetes/pkg/provider/apiserver"
	colorjson "github.com/TylerBrock/colorjson"
	cobra "github.com/spf13/cobra"
	"github.com/wader/readline"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed default_macros.txt
var defaultMacros string
var executeStatementFunc = executeStatement
var ctx string

var ShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an interactive shell",
	Run:   runShell,
}

var executor *core.QueryExecutor
var execTime time.Duration
var completer = &CyphernetesCompleter{}
var printQueryExecutionTime bool = true
var returnRawJsonOutput bool = false
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
	ns := core.Namespace
	color := getPromptColor(ns)
	if ns == "" {
		ns = "ALL NAMESPACES"
	}

	prompt := fmt.Sprintf("(%s) %s »", ctx, ns)
	return wrapInColor(prompt, color) + " "
}

func multiLinePrompt() string {
	shellPromptLength := len(regexp.MustCompile(`\033\[[0-9;]*m`).ReplaceAllString(shellPrompt(), ""))
	prompt := fmt.Sprintf("%s»", strings.Repeat(" ", shellPromptLength-3))
	return wrapInColor(prompt, getPromptColor(core.Namespace)) + " "
}

func getPromptColor(ns string) int {
	if ns == "" {
		return 31
	}
	return 32
}

func SetQueryExecutor(exec *core.QueryExecutor) {
	executor = exec
}

var getCurrentContextFunc = getCurrentContextFromConfig

func getCurrentContext() (string, string, error) {
	return getCurrentContextFunc()
}

func getCurrentContextFromConfig() (string, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.RawConfig()
	if err != nil {
		return "", "", fmt.Errorf("error getting current context from kubeconfig: %v", err)
	}

	currentContextName := config.CurrentContext
	currentContext, exists := config.Contexts[currentContextName]
	if !exists {
		return "", "", fmt.Errorf("current context %s does not exist in kubeconfig", currentContextName)
	}

	namespace := currentContext.Namespace
	// We don't set a default namespace here, as it wasn't in the original code

	return currentContextName, namespace, nil
}

type syntaxHighlighter struct{}

var (
	keywordsRegex   = regexp.MustCompile(`(?i)\b(match|where|contains|set|delete|create|sum|count|as|in)\b`)
	bracketsRegex   = regexp.MustCompile(`[\(\)\[\]\{\}\<\>]`)
	variableRegex   = regexp.MustCompile(`"(.*?)"`)
	identifierRegex = regexp.MustCompile(`0m(\w+):(\w+)`)
	propertiesRegex = regexp.MustCompile(`\{((?:[^{}]|\{[^{}]*\})*)\}`)
	returnRegex     = regexp.MustCompile(`(?i)(return)(\s+.*)`)
)

func (h *syntaxHighlighter) Paint(line []rune, pos int) []rune {
	lineStr := string(line)

	// Coloring for brackets ((), {}, [], <>)
	lineStr = bracketsRegex.ReplaceAllString(lineStr, wrapInColor("$0", 37)) // White for brackets

	// Coloring for keywords
	lineStr = keywordsRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := keywordsRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			return wrapInColor(strings.ToUpper(parts[1]), 35) // Purple for keywords
		}
		return match
	})

	// Coloring for quoted variables
	lineStr = variableRegex.ReplaceAllString(lineStr, wrapInColor("$0", 90)) // Dark grey for quoted variables

	// Coloring for identifiers (left and right of the colon)
	lineStr = identifierRegex.ReplaceAllString(lineStr, wrapInColor("$1", 33)+":"+wrapInColor("$2", 94)) // Orange for left, Light blue for right

	// Color RETURN keyword and its arguments
	lineStr = returnRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := returnRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			rest := parts[2]

			nonAsterisksOrDotsRegex := `[^*.,]+`
			coloredRest := regexp.MustCompile(nonAsterisksOrDotsRegex).ReplaceAllStringFunc(rest, func(submatch string) string {
				return wrapInColor(submatch, 35) // Purple for non-asterisks or dots
			})

			return wrapInColor(strings.ToUpper(parts[1]), 35) + coloredRest
		}
		return match
	})

	lineStr = regexp.MustCompile(propertiesRegex.String()).ReplaceAllStringFunc(lineStr, func(match string) string {
		return colorizeProperties(match)
	})

	return []rune(lineStr)
}

func colorizeProperties(obj string) string {
	if core.NoColor {
		return obj
	}

	// Remove existing color codes
	stripped := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`).ReplaceAllString(obj, "")

	// Remove the outer curly braces before processing
	stripped = strings.TrimPrefix(stripped, "{")
	stripped = strings.TrimSuffix(stripped, "}")

	// Colorize properties
	colored := regexp.MustCompile(`((?:"(?:[^"\\]|\\.)*"|[^:,{}]+)\s*:\s*)("[^"]*"|[^,{}]+|(\{[^{}]*\}))`).ReplaceAllStringFunc(stripped, func(prop string) string {
		parts := regexp.MustCompile(`((?:"(?:[^"\\]|\\.)*"|[^:,{}]+)\s*:\s*)(.+)`).FindStringSubmatch(prop)
		if len(parts) == 3 {
			key := parts[1]
			value := parts[2]

			// Check if value is a nested object
			if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
				value = colorizeProperties(value) // Recursively colorize nested objects
			} else {
				value = wrapInColor(value, 36) // Cyan for non-object values
			}

			return wrapInColor(key, 33) + value
		}
		return prop
	})

	// Add back the outer curly braces
	return "{" + colored + "}"
}

type Listener interface {
	OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
}

func runShell(cmd *cobra.Command, args []string) {
	// Create the API server provider
	p, err := apiserver.NewAPIServerProvider()
	if err != nil {
		fmt.Println("Error creating provider:", err)
		os.Exit(1)
	}

	showSplash()

	// Initialize the executor instance with the provider
	executor = core.GetQueryExecutorInstance(p)
	if executor == nil {
		fmt.Println("Error initializing query executor")
		os.Exit(1)
	}

	// Get current context
	currentContext, currentNamespace, err := getCurrentContext()
	if err != nil {
		fmt.Println("Error getting current context:", err)
		os.Exit(1)
	}
	ctx = currentContext

	// Initialize shell environment
	if core.AllNamespaces {
		core.Namespace = ""
		core.AllNamespaces = false
	} else if currentNamespace != "" {
		core.Namespace = currentNamespace
	}

	// Load default macros
	macroManager = NewMacroManager()
	if err := macroManager.LoadMacrosFromString("default_macros.txt", defaultMacros); err != nil {
		fmt.Println("Error loading default macros:", err)
	}

	// Load user macros
	userMacrosFile := os.Getenv("HOME") + "/.cyphernetes/macros"
	if _, err := os.Stat(userMacrosFile); err == nil {
		if err := macroManager.LoadMacrosFromFile(userMacrosFile); err != nil {
			fmt.Printf("Error loading user macros: %v\n", err)
		}
	}

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
		UniqueEditLine:         true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	rl.Config.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		rl.Refresh()
		return line, pos, false
	})

	// Set up a channel to receive interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("")
	fmt.Println("Type 'exit' or press Ctrl-D to exit")
	fmt.Println("Type 'help' for information on how to use the shell")
	fmt.Println("")
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
		fmt.Print("\n")
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
				if !returnRawJsonOutput {
					result = formatJson(result)
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
			lastLine := rl.Config.Painter.Paint([]rune(line), 0)
			// delete one line up
			fmt.Print("\033[A\033[K")
			if len(cmds) == 1 {
				fmt.Print(shellPrompt())
			} else {
				fmt.Print(multiLinePrompt())
			}
			fmt.Println(string(lastLine))
			if !strings.HasSuffix(line, ";") && !strings.HasPrefix(line, "\\") && line != "exit" && line != "help" {
				rl.SetPrompt(multiLinePrompt())
				continue
			}
			cmd := strings.Join(cmds, " ")
			cmd = strings.TrimSuffix(cmd, ";")
			cmds = cmds[:0]
			rl.SetPrompt(shellPrompt())
			input = strings.TrimSpace(cmd)
		} else {
			input = strings.TrimSpace(line)
			lastQuery := rl.Config.Painter.Paint([]rune(input), 0)
			fmt.Println(string(lastQuery))
		}
		fmt.Print("\n")

		rl.SaveHistory(input)

		if input == "exit" {
			break
		}

		if input == "\\n" {
			fmt.Println("Namespace cannot be empty. Usage: \\n <namespace>|all")
			continue
		}

		if strings.HasPrefix(input, "\\n ") {
			input = strings.TrimPrefix(input, "\\n ")
			if strings.ToLower(input) == "all" {
				core.Namespace = ""
			} else {
				core.Namespace = strings.ToLower(input)
			}
			rl.SetPrompt(shellPrompt())
		} else if input == "\\d" {
			// Toggle debug mode
			if core.LogLevel == "debug" {
				core.LogLevel = "info"
			} else {
				core.LogLevel = "debug"
			}
			fmt.Printf("Debug mode: %s\n", core.LogLevel)
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
			executor.Provider().PrintCache()
		} else if input == "\\cc" {
			// Clear the cache
			executor.Provider().ClearCache()
			fmt.Println("Cache cleared")
		} else if input == "\\lm" {
			fmt.Println("Registered macros:")
			for name, macro := range macroManager.Macros {
				description := macro.Description
				if description == "" {
					description = "No description provided"
				}

				// print a line that looks like this but make it colorful (in case color is enabled) so that the command, args and description have distinct colors: (":%s %v - %s\n", name, macro.Args, description)
				fmt.Printf("%s %s - %s\n",
					wrapInColor(":"+name, 33),
					wrapInColor(fmt.Sprint(macro.Args), 36),
					wrapInColor(description, 35))
			}
		} else if input == "\\r" {
			// Toggle raw json output
			returnRawJsonOutput = !returnRawJsonOutput
			fmt.Printf("Raw output mode: %t\n", returnRawJsonOutput)
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
			if !returnRawJsonOutput {
				result = formatJson(result)
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

func processQuery(query string) (string, core.Graph, error) {
	startTime := time.Now()

	query = strings.TrimSuffix(query, ";")

	var result string
	var graph core.Graph
	var err error

	if strings.HasPrefix(query, ":") {
		macroName := strings.TrimPrefix(query, ":")
		parts := strings.Fields(macroName)
		macroName = parts[0]
		args := parts[1:]

		statements, err := macroManager.ExecuteMacro(macroName, args)
		if err != nil {
			return "", core.Graph{}, err
		}

		var results []string
		var graphInternal core.Graph
		for i, stmt := range statements {
			result, err := executeStatementFunc(stmt)
			if err != nil {
				return "", core.Graph{}, fmt.Errorf("error executing statement %d: %w", i+1, err)
			}

			// unmarshal the result into a map[string]interface{}
			var resultMap map[string]interface{}
			err = json.Unmarshal([]byte(result), &resultMap)
			if err != nil {
				return "", core.Graph{}, fmt.Errorf("error unmarshalling result: %w", err)
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
			return "", core.Graph{}, err
		}
		var resultMap map[string]interface{}
		err = json.Unmarshal([]byte(res), &resultMap)
		if err != nil {
			return "", core.Graph{}, fmt.Errorf("error unmarshalling result: %w", err)
		}

		buildDataAndGraph(resultMap, &result, &graph)
	}

	execTime = time.Since(startTime)
	return result, graph, err
}

func buildDataAndGraph(resultMap map[string]interface{}, result *string, graph *core.Graph) error {
	// check if interface is nil
	if graphInternal, ok := resultMap["Graph"]; ok {
		// check that graphInternal has "Nodes" and "Edges"
		if nodes, ok := graphInternal.(map[string]interface{})["Nodes"]; ok {
			nodeIds := nodes.([]interface{})
			for _, nodeId := range nodeIds {
				graph.Nodes = append(graph.Nodes, core.Node{
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
					graph.Edges = append(graph.Edges, core.Edge{
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
	ast, err := core.ParseQuery(query)
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

func formatJson(jsonString string) string {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonString), &obj)
	if err != nil {
		// Not a valid JSON object, likely an empty response.
		// We simply return the non-colorized payload.
		return jsonString
	}

	if core.NoColor {
		s, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling json: ", err)
			return jsonString
		}
		return string(s)
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

	// Get the name of the current Kubernetes context
	contextName, namespace, err := getCurrentContext()
	if err != nil {
		fmt.Println("Error getting current context: ", err)
		return
	}
	ctx = contextName

	if namespace != "" && namespace != "default" {
		core.Namespace = namespace
	}

	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		core.NoColor = true
	}
}

func handleInterrupt(rl *readline.Instance, cmds *[]string, executing *bool) {
	if *executing {
		// If we're executing a query, do nothing
		return
	}

	if len(*cmds) == 0 {
		// If the input is empty, exit the program
		os.Exit(0)
	}

	// Clear the current input and reset the prompt
	*cmds = []string{}
	rl.SetPrompt(shellPrompt())
	rl.Refresh()
}

func wrapInColor(input string, color int) string {
	if core.NoColor {
		return input
	}
	return fmt.Sprintf("\033[%dm%s\033[0m", color, input)
}

func InitShell() {
	if executor == nil {
		return
	}
	if err := core.InitResourceSpecs(executor.Provider()); err != nil {
		fmt.Printf("Error initializing resource specs: %v\n", err)
	}
}

func showSplash() {
	logDebug("Showing splash")
	fmt.Println(`
                __                    __        
 ______ _____  / /  ___ _______  ___ / /____ ___
/ __/ // / _ \/ _ \/ -_) __/ _ \/ -_) __/ -_|_-<
\__/\_, / .__/_//_/\__/_/ /_//_/\__/\__/\__/___/
   /___/_/ Interactive Shell`)
	fmt.Println("")
}
