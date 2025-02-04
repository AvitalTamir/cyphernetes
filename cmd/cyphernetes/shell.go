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
	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	cobra "github.com/spf13/cobra"
	"github.com/wader/readline"
	"gopkg.in/yaml.v3"
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
	Long:  `Start an interactive shell for executing Cypher-inspired queries against Kubernetes.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate format flag
		f := cmd.Flag("format").Value.String()
		if f != "yaml" && f != "json" {
			return fmt.Errorf("invalid value for --format: must be 'json' or 'yaml'")
		}
		// Get the name of the current Kubernetes context
		contextName, namespace, err := getCurrentContext()
		if err != nil {
			return fmt.Errorf("error getting current context: %v", err)
		}
		ctx = contextName

		if namespace != "" && namespace != "default" {
			core.Namespace = namespace
		}

		// Initialize kubernetes before running the command
		return initializeKubernetes()
	},
	Run: func(cmd *cobra.Command, args []string) {
		showSplash()

		// Create provider with dry-run config
		provider, err := apiserver.NewAPIServerProviderWithOptions(&apiserver.APIServerProviderConfig{
			DryRun: DryRun,
		})
		if err != nil {
			fmt.Printf("Error creating provider: %v\n", err)
			return
		}

		executor = core.GetQueryExecutorInstance(provider)
		if executor == nil {
			return
		}

		initAndRunShell(cmd, args)
	},
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
	identifierRegex = regexp.MustCompile(`\(([^:)]*?)(?::([^)]+))?\)`)
	propertiesRegex = regexp.MustCompile(`{([^{}]+)}`)
	returnRegex     = regexp.MustCompile(`(?i)(return)(\s+.*)`)
)

func (h *syntaxHighlighter) Paint(line []rune, pos int) []rune {
	lineStr := string(line)

	// Coloring for keywords
	lineStr = keywordsRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := keywordsRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			return wrapInColor(strings.ToUpper(parts[1]), 35) // Purple for keywords
		}
		return match
	})

	// Coloring for node patterns (must come before properties)
	lineStr = identifierRegex.ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := identifierRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			if parts[1] == "" && parts[2] != "" {
				// Case 2: (:word) - kindless node
				return wrapInColor("(", 37) + ":" + wrapInColor(parts[2], 94) + wrapInColor(")", 37)
			} else if parts[1] != "" && parts[2] == "" {
				// Case 1: (word) - anonymous node
				return wrapInColor("(", 37) + wrapInColor(parts[1], 33) + wrapInColor(")", 37)
			} else if parts[1] != "" && parts[2] != "" {
				// Case 3: (word:word) - standard node
				return wrapInColor("(", 37) + wrapInColor(parts[1], 33) + ":" + wrapInColor(parts[2], 94) + wrapInColor(")", 37)
			}
		}
		return match
	})

	// Color relationship patterns
	lineStr = regexp.MustCompile(`\[(:([^\]]+))\]`).ReplaceAllStringFunc(lineStr, func(match string) string {
		parts := regexp.MustCompile(`\[(:([^\]]+))\]`).FindStringSubmatch(match)
		if len(parts) == 3 {
			return wrapInColor("[", 37) + wrapInColor(":"+parts[2], 94) + wrapInColor("]", 37)
		}
		return match
	})

	// Color RETURN keyword and its arguments with JSONPath
	lineStr = regexp.MustCompile(`(?i)(RETURN)(\s+[^,]+(?:\s*,\s*[^,]+)*)`).ReplaceAllStringFunc(lineStr, func(match string) string {
		// Color the RETURN keyword
		result := wrapInColor("RETURN", 35)

		// Split the rest by commas
		rest := match[6:] // Skip "RETURN"
		parts := strings.Split(rest, ",")

		for i, part := range parts {
			if i > 0 {
				result += ","
			}

			// Handle "AS" keyword
			if strings.Contains(strings.ToUpper(part), " AS ") {
				asParts := strings.SplitN(part, " AS ", 2)
				result += wrapInColor(" "+strings.TrimSpace(asParts[0]), 35)
				result += " " + wrapInColor("AS", 35) + " " + strings.TrimSpace(asParts[1])
				continue
			}

			// Handle JSONPath
			part = strings.TrimSpace(part)
			if strings.Contains(part, ".*") {
				varName := strings.TrimSuffix(part, ".*")
				result += wrapInColor(" "+varName, 35) + ".*"
			} else if strings.Contains(part, ".") {
				dotParts := strings.SplitN(part, ".", 2)
				result += wrapInColor(" "+dotParts[0], 35) + "." + wrapInColor(dotParts[1], 35)
			} else {
				result += wrapInColor(" "+part, 35)
			}
		}

		return result
	})

	// Coloring for properties
	lineStr = regexp.MustCompile(`{([^{}]+)}`).ReplaceAllStringFunc(lineStr, func(match string) string {
		// Extract just the inner content without braces
		inner := match[1 : len(match)-1]
		return wrapInColor("{", 37) + colorizePropertyContent(inner) + wrapInColor("}", 37)
	})

	return []rune(lineStr)
}

func colorizePropertyContent(content string) string {
	if core.NoColor {
		return content
	}

	// Colorize key-value pairs
	return regexp.MustCompile(`((?:"(?:[^"\\]|\\.)*"|[^:,{}]+)\s*:\s*)("[^"]*"|[^,{}]+)`).ReplaceAllStringFunc(content, func(prop string) string {
		parts := regexp.MustCompile(`((?:"(?:[^"\\]|\\.)*"|[^:,{}]+)\s*:\s*)(.+)`).FindStringSubmatch(prop)
		if len(parts) == 3 {
			key := parts[1]
			value := parts[2]
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				return wrapInColor(key, 33) + wrapInColor(value, 36)
			}
			return wrapInColor(key, 33) + wrapInColor(value, 36)
		}
		return prop
	})
}

