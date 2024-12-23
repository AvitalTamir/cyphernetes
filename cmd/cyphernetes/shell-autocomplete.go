package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CyphernetesCompleter struct{}

func (c *CyphernetesCompleter) Do(line []rune, pos int) ([][]rune, int) {
	var suggestions [][]rune
	lineStr := string(line[:pos])
	if len(lineStr) < 1 {
		return suggestions, 0
	}

	words := strings.Fields(lineStr)
	lastWord := words[len(words)-1]
	prefix := strings.ToLower(lastWord)

	if len(words) > 0 {
		resourceKindIdentifierRegex := regexp.MustCompile(`(\(\w+:\w+$|\)->\(\w+:\w+$)`)
		if resourceKindIdentifierRegex.MatchString(lastWord) {
			var identifier string
			if strings.Contains(lastWord, ")->") {
				parts := strings.Split(lastWord, ")->")
				identifier = strings.SplitAfter(parts[len(parts)-1], ":")[1]
			} else {
				identifier = strings.SplitAfter(lastWord, ":")[1]
			}
			resourceKinds := getResourceKinds(identifier)
			for _, kind := range resourceKinds {
				if strings.HasPrefix(kind, identifier) {
					suggestion := string(kind)[len(identifier):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
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

				var sortedSuggestions []string
				for suggestion := range currentLevelSuggestions {
					sortedSuggestions = append(sortedSuggestions, suggestion)
				}
				sort.Strings(sortedSuggestions)

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

				for _, suggestion := range filteredSuggestions {
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		} else if isMacroContext(lineStr) {
			macros := getMacros()
			for _, macro := range macros {
				macro = ":" + macro
				if strings.HasPrefix(macro, prefix) {
					suggestion := macro[len(prefix):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		} else {
			keywords := []string{"match", "where", "return", "set", "delete", "create", "as", "sum", "count", "in", "contains"}
			for _, k := range keywords {
				if strings.HasPrefix(k, prefix) {
					suggestion := k[len(prefix):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		}
	}
	return suggestions, len(lastWord)
}

func isMacroContext(line string) bool {
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
	if executor == nil {
		return nil, fmt.Errorf("executor not initialized")
	}

	schemaName := getSchemaName(kind)
	if schemaName == "" {
		return nil, fmt.Errorf("schema not found for kind %s", kind)
	}

	fields, ok := core.ResourceSpecs[schemaName]
	if !ok {
		return nil, fmt.Errorf("resource %s not found in OpenAPI specs", schemaName)
	}

	return fields, nil
}

func getSchemaName(kind string) string {
	exactMatch := "io.k8s.api.apps.v1.Deployment"
	if strings.EqualFold(kind, "deployments") {
		return exactMatch
	}

	for schemaName := range core.ResourceSpecs {
		if strings.Contains(strings.ToLower(schemaName), strings.ToLower(kind)) {
			parts := strings.Split(schemaName, ".")
			if len(parts) >= 4 {
				lastPart := parts[len(parts)-1]
				if strings.EqualFold(lastPart+"s", kind) || strings.EqualFold(lastPart, kind) {
					return schemaName
				}
			}
		}
	}

	if gvr, ok := core.GvrCache[strings.ToLower(kind)]; ok {
		pattern := fmt.Sprintf("io.k8s.api.%s.%s.%s", gvr.Group, gvr.Version, strings.TrimSuffix(kind, "s"))
		if _, ok := core.ResourceSpecs[pattern]; ok {
			return pattern
		}
	}

	return ""
}

func getKindForIdentifier(line string, identifier string) string {
	regex := regexp.MustCompile(`\((\w+):(\w+)`)
	matches := regex.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if match[1] == identifier {
			kind := match[2]
			if executor != nil {
				gvrCache, err := executor.Provider().GetGVRCache()
				if err != nil {
					return kind
				}

				if gvr, ok := gvrCache[strings.ToLower(kind)]; ok {
					return findCanonicalKind(gvrCache, gvr)
				}

				for k, gvr := range gvrCache {
					if strings.EqualFold(gvr.Resource, kind) ||
						strings.EqualFold(strings.TrimSuffix(gvr.Resource, "s"), kind) ||
						strings.EqualFold(k, kind) {
						return findCanonicalKind(gvrCache, gvr)
					}
				}
			}
			return kind
		}
	}
	return ""
}

func findCanonicalKind(gvrCache map[string]schema.GroupVersionResource, gvr schema.GroupVersionResource) string {
	for k, v := range gvrCache {
		if v == gvr && !strings.Contains(k, "/") &&
			!strings.HasSuffix(k, "List") &&
			!strings.Contains(strings.ToLower(k), "s") {
			return k
		}
	}
	return strings.TrimSuffix(gvr.Resource, "s")
}

func getResourceKinds(identifier string) []string {
	if executor == nil {
		return nil
	}

	var kinds []string
	gvrCache, err := executor.Provider().GetGVRCache()
	if err != nil {
		fmt.Printf("Error getting GVR cache: %v\n", err)
		return kinds
	}

	for _, gvr := range gvrCache {
		if strings.HasPrefix(gvr.GroupResource().Resource, identifier) {
			kinds = append(kinds, gvr.Resource)
		}
	}
	return kinds
}

func isJSONPathContext(line string, pos int, lastWord string) bool {
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
