package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var resourceTreeStructureCache = make(map[string][]string)
var resourceSpecs = make(map[string][]string)

func initResourceSpecs() {
	resourceSpecs = parser.ResourceSpecs
}

type CyphernetesCompleter struct {
	// You can add fields if needed, for example, a reference to your GRV cache
}

func (c *CyphernetesCompleter) Do(line []rune, pos int) ([][]rune, int) {
	var suggestions [][]rune

	// Extract the last word from the line up to the cursor position.
	// This assumes words are separated by spaces.
	lineStr := string(line[:pos])
	if len(lineStr) < 1 {
		// If the line is empty, return no suggestions
		return suggestions, 0
	}

	words := strings.Fields(lineStr)
	lastWord := words[len(words)-1]
	prefix := strings.ToLower(lastWord)

	if len(words) > 0 {

		// Check if the last word starts with a '(', followed by a word, followed by a colon
		// or if it's a connected node pattern
		resourceKindIdentifierRegex := regexp.MustCompile(`(\(\w+:\w+$|\)->\(\w+:\w+$)`)
		if resourceKindIdentifierRegex.MatchString(lastWord) {
			var identifier string
			if strings.Contains(lastWord, ")->") {
				// For connected nodes, get the last node
				parts := strings.Split(lastWord, ")->")
				identifier = strings.SplitAfter(parts[len(parts)-1], ":")[1]
			} else {
				identifier = strings.SplitAfter(lastWord, ":")[1]
			}
			resourceKinds := getResourceKinds(identifier)
			for _, kind := range resourceKinds {
				if strings.HasPrefix(kind, identifier) {
					// Offer each kind as a suggestion
					// everything from the last word after the colon:
					suggestion := string(kind)[len(identifier):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
			// Set the length to 0 since we want to append the entire resource kind
		} else if isJSONPathContext(lineStr, pos, lastWord) {
			identifier := strings.Split(lastWord, ".")[0]
			kind := getKindForIdentifier(lineStr, identifier)
			treeStructure, err := fetchResourceTreeStructureForKind(kind)
			matchedWord := strings.Replace(lastWord, identifier, "$", 1)
			if err == nil {
				currentLevelSuggestions := make(map[string]bool)
				for _, node := range treeStructure {
					node = "$." + node
					if strings.HasPrefix(node, matchedWord) {
						fullPath := strings.TrimPrefix(node[len(matchedWord):], ".")
						parts := strings.Split(fullPath, ".")
						if len(parts) > 0 {
							suggestion := parts[0]
							if len(parts) > 1 {
								suggestion += "."
							}
							if suggestion == "" {
								suggestions = append(suggestions, []rune("."))
								break
							}
							currentLevelSuggestions[suggestion] = true
						}
					}
				}

				// Convert map to sorted slice
				var sortedSuggestions []string
				for suggestion := range currentLevelSuggestions {
					sortedSuggestions = append(sortedSuggestions, suggestion)
				}
				sort.Strings(sortedSuggestions)

				// Remove entries without a dot if a corresponding entry with a dot exists
				filteredSuggestions := make([]string, 0, len(sortedSuggestions))
				suggestionMap := make(map[string]bool)

				for _, suggestion := range sortedSuggestions {
					if strings.HasSuffix(suggestion, ".") {
						suggestionMap[strings.TrimSuffix(suggestion, ".")] = true
					}
					if strings.HasSuffix(suggestion, "[].") {
						suggestionMap[strings.TrimSuffix(suggestion, "[].")] = true
					}
				}

				for _, suggestion := range sortedSuggestions {
					if !strings.HasSuffix(suggestion, ".") && suggestionMap[suggestion] {
						continue
					}
					filteredSuggestions = append(filteredSuggestions, suggestion)
				}

				sortedSuggestions = filteredSuggestions

				// Add sorted and unique suggestions
				for _, suggestion := range sortedSuggestions {
					suggestions = append(suggestions, []rune(suggestion))
				}
			} //	 else {
			// 	fmt.Println("Error fetching resource tree structure: ", err)
			// }
		} else if isMacroContext(lineStr) {
			// Offer each macro as a suggestion
			macros := getMacros()
			for _, macro := range macros {
				macro = ":" + macro
				if strings.HasPrefix(macro, prefix) {
					// Append only the part of the macro that comes after the prefix
					suggestion := macro[len(prefix):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		} else {
			// Handle other autocompletion cases (like keywords)

			// Keywords
			keywords := []string{"match", "where", "return", "set", "delete", "create", "as", "sum", "count"}

			for _, k := range keywords {
				if strings.HasPrefix(k, prefix) {
					// Append only the part of the keyword that comes after the prefix
					suggestion := k[len(prefix):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		}
	}

	// The length returned should be the length of the last word.
	return suggestions, len(lastWord)
}

func isMacroContext(line string) bool {
	// line starts with a colon and is followed by a word
	regex := regexp.MustCompile(`^:\w+$`)
	return regex.MatchString(line)
}

func getMacros() []string {
	macros := []string{}
	for _, macro := range macroManager.Macros {
		macros = append(macros, macro.Name)
	}
	return macros
}

func fetchResourceTreeStructureForKind(kind string) ([]string, error) {
	// First, get the full GVR for the kind from the GVR cache
	gvr, err := parser.FindGVR(executor.Clientset, kind)
	if err != nil {
		return nil, err
	}

	resourceNormalizedName := strings.ToLower(gvr.Resource)

	// Check if the tree structure is already cached
	if treeStructure, ok := resourceTreeStructureCache[resourceNormalizedName]; ok {
		return treeStructure, nil
	}

	// Fetch the API definition (field paths)
	treeStructure, err := fetchResourceAPIDefinition(gvr)
	if err != nil {
		return nil, err
	}

	// Cache the tree structure for future use
	resourceTreeStructureCache[resourceNormalizedName] = treeStructure

	return treeStructure, nil
}

func fetchResourceAPIDefinition(gvr schema.GroupVersionResource) ([]string, error) {
	// Ensure resourceSpecs is initialized
	if len(resourceSpecs) == 0 {
		initResourceSpecs()
	}

	// Get the kind associated with the GVR
	kind, err := getKindFromGVR(gvr)
	if err != nil {
		return nil, fmt.Errorf("error getting kind for GVR %v: %v", gvr, err)
	}

	// Get the schema name
	schemaName := getSchemaName(gvr.Group, gvr.Version, kind)

	// Retrieve the fields for the schema
	fields, ok := resourceSpecs[schemaName]
	if !ok {
		return nil, fmt.Errorf("resource %s not found in OpenAPI specs", schemaName)
	}

	return fields, nil
}

// Helper function to get the kind from GVR
func getKindFromGVR(gvr schema.GroupVersionResource) (string, error) {
	discoveryClient := executor.Clientset.Discovery()
	apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return "", err
	}

	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Name == gvr.Resource || apiResource.Name == strings.TrimSuffix(gvr.Resource, "s") {
			return apiResource.Kind, nil
		}
	}

	return "", fmt.Errorf("kind not found for GVR %v", gvr)
}

// Helper function to build the schema name
func getSchemaName(group, version, kind string) string {
	// Ensure resourceSpecs is initialized
	if len(resourceSpecs) == 0 {
		initResourceSpecs()
	}

	// First, try to find the schema name in the resourceSpecs
	for schemaName := range resourceSpecs {
		if strings.Contains(strings.ToLower(schemaName), strings.ToLower(kind)) {
			parts := strings.Split(schemaName, ".")
			if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-1], kind) {
				return schemaName
			}
		}
	}

	// If not found in resourceSpecs, use the dynamic client to get more information
	gvr, err := parser.FindGVR(parser.GetQueryExecutorInstance().Clientset, kind)
	if err == nil {
		// Construct a potential schema name based on the GVR
		potentialSchemaName := fmt.Sprintf("io.k8s.api.%s.%s.%s", gvr.Group, gvr.Version, kind)

		// Check if this constructed name exists in resourceSpecs
		if _, ok := resourceSpecs[potentialSchemaName]; ok {
			return potentialSchemaName
		}

		// If not found, try variations
		variations := []string{
			fmt.Sprintf("io.k8s.api.%s.%s.%s", strings.ReplaceAll(gvr.Group, ".", ""), gvr.Version, kind),
			fmt.Sprintf("%s.%s.%s", gvr.Group, gvr.Version, kind),
		}

		for _, variation := range variations {
			if _, ok := resourceSpecs[variation]; ok {
				return variation
			}
		}
	}

	// If still not found, log a warning and return a best guess
	fmt.Printf("Warning: No exact schema match found for group=%s, version=%s, kind=%s\n", group, version, kind)
	return fmt.Sprintf("io.k8s.api.%s.%s.%s", group, version, kind)
}

func getKindForIdentifier(line string, identifier string) string {
	result := ""
	// Regular expression to find the position of words after '(' and before ':'
	regex := regexp.MustCompile(`\((\w+):(\w+)`)
	matches := regex.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if match[1] == identifier {
			// return the last capture group
			return match[2]
		}
	}

	return result
}

func getResourceKinds(identifier string) []string {
	// iterate over the gvr cache and return all resource kinds that match the identifier
	var kinds []string
	for _, gvr := range parser.GvrCache {
		if strings.HasPrefix(gvr.GroupResource().Resource, identifier) {
			kinds = append(kinds, gvr.Resource)
		}
	}
	return kinds
}

func isJSONPathContext(line string, pos int, lastWord string) bool {
	// Regular expression to find the position of "RETURN" and any JSONPaths after it
	regex := regexp.MustCompile(`(?i)(return|set|where)(\s+.*)(,|$)`)
	matches := regex.FindAllStringSubmatchIndex(line, -1)
	for _, match := range matches {
		wordRegex := regexp.MustCompile(`\w+\.|\$\.\w+\.|\$\.\w+\.(\w)|\w+\.(\w+)`)
		if wordRegex.MatchString(lastWord) {
			if pos > match[2] && ((pos == len(line) && len(line) > 20) || pos < match[3]) {
				return true
			}
		}
	}
	return false
}