type Listener interface {
	OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool)
}

func initAndRunShell(_ *cobra.Command, _ []string) {
	// Create the API server provider
	p, err := apiserver.NewAPIServerProvider()
	if err != nil {
		fmt.Println("Error creating provider:", err)
		os.Exit(1)
	}

	// Initialize the executor instance with the provider
	executor = core.GetQueryExecutorInstance(p)
	if executor == nil {
		fmt.Println("Error initializing query executor")
		os.Exit(1)
	}

	// Get current context
	currentContext, _, err := getCurrentContext()
	if err != nil {
		fmt.Println("Error getting current context:", err)
		os.Exit(1)
	}
	ctx = currentContext

	// Initialize shell environment
	if core.AllNamespaces {
		core.Namespace = ""
		core.AllNamespaces = false
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
		} else if input == "\\lm" {
			fmt.Print("Registered macros:\n\n")
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
			fmt.Println("")
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
			printHelp()
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
		if edges, ok := graphInternal.(map[string]interface{})["Edges"]; ok && edges != nil {
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
		// Marshal data based on the output format
		var output []byte
		var err error
		if core.OutputFormat == "yaml" {
			output, err = yaml.Marshal(data)
		} else { // core.OutputFormat == "json"
			output, err = json.MarshalIndent(data, "", "  ")
			if !returnRawJsonOutput {
				output = []byte(formatJson(string(output)))
			}
		}

		// Handle marshalling errors
		if err != nil {
			return fmt.Errorf("error marshalling data: %w", err)
		}

		*result = string(output)
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

	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		core.NoColor = true
	}

	// Add format flag to shell command
	ShellCmd.Flags().StringVar(&core.OutputFormat, "format", "json", "Output format (json or yaml)")
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
	splash := `
                __                    __        
 ______ _____  / /  ___ _______  ___ / /____ ___
/ __/ // / _ \/ _ \/ -_) __/ _ \/ -_) __/ -_|_-<
\__/\_, / .__/_//_/\__/_/ /_//_/\__/\__/\__/___/
   /___/_/ Interactive Shell`
	fmt.Println(wrapInColor(splash, 36))
	fmt.Println("")
}

func printHelp() {
	// Define colors
	cmdColor := 35     // Purple for commands
	descColor := 36    // Cyan for descriptions
	sectionColor := 33 // Yellow for sections

	// Helper function to format command help
	formatCmd := func(cmd, desc string) string {
		return fmt.Sprintf("  %s %s",
			wrapInColor(cmd, cmdColor),
			wrapInColor(desc, descColor))
	}

	// Helper function for section headers
	formatSection := func(title string) string {
		return wrapInColor(title, sectionColor)
	}

	fmt.Printf(`%s
%s
%s
%s
%s
%s
%s
%s
%s
%s
%s
%s

%s
%s
%s

%s
%s
%s
%s
%s

%s
%s`,
		formatSection("Commands:"),
		formatCmd("\\h, \\help", "Show this help"),
		formatCmd("\\q, \\quit, \\exit", "Exit the shell"),
		formatCmd("\\c, \\clear", "Clear the screen"),
		formatCmd("\\n, \\namespace <name>", "Switch to namespace"),
		formatCmd("\\A, \\all-namespaces", "Query all namespaces"),
		formatCmd("\\lm, \\list-macros", "List available macros"),
		formatCmd("\\t, \\time", "Toggle query execution time display"),
		formatCmd("\\g, \\graph", "Toggle graph output"),
		formatCmd("\\gl, \\graph-layout", "Toggle graph layout direction"),
		formatCmd("\\m, \\multiline", "Toggle multiline input mode"),
		formatCmd("\\r, \\raw", "Toggle raw JSON output"),

		formatSection("\nQuery syntax:"),
		wrapInColor("  MATCH (n:Pod) RETURN n;", descColor),
		wrapInColor("  MATCH (n:Pod)->(s:Service) RETURN n.metadata.name, s.metadata.name;", descColor),

		formatSection("\nMacros:"),
		formatCmd(":getpo", "List pods"),
		formatCmd(":getdeploy", "List deployments"),
		formatCmd(":getsvc", "List services"),
		wrapInColor("  ... and more, use \\lm to list all available macros", descColor),

		wrapInColor("\nPress Ctrl+D or type \\q to exit", descColor),
		wrapInColor("Press Ctrl+C to cancel current input", descColor))
	fmt.Println("")
	fmt.Println("")
}
